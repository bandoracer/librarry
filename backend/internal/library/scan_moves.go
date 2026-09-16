package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type scanMove struct {
	FileID, DiscoveredID, PreviousPath, CurrentPath, Root, Device, PreviousRoot, PreviousDevice, SHA256, Stamp string
	Size                                                                                                       int64
	PreviousUpdated, CurrentUpdated                                                                            time.Time
	observed                                                                                                   os.FileInfo
}

// A content match is eligible only when exactly two records identify those
// bytes: one safely absent old identity and one untouched scan discovery. Manual
// assignments, import references and a third copy all retain separate records.
const scanMoveCandidates = `select f.id::text,p.id::text,f.path,p.path,p.scan_root,p.scan_device,f.scan_root,f.scan_device,f.checksum,f.size_bytes,f.updated_at,p.updated_at,p.scan_file_stamp
from library_scan_absent a join files f on f.id=a.file_id
join files p on p.id<>f.id and p.checksum=f.checksum and p.size_bytes=f.size_bytes and p.media_format=f.media_format
join library_scan_discoveries d on d.file_id=p.id and d.identity=scan_discovery_identity(p)
where a.job_id=$1 and f.path=a.path and f.updated_at=a.observed_updated_at and f.last_seen_scan_id<>$1
 and f.checksum ~ '^[0-9a-f]{64}$' and f.size_bytes>0
 and p.scan_file_stamp<>'' and p.last_seen_scan_id=$1 and p.presence_state='present' and p.import_status='available' and p.edition_id is null
 and not (f.metadata ? 'calibreId') and not (p.metadata ? 'calibreId')
 and (select count(*) from files same where same.checksum=f.checksum and same.size_bytes=f.size_bytes and same.media_format=f.media_format)=2
 and not exists(select 1 from file_wanted_links l where l.file_id=p.id)
 and not exists(select 1 from file_download_links l where l.file_id=p.id)
 and not exists(select 1 from downloads d where d.imported_file_id=p.id)
 and not exists(select 1 from import_operation_files m where m.file_id=p.id)
 and not exists(select 1 from import_operation_files m join import_operations o on o.id=m.operation_id where o.state<>'committed' and (m.destination_path in(f.path,p.path) or m.file_id=f.id))`

type scanMoveReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readScanMoves(ctx context.Context, db scanMoveReader, jobID string) ([]scanMove, error) {
	rows, err := db.QueryContext(ctx, scanMoveCandidates+` order by f.id`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []scanMove{}
	for rows.Next() {
		var m scanMove
		if err := rows.Scan(&m.FileID, &m.DiscoveredID, &m.PreviousPath, &m.CurrentPath, &m.Root, &m.Device, &m.PreviousRoot, &m.PreviousDevice, &m.SHA256, &m.Size, &m.PreviousUpdated, &m.CurrentUpdated, &m.Stamp); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
func scanCalibreRoots(ctx context.Context, db scanMoveReader) ([]string, error) {
	rows, err := db.QueryContext(ctx, `select path from root_folders where use_calibre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roots := []string{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		canonical, err := canonicalPlannedPath(path)
		if err != nil {
			return nil, fmt.Errorf("cannot verify Calibre root boundary: %w", err)
		}
		roots = append(roots, canonical, path)
	}
	return roots, rows.Err()
}
func moveTouchesCalibre(m scanMove, roots []string) bool {
	previous, err := canonicalPlannedPath(m.PreviousPath)
	if err != nil {
		return true
	}
	for _, root := range roots {
		if pathWithinRoot(previous, root) || pathWithinRoot(m.PreviousPath, root) || pathWithinRoot(m.CurrentPath, root) {
			return true
		}
	}
	return false
}
func verifyMoveAbsence(m scanMove) error {
	canonical, err := canonicalPlannedPath(m.PreviousPath)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(m.PreviousPath); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return err
		}
		return errors.New("previous file path reappeared during move reconciliation")
	}
	return verifyScanAbsence(canonical, m.PreviousRoot, m.PreviousDevice)
}
func verifyMoveDestination(m scanMove) (os.FileInfo, error) {
	if err := scanPathWithinRoot(m.CurrentPath, m.Root); err != nil {
		return nil, err
	}
	info, err := os.Lstat(m.CurrentPath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() != m.Size {
		return nil, errors.New("moved-file candidate changed size or type")
	}
	device, err := scanFileDevice(m.CurrentPath)
	if err != nil {
		return nil, err
	}
	if device != m.Device {
		return nil, errors.New("moved-file candidate filesystem changed")
	}
	return info, nil
}
func (s *Service) prepareScanMoves(ctx context.Context, job ScanJob) ([]scanMove, error) {
	candidates, err := readScanMoves(ctx, s.store.db, job.ID)
	if err != nil || len(candidates) == 0 {
		return candidates, err
	}
	roots, err := scanCalibreRoots(ctx, s.store.db)
	if err != nil {
		return nil, err
	}
	verified := []scanMove{}
	for _, m := range candidates {
		if moveTouchesCalibre(m, roots) {
			continue
		}
		// A previously staged absence may have reappeared. completeScan handles that
		// as present; it must never become a move just because the saved hashes match.
		if _, err := os.Lstat(m.PreviousPath); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err := verifyMoveAbsence(m); err != nil {
			return nil, err
		}
		after, err := verifyMoveDestination(m)
		if err != nil {
			return nil, err
		}
		stamp, err := scanFileStamp(after)
		if err != nil {
			return nil, err
		}
		// Discovery already hashed every file in a resumable batch. Bind that
		// saved hash to the same inode/size/nanosecond mtime, avoiding a second
		// unbounded full-library hash pass at completion.
		if stamp != m.Stamp {
			return nil, errors.New("moved-file candidate changed after scan hashing; start a fresh scan")
		}
		m.observed = after
		verified = append(verified, m)
	}
	return verified, verifyScanRoots(job)
}

// Invoked within the completion transaction and scan ownership fence. Lock
// candidate rows before checking references so concurrent manual assignments
// cannot be silently cascaded away with the temporary discovery record.
func (s *Service) commitScanMoves(ctx context.Context, tx *sql.Tx, job ScanJob, moves []scanMove) (int, error) {
	if len(moves) == 0 {
		return 0, nil
	}
	paths := []string{}
	ids := []string{}
	for _, m := range moves {
		paths = append(paths, m.PreviousPath, m.CurrentPath)
		ids = append(ids, m.FileID, m.DiscoveredID)
	}
	sort.Strings(paths)
	sort.Strings(ids)
	for _, path := range paths {
		if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,1))`, path); err != nil {
			return 0, err
		}
	}
	for _, id := range ids {
		var locked string
		if err := tx.QueryRowContext(ctx, `select id::text from files where id=$1 for update`, id).Scan(&locked); err != nil {
			return 0, fmt.Errorf("move identity changed before commit: %w", err)
		}
	}
	current, err := readScanMoves(ctx, tx, job.ID)
	if err != nil {
		return 0, err
	}
	eligible := map[string]scanMove{}
	for _, m := range current {
		eligible[m.FileID] = m
	}
	roots, err := scanCalibreRoots(ctx, tx)
	if err != nil {
		return 0, err
	}
	moved := 0
	for _, m := range moves {
		actual, ok := eligible[m.FileID]
		if !ok || actual.DiscoveredID != m.DiscoveredID || actual.PreviousPath != m.PreviousPath || actual.CurrentPath != m.CurrentPath || !actual.PreviousUpdated.Equal(m.PreviousUpdated) || !actual.CurrentUpdated.Equal(m.CurrentUpdated) || actual.SHA256 != m.SHA256 || actual.Size != m.Size || actual.Stamp != m.Stamp || actual.Root != m.Root || actual.Device != m.Device || actual.PreviousRoot != m.PreviousRoot || actual.PreviousDevice != m.PreviousDevice || moveTouchesCalibre(actual, roots) {
			continue
		}
		if err := verifyMoveAbsence(m); err != nil {
			return 0, err
		}
		info, err := verifyMoveDestination(m)
		if err != nil {
			return 0, err
		}
		if !os.SameFile(info, m.observed) || !info.ModTime().Equal(m.observed.ModTime()) {
			return 0, errors.New("moved-file candidate changed before commit")
		}
		// Delete only the unassigned discovery. All existing links/metadata belong to
		// the original row and are retained. No files on disk are renamed or removed.
		if _, err := tx.ExecContext(ctx, `delete from files where id=$1`, m.DiscoveredID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `update files set path=$2,scan_root=$3,scan_device=$4,last_seen_scan_id=$5,presence_state='present',modified_at=$6,extension=$7,scan_file_stamp=$8,updated_at=now() where id=$1`, m.FileID, m.CurrentPath, m.Root, m.Device, job.ID, info.ModTime(), strings.ToLower(filepath.Ext(m.CurrentPath)), m.Stamp); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `insert into library_scan_moves(job_id,file_id,discovered_file_id,previous_path,current_path,sha256,size_bytes) values($1,$2,$3,$4,$5,$6,$7)`, job.ID, m.FileID, m.DiscoveredID, m.PreviousPath, m.CurrentPath, m.SHA256, m.Size); err != nil {
			return 0, err
		}
		// It is now a retained identity, not a disposable discovery on future scans.
		if _, err := tx.ExecContext(ctx, `delete from library_scan_discoveries where file_id=$1`, m.FileID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `delete from library_scan_absent where job_id=$1 and file_id=$2`, job.ID, m.FileID); err != nil {
			return 0, err
		}
		moved++
	}
	return moved, verifyScanRoots(job)
}
