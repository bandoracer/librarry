package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// File registration, installed-release selection and one event per imported
// book share the import transaction. Replaying an already committed operation
// never enters this code again.
func commitImportBookkeeping(ctx context.Context, tx *sql.Tx, op ImportOperation, files []FileRecord, books map[string]string) error {
	if op.DownloadRecordID != "" {
		var pending bool
		if err := tx.QueryRowContext(ctx, `select exists(select 1 from downloads d join acquisition_intents a on a.client=d.client and a.external_id=d.external_id where d.id=$1 and a.state='accepted' and a.bookkeeping_required and a.bookkeeping_at is null)`, op.DownloadRecordID).Scan(&pending); err != nil {
			return err
		}
		if pending {
			return errors.New("finish accepted acquisition recovery in Activity before committing this import")
		}
	}
	for bookID, format := range books {
		releaseID := ""
		score := 0.0
		if op.SourceKind != "manual" && op.DownloadRecordID != "" {
			// A mapped pack cannot borrow the original requested book's release score.
			// Unknown/legacy grabs stay unknown, rather than using the best search hit.
			err := tx.QueryRowContext(ctx, `select coalesce(r.id::text,''),coalesce((a.selection->>'score')::numeric,0)
 from downloads d left join acquisition_intents a on a.id=d.acquisition_intent_id
 left join releases r on r.id=d.release_id and r.wanted_item_id=$2 and r.id::text=a.selection->>'releaseId' and a.wanted_item_id=$2
 where d.id=$1`, op.DownloadRecordID, bookID).Scan(&releaseID, &score)
			if err != nil {
				return err
			}
			if releaseID == "" {
				score = 0
			}
			if _, err = tx.ExecContext(ctx, `update wanted_items set current_release_id=nullif($2,'')::uuid,current_release_score=$3 where id=$1`, bookID, releaseID, score); err != nil {
				return err
			}
		} else if op.Metadata["conflictAction"] == "replace" {
			// A manual replacement has no proven release identity.
			if _, err := tx.ExecContext(ctx, `update wanted_items set current_release_id=null,current_release_score=0 where id=$1`, bookID); err != nil {
				return err
			}
		}
		ids, paths := []string{}, []string{}
		for _, f := range files {
			if f.Metadata["wantedId"] == bookID {
				ids = append(ids, f.ID)
				paths = append(paths, f.Path)
			}
		}
		data, err := json.Marshal(map[string]any{"operationId": op.ID, "downloadRecordId": op.DownloadRecordID, "downloadId": op.DownloadID, "client": op.Client, "fileIds": ids, "paths": paths, "format": format, "releaseId": releaseID, "score": score, "sourceKind": op.SourceKind, "conflictAction": op.Metadata["conflictAction"]})
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `insert into history_events(event_type,entity_type,entity_id,message,data) values('book_imported','wanted_item',$1,'Book file set imported',$2::jsonb)`, bookID, string(data)); err != nil {
			return err
		}
	}
	// Unassigned manual imports still have a committed outcome worth notifying.
	if len(books) == 0 && len(files) > 0 {
		ids, paths := []string{}, []string{}
		for _, file := range files {
			ids = append(ids, file.ID)
			paths = append(paths, file.Path)
		}
		data, err := json.Marshal(map[string]any{"operationId": op.ID, "fileIds": ids, "paths": paths, "format": files[0].MediaFormat, "title": files[0].Title, "sourceKind": op.SourceKind})
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `insert into history_events(event_type,entity_type,entity_id,message,data) values('book_imported','import_operation',$1,'Unassigned file set imported',$2::jsonb)`, op.ID, string(data)); err != nil {
			return err
		}
	}
	return nil
}
