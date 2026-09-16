package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type scanEntry struct {
	Root string `json:"root"`
	Path string `json:"path"`
	Kind string `json:"kind"`
}

func (s *Service) runScanBatch(ctx context.Context, id string, limit int) (out ScanOutcome, resultErr error) {
	job, err := s.claimScan(ctx, id)
	if err != nil {
		return out, err
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
				if err := s.scanTransaction(runCtx, job, func(*sql.Tx) error { return nil }); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	files := []FileRecord{}
	defer func() {
		cancel()
		<-done
		saveCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		state, message := "queued", ""
		if resultErr != nil && !errors.Is(resultErr, context.Canceled) && !errors.Is(resultErr, context.DeadlineExceeded) {
			state, message = "failed", resultErr.Error()
		}
		if errors.Is(resultErr, ErrScanCancelled) {
			state, message = "cancelled", ""
		}
		_, saveErr := s.store.db.ExecContext(saveCtx, `update library_scan_jobs set state=case when cancel_requested then 'cancelled' else $3 end,last_error=$4,lease_token=null,lease_expires_at=null,updated_at=now() where id=$1 and lease_token=$2 and state='running'`, job.ID, job.LeaseToken, state, message)
		current, readErr := s.GetScan(saveCtx, job.ID)
		if readErr == nil {
			out = scanOutcome(current, files)
		}
		resultErr = errors.Join(resultErr, saveErr, readErr)
	}()
	if err := verifyScanRoots(job); err != nil {
		return out, err
	}
	for processed := 0; processed < limit; processed++ {
		if err := runCtx.Err(); err != nil {
			return out, err
		}
		if job.Phase == "discover" {
			var entry scanEntry
			err := s.store.db.QueryRowContext(runCtx, `select root_path,path,kind from library_scan_entries where job_id=$1 and not done order by path limit 1`, job.ID).Scan(&entry.Root, &entry.Path, &entry.Kind)
			if errors.Is(err, sql.ErrNoRows) {
				err = s.scanTransaction(runCtx, job, func(tx *sql.Tx) error {
					_, err := tx.ExecContext(runCtx, `update library_scan_jobs set phase='reconcile' where id=$1`, job.ID)
					return err
				})
				if err != nil {
					return out, err
				}
				job.Phase = "reconcile"
				processed--
				continue
			}
			if err != nil {
				return out, err
			}
			if entry.Kind == "directory" {
				if err := s.discoverScanDirectory(runCtx, job, entry); err != nil {
					return out, err
				}
				continue
			}
			record, err := s.observeScanEntry(runCtx, job, entry)
			if err != nil {
				return out, err
			}
			if record != nil {
				files = append(files, *record)
			}
		} else {
			file, found, err := s.nextScanReconciliation(runCtx, job)
			if err != nil {
				return out, err
			}
			if !found {
				return out, s.completeScan(runCtx, job)
			}
			if err := s.reconcileScanFile(runCtx, job, file); err != nil {
				return out, err
			}
			job.ReconcileAfter = file.ID
		}
	}
	return out, nil
}
func (s *Service) discoverScanDirectory(ctx context.Context, job ScanJob, entry scanEntry) error {
	if err := verifyScanRoots(job); err != nil {
		return err
	}
	if err := scanPathWithinRoot(entry.Path, entry.Root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.finishSkippedScanEntry(ctx, job, entry)
		}
		return err
	}
	info, err := os.Lstat(entry.Path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("queued scan directory is no longer a directory")
	}
	dir, err := os.Open(entry.Path)
	if err != nil {
		return err
	}
	defer dir.Close()
	for {
		entries, readErr := dir.ReadDir(256)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		children := []scanEntry{}
		for _, child := range entries {
			kind := "file"
			if child.IsDir() {
				kind = "directory"
			}
			path := filepath.Join(entry.Path, child.Name())
			if kind == "directory" && s.Config().RecycleBin != "" && filepath.Clean(s.Config().RecycleBin) == path {
				continue
			}
			children = append(children, scanEntry{entry.Root, path, kind})
		}
		raw, err := json.Marshal(children)
		if err != nil {
			return err
		}
		if err := verifyScanRoots(job); err != nil {
			return err
		}
		if err := s.scanTransaction(ctx, job, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `insert into library_scan_entries(job_id,root_path,path,kind) select $1,root,path,kind from jsonb_to_recordset($2::jsonb) as item(root text,path text,kind text) on conflict do nothing`, job.ID, string(raw)); err != nil {
				return err
			}
			if errors.Is(readErr, io.EOF) {
				_, err := tx.ExecContext(ctx, `update library_scan_entries set done=true where job_id=$1 and path=$2`, job.ID, entry.Path)
				return err
			}
			return nil
		}); err != nil {
			return err
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
	}
}
func (s *Service) observeScanEntry(ctx context.Context, job ScanJob, entry scanEntry) (*FileRecord, error) {
	info, err := os.Lstat(entry.Path)
	if errors.Is(err, os.ErrNotExist) {
		if err := verifyScanRoots(job); err != nil {
			return nil, err
		}
		return nil, s.finishSkippedScanEntry(ctx, job, entry)
	}
	if err != nil {
		return nil, err
	}
	format, supported := classifyFile(entry.Path)
	if !info.Mode().IsRegular() || !supported || !formatAllowed(job.Format, format) {
		return nil, s.finishSkippedScanEntry(ctx, job, entry)
	}
	if err := scanPathWithinRoot(entry.Path, entry.Root); err != nil {
		return nil, err
	}
	checksum, err := scanContentHash(ctx, entry.Path)
	if err != nil {
		return nil, err
	}
	observedPath := entry.Path
	aliases := []string{entry.Path}
	for _, root := range job.rootIdentities {
		if root.Path == entry.Root && root.OriginalPath != "" {
			relative, err := filepath.Rel(root.Path, entry.Path)
			if err != nil {
				return nil, err
			}
			aliases = append(aliases, filepath.Join(root.OriginalPath, relative))
		}
	}
	existing, err := s.store.FindFiles(ctx, nil, aliases)
	if err != nil {
		return nil, err
	}
	if len(existing) > 1 {
		return nil, errors.New("multiple records identify the same scan path; review required")
	}
	if len(existing) == 1 {
		observedPath = existing[0].Path
	}
	record := fileRecordFromPath(observedPath, format, info, "available")
	record.Checksum = checksum
	after, err := os.Lstat(entry.Path)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, after) || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
		return nil, errors.New("file changed while scanning; retry the scan")
	}
	device, err := scanFileDevice(entry.Path)
	if err != nil {
		return nil, err
	}
	if err := verifyScanRoots(job); err != nil {
		return nil, err
	}
	var stored FileRecord
	err = s.scanTransaction(ctx, job, func(tx *sql.Tx) error {
		// Shares the import planner's path lock; an unfinished import stays hidden.
		if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,1))`, entry.Path); err != nil {
			return err
		}
		var pending bool
		if err := tx.QueryRowContext(ctx, `select exists(select 1 from import_operation_files f join import_operations o on o.id=f.operation_id where f.destination_path=$1 and o.state<>'committed')`, entry.Path).Scan(&pending); err != nil {
			return err
		}
		if pending {
			return markScanEntryDone(ctx, tx, job, entry, true)
		}
		var err error
		stored, err = persistFile(ctx, tx, record, true)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `update files set presence_state='present',scan_root=$2,scan_device=$4,last_seen_scan_id=$3 where id=$1`, stored.ID, entry.Root, job.ID, device); err != nil {
			return err
		}
		stored.PresenceState = "present"
		return markScanEntryDone(ctx, tx, job, entry, false)
	})
	if err != nil {
		return nil, err
	}
	if stored.ID == "" {
		return nil, nil
	}
	return &stored, nil
}
func markScanEntryDone(ctx context.Context, tx *sql.Tx, job ScanJob, entry scanEntry, skipped bool) error {
	result, err := tx.ExecContext(ctx, `update library_scan_entries set done=true where job_id=$1 and path=$2 and not done`, job.ID, entry.Path)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	if skipped {
		_, err = tx.ExecContext(ctx, `update library_scan_jobs set skipped=skipped+1 where id=$1`, job.ID)
	} else {
		_, err = tx.ExecContext(ctx, `update library_scan_jobs set scanned=scanned+1,upserted=upserted+1 where id=$1`, job.ID)
	}
	return err
}
func (s *Service) finishSkippedScanEntry(ctx context.Context, job ScanJob, entry scanEntry) error {
	return s.scanTransaction(ctx, job, func(tx *sql.Tx) error { return markScanEntryDone(ctx, tx, job, entry, true) })
}
func scanContentHash(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	buf := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := file.Read(buf)
		if n > 0 {
			_, _ = hash.Write(buf[:n])
		}
		if errors.Is(err, io.EOF) {
			return hex.EncodeToString(hash.Sum(nil)), nil
		}
		if err != nil {
			return "", err
		}
	}
}

type reconcileScanRecord struct {
	ID, Path, Root, Device string
	UpdatedAt              time.Time
}

func (s *Service) nextScanReconciliation(ctx context.Context, job ScanJob) (reconcileScanRecord, bool, error) {
	var file reconcileScanRecord
	err := s.store.db.QueryRowContext(ctx, `select id::text,path,scan_root,scan_device,updated_at from files where scan_root in (select item->>'path' from library_scan_jobs j,jsonb_array_elements(j.roots) item where j.id=$1) and last_seen_scan_id is not null and last_seen_scan_id<>$1 and ($2='' or id>nullif($2,'')::uuid) and ($3='any' or media_format=$3) order by id limit 1`, job.ID, job.ReconcileAfter, job.Format).Scan(&file.ID, &file.Path, &file.Root, &file.Device, &file.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return file, false, nil
	}
	return file, err == nil, err
}
func (s *Service) reconcileScanFile(ctx context.Context, job ScanJob, file reconcileScanRecord) error {
	if err := verifyScanRoots(job); err != nil {
		return err
	}
	canonical, err := canonicalPlannedPath(file.Path)
	if err != nil {
		return err
	}
	info, err := os.Lstat(file.Path)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return err
	}
	if !missing {
		if !info.Mode().IsRegular() {
			return errors.New("previously scanned file is no longer regular; review required")
		}
		// A file added behind an already-enumerated directory is still present.
		if err := scanPathWithinRoot(canonical, file.Root); err != nil {
			return err
		}
	} else {
		if err := verifyScanAbsence(canonical, file.Root, file.Device); err != nil {
			return err
		}
	}
	return s.scanTransaction(ctx, job, func(tx *sql.Tx) error {
		if missing {
			if _, err := tx.ExecContext(ctx, `insert into library_scan_absent(job_id,file_id,path,observed_updated_at) values($1,$2,$3,$4) on conflict do nothing`, job.ID, file.ID, file.Path, file.UpdatedAt); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `update files set presence_state='present',last_seen_scan_id=$2 where id=$1 and path=$3 and updated_at=$4`, file.ID, job.ID, file.Path, file.UpdatedAt); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `update library_scan_jobs set reconcile_after=$2 where id=$1`, job.ID, file.ID)
		return err
	})
}

// Check every existing ancestor, including the device below nested mounts. A
// disappeared nested filesystem cannot be mistaken for individual deleted files.
func verifyScanAbsence(path, root, expectedDevice string) error {
	if !pathWithinRoot(path, root) || expectedDevice == "" {
		return errors.New("missing-file evidence has no trusted root/device")
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	current := root
	last := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			device, err := scanFileDevice(last)
			if err != nil {
				return err
			}
			if device != expectedDevice {
				return errors.New("filesystem containing the missing file is unavailable")
			}
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("missing-file path crosses a symlink")
		}
		last = current
	}
	return errors.New("file reappeared while checking absence; retry the scan")
}
func (s *Service) completeScan(ctx context.Context, job ScanJob) error {
	if err := verifyScanRoots(job); err != nil {
		return err
	}
	// Recheck staged absences after any batch pause or process restart.
	rows, err := s.store.db.QueryContext(ctx, `select f.id::text,f.path,f.scan_root,f.scan_device from library_scan_absent a join files f on f.id=a.file_id where a.job_id=$1 and f.path=a.path and f.updated_at=a.observed_updated_at`, job.ID)
	if err != nil {
		return err
	}
	reappeared := []string{}
	for rows.Next() {
		var id, path, root, device string
		if err := rows.Scan(&id, &path, &root, &device); err != nil {
			rows.Close()
			return err
		}
		canonical, err := canonicalPlannedPath(path)
		if err != nil {
			rows.Close()
			return err
		}
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			if err := verifyScanAbsence(canonical, root, device); err != nil {
				rows.Close()
				return err
			}
		} else if err != nil {
			rows.Close()
			return err
		} else {
			if !info.Mode().IsRegular() {
				rows.Close()
				return errors.New("staged missing file reappeared with an unsupported type")
			}
			if err := scanPathWithinRoot(canonical, root); err != nil {
				rows.Close()
				return err
			}
			reappeared = append(reappeared, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if err := verifyScanRoots(job); err != nil {
		return err
	}
	return s.scanTransaction(ctx, job, func(tx *sql.Tx) error {
		for _, id := range reappeared {
			if _, err := tx.ExecContext(ctx, `update files f set presence_state='present',last_seen_scan_id=$1 from library_scan_absent a where a.job_id=$1 and a.file_id=f.id and f.id=$2 and f.path=a.path and f.updated_at=a.observed_updated_at`, job.ID, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `delete from library_scan_absent where job_id=$1 and file_id=$2`, job.ID, id); err != nil {
				return err
			}
		}
		result, err := tx.ExecContext(ctx, `update files f set presence_state='missing',updated_at=now() from library_scan_absent a where a.job_id=$1 and a.file_id=f.id and f.path=a.path and f.updated_at=a.observed_updated_at and f.last_seen_scan_id<>$1 and not exists(select 1 from import_operation_files m join import_operations o on o.id=m.operation_id where m.destination_path=f.path and o.state<>'committed')`, job.ID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		for _, r := range job.rootIdentities {
			if _, err := tx.ExecContext(ctx, `insert into library_scan_roots(path,identity,completed_job_id) values($1,$2,$3) on conflict(path) do update set identity=excluded.identity,completed_job_id=excluded.completed_job_id,updated_at=now()`, r.Path, r.Identity, job.ID); err != nil {
				return err
			}
		}
		// Keep summary/root evidence without retaining a complete path queue for
		// every successful scan. Failed jobs keep their queues for resumption.
		if _, err := tx.ExecContext(ctx, `delete from library_scan_entries where job_id=$1`, job.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `delete from library_scan_absent where job_id=$1`, job.ID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `update library_scan_jobs set state='completed',phase='complete',missing=$2,lease_token=null,lease_expires_at=null,finished_at=now(),updated_at=now() where id=$1`, job.ID, n)
		return err
	})
}
