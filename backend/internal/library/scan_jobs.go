package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"
)

var ErrScanBusy = errors.New("scan is already running")
var ErrScanCancelled = errors.New("scan was cancelled")

// Root identity is stored separately from the public JSON representation.
type storedScanRoot struct {
	OriginalPath string `json:"originalPath"`
	Path         string `json:"path"`
	Identity     string `json:"identity"`
}
type ScanJob struct {
	ID              string     `json:"id"`
	Format          string     `json:"format"`
	Roots           []string   `json:"roots"`
	State           string     `json:"state"`
	Phase           string     `json:"phase"`
	CancelRequested bool       `json:"cancelRequested"`
	Scanned         int        `json:"scanned"`
	Upserted        int        `json:"upserted"`
	Skipped         int        `json:"skipped"`
	Missing         int        `json:"missing"`
	Moved           int        `json:"moved"`
	LastError       string     `json:"lastError,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	LeaseExpiresAt  *time.Time `json:"leaseExpiresAt,omitempty"`
	LeaseToken      string     `json:"-"`
	ReconcileAfter  string     `json:"-"`
	rootIdentities  []storedScanRoot
}

const scanJobColumns = `id::text,media_format,roots,state,phase,cancel_requested,scanned,upserted,skipped,missing,moved,last_error,created_at,updated_at,lease_expires_at,coalesce(lease_token::text,''),coalesce(reconcile_after::text,'')`

type scanJobScanner interface{ Scan(...any) error }

func readScanJob(row scanJobScanner) (ScanJob, error) {
	var j ScanJob
	var raw []byte
	err := row.Scan(&j.ID, &j.Format, &raw, &j.State, &j.Phase, &j.CancelRequested, &j.Scanned, &j.Upserted, &j.Skipped, &j.Missing, &j.Moved, &j.LastError, &j.CreatedAt, &j.UpdatedAt, &j.LeaseExpiresAt, &j.LeaseToken, &j.ReconcileAfter)
	if err != nil {
		return j, err
	}
	if err := json.Unmarshal(raw, &j.rootIdentities); err != nil {
		return j, err
	}
	j.Roots = []string{}
	for _, r := range j.rootIdentities {
		j.Roots = append(j.Roots, r.Path)
	}
	return j, nil
}
func (s *Service) GetScan(ctx context.Context, id string) (ScanJob, error) {
	if !s.Available() {
		return ScanJob{}, errors.New("library scans require database persistence")
	}
	return readScanJob(s.store.db.QueryRowContext(ctx, `select `+scanJobColumns+` from library_scan_jobs where id=$1`, id))
}
func (s *Service) ListScans(ctx context.Context) ([]ScanJob, error) {
	if !s.Available() {
		return []ScanJob{}, nil
	}
	rows, err := s.store.db.QueryContext(ctx, `select `+scanJobColumns+` from library_scan_jobs order by (state in ('queued','running','failed')) desc,created_at desc limit 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ScanJob{}
	for rows.Next() {
		j, err := readScanJob(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, j)
	}
	return result, rows.Err()
}
func (s *Service) StartScan(ctx context.Context, request ScanRequest) (ScanJob, error) {
	if !s.Available() {
		return ScanJob{}, errors.New("library scans require database persistence")
	}
	format := normalizeFormat(request.Format)
	if format == "" {
		format = "any"
	}
	if format != "any" && format != "ebook" && format != "audiobook" {
		return ScanJob{}, errors.New("scan format must be any, ebook or audiobook")
	}
	roots := []storedScanRoot{}
	seen := map[string]bool{}
	for _, path := range s.scanRoots(ctx, request) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return ScanJob{}, err
		}
		canonical, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return ScanJob{}, fmt.Errorf("scan root unavailable: %w", err)
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		identity, err := scanRootIdentity(canonical)
		if err != nil {
			return ScanJob{}, err
		}
		roots = append(roots, storedScanRoot{Path: canonical, Identity: identity, OriginalPath: abs})
	}
	if len(roots) == 0 {
		return ScanJob{}, errors.New("no library roots are configured")
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Path < roots[j].Path })
	paths := []string{}
	for _, r := range roots {
		paths = append(paths, r.Path)
	}
	scopeRaw, _ := json.Marshal(struct {
		Format string
		Paths  []string
	}{format, paths})
	sum := sha256.Sum256(scopeRaw)
	scope := hex.EncodeToString(sum[:])
	raw, err := json.Marshal(roots)
	if err != nil {
		return ScanJob{}, err
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return ScanJob{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,5))`, scope); err != nil {
		return ScanJob{}, err
	}
	existing, err := readScanJob(tx.QueryRowContext(ctx, `select `+scanJobColumns+` from library_scan_jobs where scope_key=$1 and state in ('queued','running')`, scope))
	if err == nil {
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ScanJob{}, err
	}
	for _, r := range roots {
		var old string
		err := tx.QueryRowContext(ctx, `select identity from library_scan_roots where path=$1`, r.Path).Scan(&old)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return ScanJob{}, err
		}
		if err == nil && old != r.Identity && !request.AcceptRootChange {
			return ScanJob{}, fmt.Errorf("scan root identity changed: %s; verify the mounted library before accepting the new root", r.Path)
		}
	}
	job, err := readScanJob(tx.QueryRowContext(ctx, `insert into library_scan_jobs(scope_key,media_format,roots) values($1,$2,$3::jsonb) returning `+scanJobColumns, scope, format, string(raw)))
	if err != nil {
		return ScanJob{}, err
	}
	for _, r := range roots {
		if _, err := tx.ExecContext(ctx, `insert into library_scan_entries(job_id,root_path,path,kind) values($1,$2,$2,'directory')`, job.ID, r.Path); err != nil {
			return ScanJob{}, err
		}
	}
	return job, tx.Commit()
}
func (s *Service) CancelScan(ctx context.Context, id string) (ScanJob, error) {
	if !s.Available() {
		return ScanJob{}, errors.New("library scans require database persistence")
	}
	_, err := s.store.db.ExecContext(ctx, `update library_scan_jobs set cancel_requested=true,state=case when lease_expires_at>clock_timestamp() then state else 'cancelled' end,updated_at=now() where id=$1 and state in ('queued','running','failed')`, id)
	if err != nil {
		return ScanJob{}, err
	}
	return s.GetScan(ctx, id)
}
func (s *Service) RetryScan(ctx context.Context, id string) (ScanJob, error) {
	j, err := s.GetScan(ctx, id)
	if err != nil {
		return j, err
	}
	if j.State != "failed" {
		return j, errors.New("only a failed scan can be retried")
	}
	if err := verifyScanRoots(j); err != nil {
		return j, err
	}
	result, err := s.store.db.ExecContext(ctx, `update library_scan_jobs set state='queued',last_error='',lease_token=null,lease_expires_at=null,cancel_requested=false,updated_at=now() where id=$1 and state='failed'`, id)
	if err != nil {
		return j, err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return j, ErrScanBusy
	}
	return s.GetScan(ctx, id)
}
func verifyScanRoots(job ScanJob) error {
	for _, root := range job.rootIdentities {
		identity, err := scanRootIdentity(root.Path)
		if err != nil {
			return fmt.Errorf("scan root unavailable: %w", err)
		}
		if identity != root.Identity {
			return fmt.Errorf("scan root changed during scan: %s", root.Path)
		}
	}
	return nil
}
func scanPathWithinRoot(path, root string) error {
	if !pathWithinRoot(path, root) {
		return errors.New("scan path escapes its root")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if resolved != path {
		return errors.New("scan path crosses a symlink")
	}
	return nil
}

func (s *Service) claimScan(ctx context.Context, id string) (ScanJob, error) {
	job, err := readScanJob(s.store.db.QueryRowContext(ctx, `update library_scan_jobs set state='running',lease_token=gen_random_uuid(),lease_expires_at=clock_timestamp()+interval '2 minutes',updated_at=now() where id=$1 and state in ('queued','running') and not cancel_requested and (lease_expires_at is null or lease_expires_at<clock_timestamp()) returning `+scanJobColumns, id))
	if errors.Is(err, sql.ErrNoRows) {
		return job, ErrScanBusy
	}
	return job, err
}
func (s *Service) scanTransaction(ctx context.Context, job ScanJob, action func(*sql.Tx) error) error {
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var valid, cancel bool
	if err := tx.QueryRowContext(ctx, `select coalesce(lease_token=$2::uuid and lease_expires_at>clock_timestamp(),false),cancel_requested from library_scan_jobs where id=$1 for update`, job.ID, job.LeaseToken).Scan(&valid, &cancel); err != nil {
		return err
	}
	if !valid {
		return ErrScanBusy
	}
	if cancel {
		return ErrScanCancelled
	}
	if err := action(tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `update library_scan_jobs set lease_expires_at=clock_timestamp()+interval '2 minutes',updated_at=now() where id=$1 and lease_token=$2`, job.ID, job.LeaseToken); err != nil {
		return err
	}
	return tx.Commit()
}

// Advance at most one batch. Production registers this as a frequent scheduler
// task; no detached goroutine is owned by an HTTP request.
func (s *Service) RunPendingScans(ctx context.Context) error {
	if !s.Available() {
		return errors.New("library scans require database persistence")
	}
	if _, err := s.store.db.ExecContext(ctx, `update library_scan_jobs set state='cancelled',lease_token=null,lease_expires_at=null,finished_at=now(),updated_at=now() where cancel_requested and state in ('queued','running') and (lease_expires_at is null or lease_expires_at<clock_timestamp())`); err != nil {
		return err
	}
	for _, table := range []string{"library_scan_entries", "library_scan_absent"} {
		if _, err := s.store.db.ExecContext(ctx, `delete from `+table+` e using library_scan_jobs j where e.job_id=j.id and j.state='cancelled'`); err != nil {
			return err
		}
	}
	var id string
	err := s.store.db.QueryRowContext(ctx, `select id::text from library_scan_jobs where state in ('queued','running') and not cancel_requested and (lease_expires_at is null or lease_expires_at<clock_timestamp()) order by created_at,id limit 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.runScanBatch(ctx, id, 500)
	if errors.Is(err, ErrScanBusy) {
		return nil
	}
	return err
}
func scanOutcome(job ScanJob, files []FileRecord) ScanOutcome {
	out := ScanOutcome{JobID: job.ID, State: job.State, Phase: job.Phase, Roots: job.Roots, Scanned: job.Scanned, Upserted: job.Upserted, Skipped: job.Skipped, Missing: job.Missing, Moved: job.Moved, Files: files, HasMore: job.State == "queued" || job.State == "running"}
	if out.Files == nil {
		out.Files = []FileRecord{}
	}
	if job.LastError != "" {
		out.Errors = []string{job.LastError}
	}
	return out
}
func (s *Service) Scan(ctx context.Context, request ScanRequest) (ScanOutcome, error) {
	job, err := s.StartScan(ctx, request)
	if err != nil {
		return ScanOutcome{}, err
	}
	limit := request.Limit
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	out, err := s.runScanBatch(ctx, job.ID, limit)
	if errors.Is(err, ErrScanBusy) {
		return scanOutcome(job, nil), nil
	}
	return out, err
}
