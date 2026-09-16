package library

import (
	"context"
	"encoding/json"
	"errors"
)

type ImportReconciliationIssue struct {
	FileID   string         `json:"fileId"`
	Path     string         `json:"path"`
	Kind     string         `json:"kind"`
	Reason   string         `json:"reason"`
	Evidence map[string]any `json:"evidence"`
}

type ImportRecoveryReport struct {
	Operations []ImportOperation           `json:"operations"`
	Issues     []ImportReconciliationIssue `json:"issues"`
	Unfinished int                         `json:"unfinished"`
	Unresolved int                         `json:"unresolved"`
	Limit      int                         `json:"limit"`
}

// ImportRecovery exposes a bounded recent history and total unresolved counts.
func (s *Service) ImportRecovery(ctx context.Context) (ImportRecoveryReport, error) {
	report := ImportRecoveryReport{Operations: []ImportOperation{}, Issues: []ImportReconciliationIssue{}, Limit: 100}
	if !s.Available() {
		return report, errors.New("import recovery requires database persistence")
	}
	if err := s.store.db.QueryRowContext(ctx, `select (select count(*) from import_operations where state<>'committed'),(select count(*) from import_reconciliation_issues where resolved_at is null)`).Scan(&report.Unfinished, &report.Unresolved); err != nil {
		return report, err
	}
	rows, err := s.store.db.QueryContext(ctx, `select id::text from import_operations order by (state<>'committed') desc,updated_at desc,id limit $1`, report.Limit)
	if err != nil {
		return report, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return report, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, err
	}
	for _, id := range ids {
		op, err := s.store.getOperation(ctx, id)
		if err != nil {
			return report, err
		}
		report.Operations = append(report.Operations, op)
	}
	rows, err = s.store.db.QueryContext(ctx, `select i.file_id::text,f.path,i.kind,i.reason,i.evidence from import_reconciliation_issues i join files f on f.id=i.file_id where i.resolved_at is null order by i.created_at,i.file_id,i.kind limit $1`, report.Limit)
	if err != nil {
		return report, err
	}
	defer rows.Close()
	for rows.Next() {
		var issue ImportReconciliationIssue
		var raw []byte
		if err := rows.Scan(&issue.FileID, &issue.Path, &issue.Kind, &issue.Reason, &raw); err != nil {
			return report, err
		}
		if err := json.Unmarshal(raw, &issue.Evidence); err != nil {
			return report, err
		}
		report.Issues = append(report.Issues, issue)
	}
	return report, rows.Err()
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
