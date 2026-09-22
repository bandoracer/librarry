package wanted

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrCompatibilityBookSelection = errors.New("book selection changed or is inactive; refresh before retrying")
var ErrCompatibilityBookRequest = errors.New("invalid compatibility book selection")

type CompatibilityBookEdit struct {
	ID        string
	UpdatedAt time.Time
	Update    WantedUpdateRequest
}
type CompatibilityBookMutation struct {
	Books  []CompatibilityBookEdit
	Delete bool
}

// ApplyCompatibilityBooks checks every reviewed active target under locks before
// any write. A stale selection or persistence failure rolls the whole batch back.
func (s *Service) ApplyCompatibilityBooks(ctx context.Context, request CompatibilityBookMutation) ([]WantedItem, error) {
	if len(request.Books) == 0 || len(request.Books) > 500 {
		return nil, ErrCompatibilityBookRequest
	}
	ids := []string{}
	byID := map[string]CompatibilityBookEdit{}
	for _, edit := range request.Books {
		var id pgtype.UUID
		if id.Scan(edit.ID) != nil || !id.Valid || edit.UpdatedAt.IsZero() {
			return nil, ErrCompatibilityBookRequest
		}
		if _, ok := byID[edit.ID]; ok {
			return nil, ErrCompatibilityBookRequest
		}
		byID[edit.ID] = edit
		ids = append(ids, edit.ID)
	}
	if !s.Available() {
		return nil, sql.ErrConnDone
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	books, _, err := reviewBooks(ctx, tx, ids, true)
	if err != nil {
		return nil, err
	}
	if len(books) != len(ids) {
		return nil, ErrCompatibilityBookSelection
	}
	for _, book := range books {
		if book.Status == "removed" || book.Status == "ignored" || !book.UpdatedAt.Equal(byID[book.ID].UpdatedAt) {
			return nil, ErrCompatibilityBookSelection
		}
	}
	items := []WantedItem{}
	for _, id := range ids {
		update := byID[id].Update
		if request.Delete {
			no := false
			update = WantedUpdateRequest{Status: "removed", Monitored: &no}
		}
		item, e := updateWantedInTransaction(ctx, tx, id, update)
		if e != nil {
			return nil, e
		}
		items = append(items, item)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if !request.Delete {
		items = s.AnnotateWantedStates(ctx, items)
	}
	return items, nil
}

// CompatibilityBooks implements the Readarr array contract over the entire active
// collection. Native interactive pages continue to use BookCollection. File and
// download state share that collection's snapshot and one client observation.
func (s *Service) CompatibilityBooks(ctx context.Context) ([]WantedItem, error) {
	if !s.Available() {
		return nil, sql.ErrConnDone
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, args, err := s.collectionSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	items, err := readCompatibilityBooks(ctx, tx, bookCollectionSQL+`select `+wantedDetailColumns+`,b.derived_state,b.download_state,b.file_state,b.file_reason,b.present_files,b.required_files
	 from stateful b join wanted_items wi on wi.id=b.id left join works w on w.id=wi.work_id
	 order by lower(b.title) collate "C",wi.id`, args)
	if err != nil {
		return nil, err
	}
	return items, tx.Commit()
}

func readCompatibilityBooks(ctx context.Context, tx *sql.Tx, query string, args []any) ([]WantedItem, error) {
	rows, err := tx.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, err
	}
	items := []WantedItem{}
	for rows.Next() {
		var state, phase string
		var evidence FileEvidence
		item, e := scanWanted(wantedWithExtra{row: rows, extra: []any{&state, &phase, &evidence.State, &evidence.Reason, &evidence.PresentFiles, &evidence.RequiredFiles}})
		if e != nil {
			rows.Close()
			return nil, e
		}
		item.DerivedState = state
		item.DownloadState = phase
		item.StateEvidence = &BookStateEvidence{Files: evidence, Downloads: args[2].(string), Quality: "available"}
		if evidence.State != "present" && evidence.State != "missing" {
			item.StateEvidence.Message = evidence.Reason
		}
		if item.StateEvidence.Downloads == "partial" || item.StateEvidence.Downloads == "unavailable" {
			item.StateEvidence.Message += " Download-client evidence is unavailable or incomplete."
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	profiles, err := listQualityProfiles(ctx, tx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		profile := profileFromList(profiles, items[i])
		items[i].CompatibilityProfile = &profile
	}
	items, err = attachWantedDetails(ctx, tx, items)
	if err != nil {
		return nil, err
	}
	return items, nil
}
