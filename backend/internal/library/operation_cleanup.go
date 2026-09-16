package library

import (
	"context"
	"errors"
	"fmt"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func (s *Service) verifyOperationCleanup(ctx context.Context, op ImportOperation, download acquisition.DownloadStatus, inventory []acquisition.DownloadFile) (resultErr error) {
	if op.SourceKind == "manual" {
		return errors.New("manual imports do not authorize download deletion")
	}
	defer func() {
		state, message := "eligible", ""
		if resultErr != nil {
			state, message = "blocked", resultErr.Error()
		}
		_, err := s.store.db.ExecContext(ctx, `update import_operations set cleanup_state=$2,cleanup_error=$3,updated_at=now() where id=$1`, op.ID, state, message)
		if err != nil && resultErr == nil {
			resultErr = fmt.Errorf("persist cleanup verification: %w", err)
		}
	}()
	if op.State != "committed" {
		return errors.New("import operation is not committed")
	}
	if op.ReplacementCleanupState == "pending" {
		return errors.New("replacement backup cleanup is still pending")
	}
	if len(op.Files) == 0 {
		return errors.New("import manifest is empty")
	}
	for _, f := range op.Files {
		destination, err := s.currentManifestDestination(ctx, op, f)
		if err != nil {
			return err
		}
		f.DestinationPath = destination

		if pathWithinRoot(f.DestinationPath, op.SourceRoot) {
			return errors.New("destination is inside the download deletion tree")
		}
		for _, path := range []string{f.SourcePath, f.DestinationPath} {
			if err := verifyManifestPath(path, f); err != nil {
				return err
			}
		}
		if f.Format != "sidecar" {
			var linked bool
			if err := s.store.db.QueryRowContext(ctx, `select exists(select 1 from files f join file_download_links d on d.file_id=f.id join file_wanted_links w on w.file_id=f.id where f.id::text=$1 and f.path=$2 and d.download_record_id=$3 and w.wanted_item_id=$4)`, f.FileID, f.DestinationPath, op.DownloadRecordID, f.WantedID).Scan(&linked); err != nil {
				return err
			}
			if !linked {
				return errors.New("import file association is missing or changed")
			}
		}
	}
	return s.verifyOperationInventory(ctx, op, true)
}

// RecordCompletedCleanup records the result of an already-authorized remote
// delete. A remote error never rolls back the committed library copy.
func (s *Service) RecordCompletedCleanup(ctx context.Context, download acquisition.DownloadStatus, cleanupErr error) error {
	if !s.Available() {
		return errors.New("cleanup persistence is unavailable")
	}
	state, message := "cleaned", ""
	if cleanupErr != nil {
		state, message = "blocked", cleanupErr.Error()
	}
	_, err := s.store.db.ExecContext(ctx, `update import_operations o set cleanup_state=$3,cleanup_error=$4,updated_at=now() from downloads d where d.id=o.download_record_id and lower(d.client)=lower($1) and d.external_id=$2 and o.state='committed'`, download.Client, download.ID, state, message)
	return err
}
