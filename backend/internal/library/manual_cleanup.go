package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func verifyPreviousImportFile(file ImportOperationFile) error {
	previous := file
	previous.SizeBytes, previous.SHA256 = file.PreviousSizeBytes, file.PreviousSHA256
	return verifyManifestPath(file.PreviousPath, previous)
}

// The new stage has already passed its manifest hash before this function runs.
// Link the old inode to its persisted recovery path before removing its name.
func (s *Service) prepareImportReplacement(ctx context.Context, op ImportOperation, file ImportOperationFile) error {
	if filepath.Dir(file.PreviousPath) != filepath.Dir(file.DestinationPath) || file.PreviousPath == file.SourcePath {
		return errors.New("invalid replacement backup path")
	}
	previous := file
	previous.SizeBytes, previous.SHA256 = file.PreviousSizeBytes, file.PreviousSHA256
	if _, err := os.Lstat(file.PreviousPath); errors.Is(err, os.ErrNotExist) {
		if err := verifyManifestPath(file.DestinationPath, previous); err != nil {
			return fmt.Errorf("previous destination changed: %w", err)
		}
		if err := os.Link(file.DestinationPath, file.PreviousPath); err != nil {
			return err
		}
		if err := syncImportDirectory(filepath.Dir(file.PreviousPath)); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := verifyPreviousImportFile(file); err != nil {
		return err
	}
	if _, err := os.Lstat(file.DestinationPath); err == nil {
		if err := verifyManifestPath(file.DestinationPath, previous); err != nil {
			return fmt.Errorf("destination changed during replacement: %w", err)
		}
		if err := os.Remove(file.DestinationPath); err != nil {
			return err
		}
		if err := syncImportDirectory(filepath.Dir(file.DestinationPath)); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Service) finishManualOperation(ctx context.Context, op ImportOperation, outcome ImportOutcome) (ImportOutcome, error) {
	if op.SourceKind != "manual" {
		return outcome, nil
	}
	outcome.ConflictAction, _ = op.Metadata["conflictAction"].(string)
	outcome.ConflictPath, _ = op.Metadata["conflictPath"].(string)
	outcome.Hardlinked, _ = outcome.File.Metadata["hardlinked"].(bool)
	for _, file := range op.Files {
		outcome.Replaced = outcome.Replaced || file.PreviousPath != ""
	}
	if op.CleanupState == "cleaned" {
		outcome.Moved = manualOperationMoved(op)
		return outcome, nil
	}
	err := s.store.db.QueryRowContext(ctx, `update import_operations set lease_token=gen_random_uuid(),lease_expires_at=now()+interval '2 minutes'
 where id=$1 and source_kind='manual' and state='committed' and cleanup_state<>'cleaned' and (lease_expires_at is null or lease_expires_at<now()) returning lease_token::text`, op.ID).Scan(&op.LeaseToken)
	if errors.Is(err, sql.ErrNoRows) {
		return outcome, ErrImportBusy
	}
	if err != nil {
		return outcome, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if err := s.store.renewOperation(runCtx, op.ID, op.LeaseToken); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-done }()
	if err := s.cleanupManualFiles(runCtx, op); err != nil {
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, saveErr := s.store.db.ExecContext(saveCtx, `update import_operations set cleanup_state='blocked',cleanup_error=$3,lease_token=null,lease_expires_at=null,updated_at=now() where id=$1 and lease_token=$2`, op.ID, op.LeaseToken, err.Error())
		return outcome, errors.Join(fmt.Errorf("import committed; cleanup needs retry: %w", err), saveErr)
	}
	result, err := s.store.db.ExecContext(ctx, `with moved as (
 update files f set metadata=f.metadata||'{"move":true,"importMode":"move"}'::jsonb from import_operation_files m,import_operations o
 where m.operation_id=o.id and m.file_id=f.id and m.source_removed and o.id=$1 and o.lease_token=$2 and o.lease_expires_at>now() and f.metadata->>'importOperationId'=o.id::text
 ) update import_operations set cleanup_state='cleaned',cleanup_error='',lease_token=null,lease_expires_at=null,updated_at=now() where id=$1 and lease_token=$2 and lease_expires_at>now()`, op.ID, op.LeaseToken)
	if err != nil {
		return outcome, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return outcome, err
	}
	if n != 1 {
		return outcome, ErrImportBusy
	}
	outcome.Moved = manualOperationMoved(op)
	// Reflect the committed cleanup in this response as well as future reads.
	if op.Mode == "move" {
		for _, file := range op.Files {
			if file.SourcePath == file.DestinationPath {
				continue
			}
			for i := range outcome.Files {
				if outcome.Files[i].Path == file.DestinationPath {
					outcome.Files[i].Metadata["move"] = true
					outcome.Files[i].Metadata["importMode"] = "move"
				}
			}
		}
		if len(outcome.Files) > 0 {
			outcome.File = outcome.Files[0]
		}
	}
	return outcome, nil
}

func (s *Service) cleanupManualFiles(ctx context.Context, op ImportOperation) error {
	// Verify the complete set before deleting any original or backup.
	for _, file := range op.Files {
		if err := verifyManifestPath(file.DestinationPath, file); err != nil {
			return err
		}
	}
	for _, file := range op.Files {
		if err := s.store.withOperationFence(ctx, op, func(tx *sql.Tx) error {
			if err := verifyManifestPath(file.DestinationPath, file); err != nil {
				return err
			}
			if op.Mode == "move" && !file.SourceRemoved && file.SourcePath != file.DestinationPath {
				if _, err := os.Lstat(file.SourcePath); err == nil {
					if err := verifyManifestPath(file.SourcePath, file); err != nil {
						return err
					}
					if err := os.Remove(file.SourcePath); err != nil {
						return err
					}
					if err := syncImportDirectory(filepath.Dir(file.SourcePath)); err != nil {
						return err
					}
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
				if _, err := tx.ExecContext(ctx, `update import_operation_files set source_removed=true where id=$1 and operation_id=$2`, file.ID, op.ID); err != nil {
					return err
				}
			}
			if file.PreviousPath != "" {
				if _, err := os.Lstat(file.PreviousPath); err == nil {
					if err := verifyPreviousImportFile(file); err != nil {
						return err
					}
					if err := s.discardFile(file.PreviousPath); err != nil {
						return err
					}
					if err := syncImportDirectory(filepath.Dir(file.PreviousPath)); err != nil {
						return err
					}
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func manualOperationMoved(op ImportOperation) bool {
	if op.Mode != "move" {
		return false
	}
	for _, file := range op.Files {
		if file.SourcePath != file.DestinationPath {
			return true
		}
	}
	return false
}
