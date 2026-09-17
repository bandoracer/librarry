package library

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type ImportReconciliationIssue struct {
	FileID   string         `json:"fileId"`
	Path     string         `json:"path"`
	Kind     string         `json:"kind"`
	Reason   string         `json:"reason"`
	Evidence map[string]any `json:"evidence"`
}

type ImportRecoveryObservation struct {
	ObservedAt     time.Time  `json:"observedAt"`
	RecordedAt     time.Time  `json:"recordedAt"`
	LeasePurpose   string     `json:"leasePurpose"`
	LeaseState     string     `json:"leaseState"`
	LeaseExpiresAt *time.Time `json:"leaseExpiresAt,omitempty"`
	VerifiedFiles  int        `json:"verifiedFiles"`
	TotalFiles     int        `json:"totalFiles"`
}

type RecoveryPage struct {
	Total      int    `json:"total"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type ImportRecoveryReport struct {
	CalibreHandoffs   []CalibreHandoff            `json:"calibreHandoffs"`
	CalibreUnfinished int                         `json:"calibreUnfinished"`
	Operations        []ImportOperation           `json:"operations"`
	Issues            []ImportReconciliationIssue `json:"issues"`
	Unfinished        int                         `json:"unfinished"`
	Unresolved        int                         `json:"unresolved"`
	Limit             int                         `json:"limit"`
	OperationsPage    RecoveryPage                `json:"operationsPage"`
	CalibrePage       RecoveryPage                `json:"calibrePage"`
	IssuesPage        RecoveryPage                `json:"issuesPage"`
}

type ImportRecoveryQuery struct {
	Limit            int
	UnfinishedOnly   bool
	OperationsCursor string
	CalibreCursor    string
	IssuesCursor     string
}

var ErrInvalidRecoveryQuery = errors.New("invalid import recovery page: limit must be 1–100 and cursors must match their collection and filter")

const unfinishedOperation = `(state<>'committed' or (source_kind='manual' and cleanup_state<>'cleaned') or replacement_cleanup_state='pending')`

type recoveryCursor struct {
	Collection string    `json:"c"`
	Unfinished bool      `json:"u"`
	CreatedAt  time.Time `json:"t"`
	ID         string    `json:"i"`
	Kind       string    `json:"k,omitempty"`
}

func parseRecoveryCursor(raw, collection string, unfinished bool) (recoveryCursor, error) {
	c := recoveryCursor{Collection: collection, Unfinished: unfinished}
	if raw == "" {
		return c, nil
	}
	if len(raw) > 2048 {
		return c, ErrInvalidRecoveryQuery
	}
	c = recoveryCursor{}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || json.Unmarshal(b, &c) != nil {
		return c, ErrInvalidRecoveryQuery
	}
	var id pgtype.UUID
	if c.Collection != collection || c.Unfinished != unfinished || c.CreatedAt.IsZero() || id.Scan(c.ID) != nil || (collection == "issues" && c.Kind != "wanted" && c.Kind != "download") {
		return c, ErrInvalidRecoveryQuery
	}
	return c, nil
}
func (q ImportRecoveryQuery) Validate() error {
	if q.Limit < 0 || q.Limit > 100 {
		return ErrInvalidRecoveryQuery
	}
	for _, entry := range []struct{ raw, collection string }{{q.OperationsCursor, "operations"}, {q.CalibreCursor, "calibre"}, {q.IssuesCursor, "issues"}} {
		if _, err := parseRecoveryCursor(entry.raw, entry.collection, q.UnfinishedOnly); err != nil {
			return err
		}
	}
	return nil
}

// ImportRecovery preserves the first-page API for in-process consumers.
func (s *Service) ImportRecovery(ctx context.Context) (ImportRecoveryReport, error) {
	return s.ImportRecoveryPage(ctx, ImportRecoveryQuery{})
}

// Pages use immutable creation time and identity, not changing progress times.
// Counts and manifests in one response share a read-only repeatable-read snapshot.
// Across requests this is a live collection, not an export snapshot.
func (s *Service) ImportRecoveryPage(ctx context.Context, q ImportRecoveryQuery) (ImportRecoveryReport, error) {
	report := ImportRecoveryReport{Operations: []ImportOperation{}, Issues: []ImportReconciliationIssue{}, CalibreHandoffs: []CalibreHandoff{}, Limit: q.Limit}
	if err := q.Validate(); err != nil {
		return report, err
	}
	if report.Limit == 0 {
		report.Limit = 100
	}
	if !s.Available() {
		return report, errors.New("import recovery requires database persistence")
	}
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	var nativeTotal, calibreTotal int
	if err = tx.QueryRowContext(ctx, `select
 (select count(*) from import_operations),
 (select count(*) from import_operations where `+unfinishedOperation+`),
 (select count(*) from calibre_handoffs),
 (select count(*) from calibre_handoffs where phase<>'committed'),
 (select count(*) from import_reconciliation_issues where resolved_at is null)`).Scan(&nativeTotal, &report.Unfinished, &calibreTotal, &report.CalibreUnfinished, &report.Unresolved); err != nil {
		return report, err
	}
	report.OperationsPage.Total = nativeTotal
	report.CalibrePage.Total = calibreTotal
	report.IssuesPage.Total = report.Unresolved
	if q.UnfinishedOnly {
		report.OperationsPage.Total = report.Unfinished
		report.CalibrePage.Total = report.CalibreUnfinished
	}
	nativeWhere, calibreWhere := `true`, `true`
	if q.UnfinishedOnly {
		nativeWhere = unfinishedOperation
		calibreWhere = `phase<>'committed'`
	}
	nativeIDs, next, err := recoveryIDs(ctx, tx, "import_operations", nativeWhere, "operations", q.OperationsCursor, q.UnfinishedOnly, report.Limit)
	if err != nil {
		return report, err
	}
	report.OperationsPage.NextCursor = next
	for _, c := range nativeIDs {
		op, err := readOperation(ctx, tx, c.ID)
		if err != nil {
			return report, err
		}
		report.Operations = append(report.Operations, op)
	}
	if err = observeRecovery(ctx, tx, report.Operations); err != nil {
		return report, err
	}
	calibreIDs, next, err := recoveryIDs(ctx, tx, "calibre_handoffs", calibreWhere, "calibre", q.CalibreCursor, q.UnfinishedOnly, report.Limit)
	if err != nil {
		return report, err
	}
	report.CalibrePage.NextCursor = next
	for _, c := range calibreIDs {
		h, err := scanCalibreHandoff(tx.QueryRowContext(ctx, `select `+handoffColumns+` from calibre_handoffs where id=$1`, c.ID))
		if err != nil {
			return report, err
		}
		report.CalibreHandoffs = append(report.CalibreHandoffs, h.CalibreHandoff)
	}
	issueIDs, next, err := recoveryIDs(ctx, tx, "import_reconciliation_issues", `resolved_at is null`, "issues", q.IssuesCursor, q.UnfinishedOnly, report.Limit)
	if err != nil {
		return report, err
	}
	report.IssuesPage.NextCursor = next
	for _, c := range issueIDs {
		var issue ImportReconciliationIssue
		var raw []byte
		err = tx.QueryRowContext(ctx, `select i.file_id::text,f.path,i.kind,i.reason,i.evidence from import_reconciliation_issues i join files f on f.id=i.file_id where i.file_id=$1 and i.kind=$2`, c.ID, c.Kind).Scan(&issue.FileID, &issue.Path, &issue.Kind, &issue.Reason, &raw)
		if err != nil {
			return report, err
		}
		if err = json.Unmarshal(raw, &issue.Evidence); err != nil {
			return report, err
		}
		report.Issues = append(report.Issues, issue)
	}
	return report, tx.Commit()
}

// Only internal constant table/predicate strings are interpolated. Cursor values
// always use parameters. Fetching one extra row proves whether another page exists.
func recoveryIDs(ctx context.Context, tx *sql.Tx, table, predicate, collection, raw string, unfinished bool, limit int) ([]recoveryCursor, string, error) {
	c, err := parseRecoveryCursor(raw, collection, unfinished)
	if err != nil {
		return nil, "", err
	}
	id, kind := "id", "''"
	if collection == "issues" {
		id = "file_id"
		kind = "kind"
	}
	args := []any{limit + 1}
	order := "created_at desc," + id + " desc"
	if collection == "issues" {
		order += ",kind desc"
	}
	if raw != "" {
		args = append(args, c.CreatedAt, c.ID)
		if collection == "issues" {
			predicate += ` and (created_at,file_id,kind) < ($2,$3::uuid,$4)`
			args = append(args, c.Kind)
		} else {
			predicate += ` and (created_at,id) < ($2,$3::uuid)`
		}
	}
	rows, err := tx.QueryContext(ctx, `select created_at,`+id+`::text,`+kind+` from `+table+` where `+predicate+` order by `+order+` limit $1`, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := []recoveryCursor{}
	for rows.Next() {
		next := recoveryCursor{Collection: collection, Unfinished: unfinished}
		if err = rows.Scan(&next.CreatedAt, &next.ID, &next.Kind); err != nil {
			return nil, "", err
		}
		items = append(items, next)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		b, _ := json.Marshal(items[len(items)-1])
		next = base64.RawURLEncoding.EncodeToString(b)
	}
	return items, next, nil
}

func observeRecovery(ctx context.Context, tx *sql.Tx, operations []ImportOperation) error {
	if len(operations) == 0 {
		return nil
	}
	ids := make([]string, len(operations))
	indices := map[string]int{}
	for i, op := range operations {
		ids[i] = op.ID
		indices[op.ID] = i
	}
	rows, err := tx.QueryContext(ctx, `select io.id::text,now(),greatest(io.updated_at,max(f.updated_at)),
 case when io.state<>'committed' then 'transfer' when (io.source_kind='manual' and io.cleanup_state<>'cleaned') or io.replacement_cleanup_state='pending' then 'cleanup' else 'none' end,
 case when not ((io.state<>'committed' or (io.source_kind='manual' and io.cleanup_state<>'cleaned') or io.replacement_cleanup_state='pending')) then 'not_applicable' when io.lease_expires_at>now() then 'held' when io.lease_expires_at is not null then 'expired' else 'none' end,
 case when (io.state<>'committed' or (io.source_kind='manual' and io.cleanup_state<>'cleaned') or io.replacement_cleanup_state='pending') then io.lease_expires_at end,
 count(f.id) filter(where f.state in ('verified','committed')),count(f.id)
 from import_operations io left join import_operation_files f on f.operation_id=io.id
 where io.id=any($1::uuid[]) group by io.id`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var observation ImportRecoveryObservation
		if err = rows.Scan(&id, &observation.ObservedAt, &observation.RecordedAt, &observation.LeasePurpose, &observation.LeaseState, &observation.LeaseExpiresAt, &observation.VerifiedFiles, &observation.TotalFiles); err != nil {
			return err
		}
		operations[indices[id]].Recovery = &observation
	}
	return rows.Err()
}

// RetryImportOperation accepts only a persisted plan ID, never arbitrary paths
// or a new wanted selection. The same lease and current-byte checks apply.
func (s *Service) RetryImportOperation(ctx context.Context, id string) (ImportOutcome, error) {
	if !s.Available() {
		return ImportOutcome{}, errors.New("import recovery requires database persistence")
	}
	op, err := s.store.getOperation(ctx, id)
	if err != nil {
		return ImportOutcome{}, err
	}
	return s.runImportOperation(ctx, op)
}
