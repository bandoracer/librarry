package wanted

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrReviewSelection = errors.New("invalid metadata review selection")
var ErrReviewChanged = errors.New("metadata changed during confirmation; refresh and review again")

func normalizeReviewSelection(request MetadataReviewConfirmRequest) (MetadataReviewConfirmRequest, error) {
	if request.All && len(request.WantedIDs) > 0 || !request.All && len(request.WantedIDs) == 0 || len(request.WantedIDs) > 200 {
		return request, fmt.Errorf("%w: provide 1–200 wantedIds or all, exclusively", ErrReviewSelection)
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, id := range request.WantedIDs {
		var parsed pgtype.UUID
		if parsed.Scan(strings.TrimSpace(id)) != nil || !parsed.Valid {
			return request, fmt.Errorf("%w: wantedIds must be book UUIDs", ErrReviewSelection)
		}
		value, _ := parsed.Value()
		id = value.(string)
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if len(request.Revisions) > 0 {
		if request.All || len(request.Revisions) != len(ids) {
			return request, fmt.Errorf("%w: revisions must match the explicit selection", ErrReviewSelection)
		}
		revisions := map[string]string{}
		for id, revision := range request.Revisions {
			var parsed pgtype.UUID
			if parsed.Scan(strings.TrimSpace(id)) != nil || !parsed.Valid {
				return request, ErrReviewSelection
			}
			value, _ := parsed.Value()
			id = value.(string)
			decoded, e := hex.DecodeString(revision)
			if e != nil || len(decoded) != 32 || !seen[id] || revisions[id] != "" {
				return request, fmt.Errorf("%w: invalid review revision", ErrReviewSelection)
			}
			revisions[id] = strings.ToLower(revision)
		}
		request.Revisions = revisions
	}
	request.WantedIDs = ids
	return request, nil
}

// Confirmation reads the exact selection and commits every accepted field in one
// transaction. A changed owner override aborts the batch instead of being replaced.
func (s *Store) ConfirmMetadataReviewCanonical(ctx context.Context, request MetadataReviewConfirmRequest) (outcome MetadataReviewConfirmOutcome, err error) {
	request, err = normalizeReviewSelection(request)
	if err != nil {
		return outcome, err
	}
	if !s.Configured() {
		return outcome, sql.ErrConnDone
	}
	defer func() {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) && (pgerr.Code == "40001" || pgerr.Code == "40P01") {
			err = ErrReviewChanged
		}
	}()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return outcome, err
	}
	defer tx.Rollback()
	var ids []string
	if !request.All {
		ids = request.WantedIDs
	}
	items, _, err := reviewBooks(ctx, tx, ids, true)
	if err != nil {
		return outcome, err
	}
	if !request.All && len(items) != len(ids) {
		return outcome, fmt.Errorf("%w: a selected book no longer exists; refresh the selection", ErrReviewSelection)
	}
	records, err := reviewRecords(ctx, tx, items)
	if err != nil {
		return outcome, err
	}
	outcome = MetadataReviewConfirmOutcome{Status: "ok", Items: []MetadataProvenance{}, GeneratedAt: time.Now().UTC()}
	changed := []WantedItem{}
	for _, item := range items {
		if wantedItemReviewSkipped(item) {
			outcome.SkippedItems++
			continue
		}
		review := metadataReviewItem(reviewProvenance(item, records[item.ID], outcome.GeneratedAt))
		if len(request.Revisions) > 0 && request.Revisions[item.ID] != review.Revision {
			return MetadataReviewConfirmOutcome{}, ErrReviewChanged
		}
		corrections := metadataReviewCanonicalCorrections(review)
		if len(corrections) == 0 {
			outcome.SkippedItems++
			continue
		}
		for _, correction := range corrections {
			var previous *ManualOverride
			for i := range item.ManualOverrides {
				if item.ManualOverrides[i].FieldName == correction.FieldName {
					previous = &item.ManualOverrides[i]
					break
				}
			}
			var result sql.Result
			var e error
			if previous != nil {
				// UPDATE detects a concurrently changed or deleted snapshot row. An upsert
				// here would resurrect an override that the owner deliberately cleared.
				result, e = tx.ExecContext(ctx, `update manual_overrides set reason=$4,updated_at=now()
     where entity_type='wanted_item' and entity_id=$1::uuid and field_name=$2 and value #>> '{}'=$3`, item.ID, correction.FieldName, previous.Value, manualOverrideReasonCanonicalAccepted)
			} else {
				// A new owner override must win even if it happens to have the same value.
				result, e = tx.ExecContext(ctx, `insert into manual_overrides(entity_type,entity_id,field_name,value,reason)
     values('wanted_item',$1::uuid,$2,to_jsonb($3::text),$4)
     on conflict(entity_type,entity_id,field_name) do nothing`, item.ID, correction.FieldName, correction.Value, manualOverrideReasonCanonicalAccepted)
			}
			if e != nil {
				return MetadataReviewConfirmOutcome{}, e
			}
			n, e := result.RowsAffected()
			if e != nil {
				return MetadataReviewConfirmOutcome{}, e
			}
			if n != 1 {
				return MetadataReviewConfirmOutcome{}, ErrReviewChanged
			}
		}
		if err = tx.QueryRowContext(ctx, `update wanted_items set updated_at=now() where id=$1 returning updated_at`, item.ID).Scan(&item.UpdatedAt); err != nil {
			return MetadataReviewConfirmOutcome{}, err
		}
		changed = append(changed, item)
		outcome.ItemsReviewed++
		outcome.FieldsConfirmed += len(corrections)
	}
	changed, err = attachWantedDetails(ctx, tx, changed)
	if err != nil {
		return MetadataReviewConfirmOutcome{}, err
	}
	for _, item := range changed {
		outcome.Items = append(outcome.Items, reviewProvenance(item, records[item.ID], outcome.GeneratedAt))
	}
	if outcome.ItemsReviewed == 0 {
		outcome.Status = "empty"
	}
	if err = tx.Commit(); err != nil {
		return MetadataReviewConfirmOutcome{}, err
	}
	return outcome, nil
}
