package scheduler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"
)

var errNotDue = errors.New("task is not due")

const workerLockKey = `hashtextextended('librarry-worker:'||$1,0)`

// The backend PID plus the actual lock identifies a running session. A stored
// heartbeat alone must not invent liveness after process death.
const workerLockExists = `exists(select 1 from pg_locks l where l.locktype='advisory' and l.granted and l.pid=r.backend_pid and l.database=(select oid from pg_database where datname=current_database()) and l.classid::bigint=((hashtextextended('librarry-worker:'||r.task_id,0)>>32)&4294967295) and l.objid::bigint=(hashtextextended('librarry-worker:'||r.task_id,0)&4294967295) and l.objsubid=1)`

type RunStatus struct {
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
		if _, err = tx.ExecContext(ctx, `update worker_task_runs set state='interrupted',finished_at=now(),error='Previous worker connection ended before completion was recorded' where id=$1 and state='running'`, previous); err != nil {
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
func (c *taskClaim) finish(outcome string, runErr error) error {
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
	result, err := c.conn.ExecContext(ctx, `update worker_task_runs set state=$3,outcome=$4,error=$5,finished_at=now(),heartbeat_at=now() where id=$1 and state='running' and exists(select 1 from worker_tasks where task_id=$2 and run_id=$1)`, c.runID, c.taskID, state, outcome, message)
	if err != nil {
		return errors.New("task finished but its durable outcome could not be saved")
	}
	if n, e := result.RowsAffected(); e != nil || n != 1 {
		return errors.New("task ownership changed before completion")
	}
	// Keep a bounded diagnostic history per task. The current run is retained.
	_, err = c.conn.ExecContext(ctx, `delete from worker_task_runs where task_id=$1 and id in (select id from worker_task_runs where task_id=$1 and id<>$2 and state<>'running' order by started_at desc,id desc offset 99)`, c.taskID, c.runID)
	return err
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
		err := r.db.QueryRowContext(ctx, `select t.next_run_at,r.started_at,r.finished_at,coalesce(r.state,''),coalesce(r.outcome,''),coalesce(r.error,''),`+workerLockExists+` from worker_tasks t left join worker_task_runs r on r.id=t.run_id where t.task_id=$1`, s.ID).Scan(&next, &started, &finished, &state, &outcome, &message, &live)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, errors.New("shared task status is unavailable")
		}
		s.NextRunAt = &next
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
		if state == "failed" || state == "interrupted" {
			s.LastOutcome = state
		}
	}
	return statuses, nil
}
func (r *Registry) RunHistory(ctx context.Context, id string) ([]RunStatus, error) {
	r.mu.Lock()
	_, exists := r.tasks[id]
	r.mu.Unlock()
	if !exists {
		return nil, ErrTaskUnknown
	}
	items := []RunStatus{}
	if r.db == nil {
		return items, nil
	}
	rows, err := r.db.QueryContext(ctx, `select r.id::text,r.task_id,r.trigger,case when r.state='running' and not (`+workerLockExists+`) then 'interrupted' else r.state end,r.started_at,r.heartbeat_at,r.finished_at,r.outcome,r.error from worker_task_runs r where r.task_id=$1 order by r.started_at desc,r.id desc limit 100`, id)
	if err != nil {
		return nil, fmt.Errorf("task run history is unavailable")
	}
	defer rows.Close()
	for rows.Next() {
		var item RunStatus
		if err = rows.Scan(&item.ID, &item.TaskID, &item.Trigger, &item.State, &item.StartedAt, &item.HeartbeatAt, &item.FinishedAt, &item.Outcome, &item.Error); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
