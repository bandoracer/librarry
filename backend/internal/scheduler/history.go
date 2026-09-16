package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrRunNotFound = errors.New("task run not found")
var ErrRunConflict = errors.New("task run changed or is still active; refresh before reviewing it")

type RunPage struct {
	Runs   []RunStatus `json:"runs"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

const effectiveRunState = `case when r.state='running' and not (` + workerLockExists + `) then 'interrupted' else r.state end`

func (r *Registry) RunHistoryPage(ctx context.Context, id, view string, limit, offset int) (RunPage, error) {
	page := RunPage{Runs: []RunStatus{}, Limit: limit, Offset: offset}
	r.mu.Lock()
	_, exists := r.tasks[id]
	r.mu.Unlock()
	if !exists {
		return page, ErrTaskUnknown
	}
	if (view != "all" && view != "unreviewed") || limit < 1 || limit > 100 || offset < 0 {
		return page, errors.New("invalid task history page")
	}
	if r.db == nil {
		return page, nil
	}
	// Materialize effective state once so a changing advisory lock cannot make
	// the page disagree with its count between separate queries.
	var raw []byte
	err := r.db.QueryRowContext(ctx, `with retained as materialized(select r.*,`+effectiveRunState+` as effective_state from worker_task_runs r where r.task_id=$1),filtered as (select * from retained where $2='all' or (reviewed_at is null and effective_state in ('degraded','failed','interrupted'))),page as (select * from filtered order by started_at desc,id desc limit $3 offset $4)
 select (select count(*) from filtered),coalesce((select jsonb_agg(jsonb_build_object('id',id,'taskId',task_id,'trigger',trigger,'state',effective_state,'startedAt',started_at,'heartbeatAt',heartbeat_at,'finishedAt',finished_at,'outcome',outcome,'error',error,'details',details,'reviewedAt',reviewed_at) order by started_at desc,id desc) from page),'[]'::jsonb)`, id, view, limit, offset).Scan(&page.Total, &raw)
	if err != nil {
		return page, err
	}
	if err = json.Unmarshal(raw, &page.Runs); err != nil {
		return page, err
	}
	for i := range page.Runs {
		run := &page.Runs[i]
		if run.FinishedAt != nil && !run.FinishedAt.Before(run.StartedAt) {
			ms := run.FinishedAt.Sub(run.StartedAt).Milliseconds()
			run.DurationMS = &ms
		}
		if run.State == "interrupted" && run.FinishedAt == nil {
			run.Error = "Worker connection ended; completion is unverified"
		}
	}
	return page, nil
}

type RunReview struct {
	Reviewed           bool       `json:"reviewed"`
	ExpectedState      string     `json:"expectedState"`
	ExpectedReviewedAt *time.Time `json:"expectedReviewedAt"`
}

func (r *Registry) ReviewRun(ctx context.Context, taskID, runID string, request RunReview) error {
	r.mu.Lock()
	_, exists := r.tasks[taskID]
	r.mu.Unlock()
	if !exists {
		return ErrTaskUnknown
	}
	if r.db == nil {
		return ErrRunNotFound
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state string
	var reviewed *time.Time
	err = tx.QueryRowContext(ctx, `select `+effectiveRunState+`,r.reviewed_at from worker_task_runs r where r.id::text=$1 and r.task_id=$2 for update of r`, runID, taskID).Scan(&state, &reviewed)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRunNotFound
	}
	if err != nil {
		return err
	}
	if state != request.ExpectedState || (state != "failed" && state != "interrupted" && state != "degraded") {
		return ErrRunConflict
	}
	if (reviewed == nil) != (request.ExpectedReviewedAt == nil) || (reviewed != nil && !reviewed.Equal(*request.ExpectedReviewedAt)) {
		return ErrRunConflict
	}
	_, err = tx.ExecContext(ctx, `update worker_task_runs set reviewed_at=case when $2 then clock_timestamp() else null end,state=case when state='running' then 'interrupted' else state end,error=case when state='running' then 'Worker connection ended; completion is unverified' else error end where id=$1`, runID, request.Reviewed)
	if err != nil {
		return err
	}
	return tx.Commit()
}
