package acquisition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// FinalizeAcquisition repairs local bookkeeping from the saved receipt. Remote
// acceptance has already committed, so a history/database failure never permits
// another add. Callers can retry this transaction after a process restart.
func (s *SQLDownloadStore) FinalizeAcquisition(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	i, err := scanIntent(tx.QueryRowContext(ctx, `select `+intentColumns+` from acquisition_intents where id=$1 for update`, id))
	if err != nil {
		return err
	}
	if i.State != "accepted" {
		return errors.New("acquisition has no accepted receipt")
	}
	if !i.BookkeepingRequired {
		return nil
	}
	var downloadID, state, importStatus string
	var failed bool
	var newer bool
	err = tx.QueryRowContext(ctx, `select d.id::text,d.state,d.import_status,d.failed_at is not null,
 exists(select 1 from acquisition_intents a where a.id=d.acquisition_intent_id and (a.created_at,a.id)>($3::timestamptz,$4::uuid))
 from downloads d where client=$1 and external_id=$2 for update`, i.Client, i.ExternalID, i.CreatedAt, i.ID).Scan(&downloadID, &state, &importStatus, &failed, &newer)
	if err != nil {
		return fmt.Errorf("accepted download bookkeeping needs retry: %w", err)
	}
	if !newer {
		_, err = tx.ExecContext(ctx, `update downloads set acquisition_intent_id=$2,release_id=(select id from releases where id::text=$3 and wanted_item_id::text=$4) where id=$1`, downloadID, i.ID, i.Selection.ReleaseID, i.WantedID)
		if err != nil {
			return err
		}
		if i.BookkeepingAt == nil && state != "removed" && importStatus != "imported" && !failed && i.WantedID != "" {
			// An upgrade in flight must not hide the already imported book. User removal
			// during a slow client add also wins over the later acceptance receipt.
			if _, err = tx.ExecContext(ctx, `update wanted_items set status='grabbed',updated_at=now() where id=$1 and status not in ('imported','removed','ignored')`, i.WantedID); err != nil {
				return err
			}
		}
	}
	if i.BookkeepingAt != nil {
		return tx.Commit()
	}
	entityType, entityID := "acquisition", i.ID
	if i.WantedID != "" {
		entityType, entityID = "wanted_item", i.WantedID
	}
	message := "Release accepted by " + i.Client
	if i.Selection.Forced {
		message = "Manually selected release accepted by " + i.Client
	}
	data, err := json.Marshal(map[string]any{"currentScore": i.Selection.CurrentScore, "cutoffScore": i.Selection.CutoffScore, "acquisitionId": i.ID, "downloadRecordId": downloadID, "downloadId": i.ExternalID, "client": i.Client, "releaseId": i.Selection.ReleaseID, "score": i.Selection.Score, "sourceId": i.Selection.SourceID, "title": i.Title, "trigger": i.Selection.Trigger, "forced": i.Selection.Forced, "paused": i.Selection.Paused, "rejectedReason": i.Selection.RejectedReason})
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `insert into history_events(event_type,entity_type,entity_id,message,data) values('release_grabbed',$1,$2,$3,$4::jsonb)`, entityType, entityID, message, string(data)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `update acquisition_intents set bookkeeping_at=now(),last_error='',updated_at=now() where id=$1`, i.ID); err != nil {
		return err
	}
	return tx.Commit()
}
