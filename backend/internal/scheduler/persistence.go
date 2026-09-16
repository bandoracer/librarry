package scheduler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var errNotDue = errors.New("task is not due")

const workerLockKey = `hashtextextended('librarry-worker:'||$1,0)`

// The backend PID plus the actual lock identifies a running session. A stored
// heartbeat alone must not invent liveness after process death.
const workerLockExists = `exists(select 1 from worker_tasks owner where owner.task_id=r.task_id and owner.run_id=r.id) and exists(select 1 from pg_locks l where l.locktype='advisory' and l.granted and l.pid=r.backend_pid and l.database=(select oid from pg_database where datname=current_database()) and l.classid::bigint=((hashtextextended('librarry-worker:'||r.task_id,0)>>32)&4294967295) and l.objid::bigint=(hashtextextended('librarry-worker:'||r.task_id,0)&4294967295) and l.objsubid=1)`

type RunStatus struct {
	Details     RunDetails `json:"details"`
	ReviewedAt  *time.Time `json:"reviewedAt,omitempty"`
	DurationMS  *int64     `json:"durationMs,omitempty"`
	ID          string     `json:"id"`
	TaskID      string     `json:"taskId"`
	Trigger     string     `json:"trigger"`
	State       string     `json:"state"`
	StartedAt   time.Time  `json:"startedAt"`
	HeartbeatAt time.Time  `json:"heartbeatAt"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	Outcome     string     `json:"outcome,omitempty"`
	Error       string     `json:"error,omitempty"`
}
type taskClaim struct {
	conn          *sql.Conn
	taskID, runID string
}

func (r *Registry) WithDatabase(db *sql.DB) *Registry { r.db = db; return r }
func releaseWorkerConnection(conn *sql.Conn, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var released bool
	if err := conn.QueryRowContext(ctx, `select pg_advisory_unlock(`+workerLockKey+`)`, id).Scan(&released); err != nil || !released {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}
	_ = conn.Close()
}
func (r *Registry) claim(ctx context.Context, task Task, trigger string) (*taskClaim, error) {
	if err := task.executionError(); err != nil {
		return nil, err
	}
	if r.db == nil {
		return nil, nil
	}
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, errors.New("task coordination database is unavailable")
	}
	var locked bool
	if err = conn.QueryRowContext(ctx, `select pg_try_advisory_lock(`+workerLockKey+`)`, task.ID).Scan(&locked); err != nil {
		conn.Close()
		return nil, errors.New("could not acquire shared task ownership")
	}
	if !locked {
		conn.Close()
		return nil, ErrTaskBusy
	}
	success := false
	defer func() {
		if !success {
			releaseWorkerConnection(conn, task.ID)
		}
	}()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `insert into worker_tasks(task_id) values($1) on conflict do nothing`, task.ID); err != nil {
		return nil, err
	}
	var due bool
	var previous string
	if err = tx.QueryRowContext(ctx, `select next_run_at<=now(),coalesce(run_id::text,'') from worker_tasks where task_id=$1 for update`, task.ID).Scan(&due, &previous); err != nil {
		return nil, err
	}
	// Owning this lock proves any former owner's DB session is gone. Never use
	// heartbeat expiry to steal a lock from a slow but still connected worker.
	if previous != "" {
		if _, err = tx.ExecContext(ctx, `update worker_task_runs set state='interrupted',error='Previous worker connection ended before completion was recorded' where id=$1 and state='running'`, previous); err != nil {
			return nil, err
		}
	}
	if trigger != "manual" && !due {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, errNotDue
	}
	var id string
	if err = tx.QueryRowContext(ctx, `insert into worker_task_runs(task_id,trigger,backend_pid,state) values($1,$2,pg_backend_pid(),'running') returning id::text`, task.ID, trigger).Scan(&id); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `update worker_tasks set run_id=$2,next_run_at=now()+($3 * interval '1 millisecond') where task_id=$1`, task.ID, id, task.Interval.Milliseconds()); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	success = true
	return &taskClaim{conn: conn, taskID: task.ID, runID: id}, nil
}
func (c *taskClaim) heartbeat(ctx context.Context, cancel context.CancelFunc, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			beatCtx, stop := context.WithTimeout(ctx, 5*time.Second)
			result, err := c.conn.ExecContext(beatCtx, `update worker_task_runs set heartbeat_at=now() where id=$1 and state='running' and exists(select 1 from worker_tasks where task_id=$2 and run_id=$1)`, c.runID, c.taskID)
			stop()
			if err != nil {
				cancel()
				return
			}
			n, e := result.RowsAffected()
			if e != nil || n != 1 {
				cancel()
				return
			}
		}
	}
}
func (c *taskClaim) finish(outcome string, runErr error, reports ...RunDetails) error {
	defer releaseWorkerConnection(c.conn, c.taskID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state, message := "completed", ""
	if runErr != nil {
		state = "failed"
		message = runErr.Error()
		if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
			state = "interrupted"
		}
	}
	details := RunDetails{}
	if len(reports) > 0 {
		details = normalizeDetails(reports[0])
	}
	if runErr == nil && details.Errors > 0 {
		state = "degraded"
		message = fmt.Sprintf("Worker completed with %d reported errors", details.Errors)
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	tx, err := c.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `update worker_task_runs set state=$3,outcome=$4,error=$5,details=$6,finished_at=now(),heartbeat_at=now() where id=$1 and state='running' and exists(select 1 from worker_tasks where task_id=$2 and run_id=$1)`, c.runID, c.taskID, state, outcome, message, string(raw))
	if err != nil {
		return errors.New("task finished but its durable outcome could not be saved")
	}
	if n, e := result.RowsAffected(); e != nil || n != 1 {
		return errors.New("task ownership changed before completion")
	}
	if state == "completed" {
		if _, err = tx.ExecContext(ctx, `update worker_tasks set last_success_at=now(),last_success_run_id=$2 where task_id=$1 and run_id=$2`, c.taskID, c.runID); err != nil {
			return err
		}
	}
	// Keep 100 successful runs, prioritizing current/last-success identities even
	// with clock-skewed timestamps. Unreviewed failures are never routine data.
	if _, err = tx.ExecContext(ctx, `delete from worker_task_runs r where r.task_id=$1 and r.id in (select s.id from worker_task_runs s join worker_tasks t on t.task_id=s.task_id where s.task_id=$1 and s.state='completed' order by (s.id=t.run_id or s.id=t.last_success_run_id) desc,s.started_at desc,s.id desc offset 100)`, c.taskID); err != nil {
		return err
	}
	// A review makes old failures eligible after 90 days. Never delete the
	// current run, and bound each cleanup pass even after a long outage.
	if _, err = tx.ExecContext(ctx, `delete from worker_task_runs where id in (select r.id from worker_task_runs r join worker_tasks t on t.task_id=r.task_id where r.task_id=$1 and r.id<>t.run_id and r.state in ('degraded','failed','interrupted') and r.reviewed_at<now()-interval '90 days' order by r.reviewed_at,r.id limit 500 for update of r skip locked)`, c.taskID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Registry) TasksContext(ctx context.Context) ([]TaskStatus, error) {
	statuses := r.Tasks()
	if r.db == nil {
		return statuses, nil
	}
	for i := range statuses {
		s := &statuses[i]
		var started, finished sql.NullTime
		var next time.Time
		var state, outcome, message string
		var live bool
		var details []byte
		err := r.db.QueryRowContext(ctx, `select t.next_run_at,r.started_at,r.finished_at,coalesce(r.state,''),coalesce(r.outcome,''),coalesce(r.error,''),`+workerLockExists+`,t.last_success_at,coalesce(t.last_success_run_id::text,''),coalesce(r.details,'{}'),(select count(*) from worker_task_runs r where r.task_id=t.task_id and r.reviewed_at is null and (r.state in ('degraded','failed','interrupted') or (r.state='running' and not (`+workerLockExists+`)))) from worker_tasks t left join worker_task_runs r on r.id=t.run_id where t.task_id=$1`, s.ID).Scan(&next, &started, &finished, &state, &outcome, &message, &live, &s.LastSuccessAt, &s.LastSuccessRunID, &details, &s.UnreviewedFailures)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, errors.New("shared task status is unavailable")
		}
		if err = json.Unmarshal(details, &s.Details); err != nil {
			return nil, errors.New("shared task details are unavailable")
		}
		s.DurationMS = nil
		if finished.Valid && started.Valid && !finished.Time.Before(started.Time) {
			ms := max(int64(0), finished.Time.Sub(started.Time).Milliseconds())
			s.DurationMS = &ms
		}
		s.NextRunAt = nil
		if s.Enabled && s.Available {
			s.NextRunAt = &next
		}
		s.LastFinishedAt = nil
		if finished.Valid {
			s.LastFinishedAt = &finished.Time
		}
		s.Running = state == "running" && live
		s.LastError = message
		s.LastOutcome = outcome
		s.RunState = state
		if started.Valid {
			s.LastRunAt = &started.Time
		}
		if state == "running" && !live {
			s.RunState = "interrupted"
			s.LastError = "Worker connection ended; completion is unverified"
			s.LastOutcome = "interrupted"
		}
		if state == "failed" || state == "interrupted" || state == "degraded" {
			s.LastOutcome = state
		}
	}
	return statuses, nil
}
func (r *Registry) RunHistory(ctx context.Context, id string) ([]RunStatus, error) {
	page, err := r.RunHistoryPage(ctx, id, "all", 100, 0)
	return page.Runs, err
}
