package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func manifestStagePath(file ImportOperationFile, token string) string {
	return filepath.Join(filepath.Dir(file.DestinationPath), ".librarry-stage-"+file.ID+"-"+token)
}

func (s *Store) journalImportStage(ctx context.Context, op ImportOperation, file ImportOperationFile, path string) error {
	result, err := s.db.ExecContext(ctx, `update import_operation_files f set stage_path=$4,stage_lease_token=$3,updated_at=now()
 from import_operations o where f.operation_id=o.id and o.id=$1 and f.id=$2 and f.stage_path=''
 and o.lease_token=$3 and o.lease_expires_at>now() and o.state='transferring'`, op.ID, file.ID, op.LeaseToken, path)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrImportBusy
	}
	return nil
}

func (s *Store) clearImportStage(ctx context.Context, op ImportOperation, file ImportOperationFile) error {
	result, err := s.db.ExecContext(ctx, `update import_operation_files f set stage_path='',stage_lease_token=null,updated_at=now()
 from import_operations o where f.operation_id=o.id and o.id=$1 and f.id=$2 and f.stage_path=$4
 and o.lease_token=$3 and o.lease_expires_at>now() and o.state='transferring'`, op.ID, file.ID, op.LeaseToken, file.StagePath)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrImportBusy
	}
	return nil
}

func syncImportDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr, closeErr := dir.Sync(), dir.Close()
	return errors.Join(syncErr, closeErr)
}

// Never infer ownership from a filename glob. Each reclaimable path is recorded
// before creation, includes its file and lease UUIDs, and is outside the source.
func (s *Service) reclaimImportStage(ctx context.Context, op ImportOperation, file ImportOperationFile) error {
	if file.StagePath == "" {
		return nil
	}
	if file.StageLeaseToken == "" || file.StagePath != manifestStagePath(file, file.StageLeaseToken) ||
		!pathWithinRoot(file.StagePath, op.DestinationRoot) || pathWithinRoot(file.StagePath, op.SourceRoot) {
		return errors.New("invalid recorded import staging path; retain for review")
	}
	if err := s.store.renewOperation(ctx, op.ID, op.LeaseToken); err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(file.StagePath))
	if err != nil || parent != filepath.Dir(file.StagePath) {
		return errors.New("staging directory is unavailable or crosses a symlink")
	}
	if info, err := os.Lstat(file.StagePath); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("recorded import stage is not a regular file; retain for review")
		}
		if err := os.Remove(file.StagePath); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := syncImportDirectory(parent); err != nil {
		return err
	}
	return s.store.clearImportStage(ctx, op, file)
}

type importContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r importContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func copyManifestStage(ctx context.Context, file ImportOperationFile, path string) (resultErr error) {
	source, err := os.Open(file.SourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, destination.Close()) }()
	digest := sha256.New()
	size, err := io.Copy(destination, io.TeeReader(importContextReader{ctx, io.LimitReader(source, file.SizeBytes+1)}, digest))
	if err != nil {
		return err
	}
	if size != file.SizeBytes || hex.EncodeToString(digest.Sum(nil)) != file.SHA256 {
		return errors.New("source bytes changed during staged copy")
	}
	return destination.Sync()
}

func (s *Service) transferOperationFile(ctx context.Context, op ImportOperation, file ImportOperationFile) error {
	path := manifestStagePath(file, op.LeaseToken)
	if _, err := os.Lstat(path); err == nil {
		return errors.New("unowned staging path already exists; retain for review")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := s.store.journalImportStage(ctx, op, file, path); err != nil {
		return err
	}
	file.StagePath, file.StageLeaseToken = path, op.LeaseToken
	if op.Mode == "hardlink" || op.Mode == "hardlinkOrCopy" {
		if err := os.Link(file.SourcePath, path); err != nil {
			if op.Mode == "hardlink" {
				return err
			}
			if err := copyManifestStage(ctx, file, path); err != nil {
				return err
			}
		}
	} else if err := copyManifestStage(ctx, file, path); err != nil {
		return err
	}
	if err := verifyManifestPath(path, file); err != nil {
		return err
	}
	// Renew after the potentially slow copy/hash. A worker whose lease expired
	// cannot publish after another worker has taken over.
	if err := s.store.renewOperation(ctx, op.ID, op.LeaseToken); err != nil {
		return err
	}
	if err := os.Link(path, file.DestinationPath); err != nil {
		return fmt.Errorf("publish verified import: %w", err)
	}
	if err := syncImportDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	return s.reclaimImportStage(ctx, op, file)
}
