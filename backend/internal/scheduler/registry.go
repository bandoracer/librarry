// Package scheduler owns the background-worker loops. Each worker registers a
// Task (id, name, interval, run function); the registry runs the
// due-time timer loop, tracks last/next run status for the System Tasks
// view, and serializes manual triggers against scheduled runs with a per-task
// busy flag and optional shared Postgres ownership.
package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
)

var (
	// ErrTaskBusy is returned when a run is requested while the task is
	// already running (HTTP handlers map it to 409).
	ErrTaskBusy = errors.New("task is running")
	// ErrTaskUnknown is returned for unregistered task ids.
	ErrTaskUnknown = errors.New("task not found")
)

// RunFunc executes one pass of a background worker and returns a one-line
// outcome summary. The trigger is "scheduled-startup", "scheduled", or
// "manual".
type RunFunc func(ctx context.Context, trigger string) (string, error)

// Task describes a background worker managed by the registry.
type Task struct {
	ID           string
	Name         string
	Interval     time.Duration
	StartupDelay time.Duration
	Run          RunFunc
}

// TaskStatus is the API-facing snapshot of a registered task.
type TaskStatus struct {
	Details            RunDetails `json:"details"`
	DurationMS         *int64     `json:"durationMs,omitempty"`
	LastSuccessAt      *time.Time `json:"lastSuccessAt,omitempty"`
	LastSuccessRunID   string     `json:"lastSuccessRunId,omitempty"`
	UnreviewedFailures int        `json:"unreviewedFailures"`
	RunState           string     `json:"runState,omitempty"`
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Interval           string     `json:"interval"`
	LastRunAt          *time.Time `json:"lastRunAt,omitempty"`
	LastOutcome        string     `json:"lastOutcome,omitempty"`
	LastError          string     `json:"lastError,omitempty"`
	NextRunAt          *time.Time `json:"nextRunAt,omitempty"`
	Running            bool       `json:"running"`
}

type taskState struct {
	wake          chan struct{}
	task          Task
	running       bool
	lastRunAt     *time.Time
	lastOutcome   string
	details       RunDetails
	durationMS    *int64
	lastSuccessAt *time.Time
	lastError     string
	nextRunAt     *time.Time
}

// Registry wraps every background worker with scheduling and run-status
// bookkeeping.
type Registry struct {
	db     *sql.DB
	wg     *sync.WaitGroup
	logger *slog.Logger

	mu      sync.Mutex
	tasks   map[string]*taskState
	order   []string
	baseCtx context.Context
	started bool
}

func NewRegistry(logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		logger: logger,
		tasks:  map[string]*taskState{},
	}
}

// Register adds a task to the registry. It must be called before Start.
func (r *Registry) Register(task Task) error {
	task.ID = strings.TrimSpace(task.ID)
	task.Name = strings.TrimSpace(task.Name)
	if task.ID == "" {
		return errors.New("task id is required")
	}
	if task.Name == "" {
		task.Name = task.ID
	}
	if task.Interval <= 0 {
		return errors.New("task interval must be positive")
	}
	if task.Run == nil {
		return errors.New("task run function is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return errors.New("registry has already started")
	}
	if _, exists := r.tasks[task.ID]; exists {
		return errors.New("task " + task.ID + " is already registered")
	}
	r.tasks[task.ID] = &taskState{task: task, wake: make(chan struct{}, 1)}
	r.order = append(r.order, task.ID)
	return nil
}

// Start launches one scheduling loop per registered task. The loops stop when
// ctx is cancelled; wg tracks them for shutdown.
func (r *Registry) Start(ctx context.Context, wg *sync.WaitGroup) {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return
	}
	r.baseCtx = ctx
	r.wg = wg
	r.started = true
	states := make([]*taskState, 0, len(r.order))
	for _, id := range r.order {
		states = append(states, r.tasks[id])
	}
	r.mu.Unlock()
	for _, state := range states {
		wg.Add(1)
		go r.runLoop(ctx, wg, state)
	}
}

// Tasks returns status snapshots in registration order.
func (r *Registry) Tasks() []TaskStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	statuses := make([]TaskStatus, 0, len(r.order))
	for _, id := range r.order {
		state := r.tasks[id]
		status := TaskStatus{
			ID:          state.task.ID,
			Name:        state.task.Name,
			Interval:    FormatInterval(state.task.Interval),
			LastOutcome: state.lastOutcome,
			Details:     normalizeDetails(state.details), DurationMS: state.durationMS, LastSuccessAt: state.lastSuccessAt,
			LastError: state.lastError,
			Running:   state.running,
		}
		if state.lastRunAt != nil {
			at := *state.lastRunAt
			status.LastRunAt = &at
		}
		if state.nextRunAt != nil {
			at := *state.nextRunAt
			status.NextRunAt = &at
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// Trigger starts a manual run of the task in the background. It returns
// ErrTaskUnknown for unregistered ids and ErrTaskBusy while a run is in
// flight.
func (r *Registry) Trigger(id string) error { return r.TriggerContext(context.Background(), id) }

func (r *Registry) TriggerContext(requestCtx context.Context, id string) error {
	r.mu.Lock()
	state, ok := r.tasks[strings.TrimSpace(id)]
	if !ok {
		r.mu.Unlock()
		return ErrTaskUnknown
	}
	if state.running {
		r.mu.Unlock()
		return ErrTaskBusy
	}
	ctx := r.baseCtx
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		r.mu.Unlock()
		return errors.New("task scheduler is shutting down")
	}
	state.running = true
	wg := r.wg
	if wg != nil {
		wg.Add(1)
	}
	r.mu.Unlock()
	claimCtx, cancel := context.WithTimeout(requestCtx, 5*time.Second)
	claim, err := r.claim(claimCtx, state.task, "manual")
	cancel()
	if err != nil {
		r.mu.Lock()
		state.running = false
		r.mu.Unlock()
		if wg != nil {
			wg.Done()
		}
		return err
	}
	wakeTask(state)
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		r.runClaimed(ctx, state, "manual", claim)
	}()
	return nil
}

func (r *Registry) runLoop(ctx context.Context, wg *sync.WaitGroup, state *taskState) {
	defer wg.Done()
	delay := state.task.StartupDelay
	if delay <= 0 {
		delay = 15 * time.Second
	}
	if state.task.Interval < delay {
		delay = state.task.Interval
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	r.setNextRun(state, time.Now().UTC().Add(delay))
	first := true
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			trigger := "scheduled"
			if first {
				trigger = "scheduled-startup"
				first = false
			}
			if r.tryBegin(state) {
				r.run(ctx, state, trigger)
			}
		case <-state.wake:
		}
		if ctx.Err() != nil {
			return
		}
		next := r.nextTaskRun(ctx, state)
		r.setNextRun(state, next)
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		wait := time.Until(next)
		if wait <= 0 {
			wait = time.Second
		}
		timer.Reset(wait)
	}
}
func (r *Registry) nextTaskRun(ctx context.Context, state *taskState) time.Time {
	if r.db != nil {
		queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		var next time.Time
		if err := r.db.QueryRowContext(queryCtx, `select next_run_at from worker_tasks where task_id=$1`, state.task.ID).Scan(&next); err == nil {
			return next
		}
		return time.Now().UTC().Add(5 * time.Second)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if state.lastRunAt != nil {
		return state.lastRunAt.Add(state.task.Interval)
	}
	return time.Now().UTC().Add(state.task.Interval)
}
func wakeTask(state *taskState) {
	select {
	case state.wake <- struct{}{}:
	default:
	}
}

// tryBegin claims the busy flag; scheduled ticks skip silently while a manual
// run is in flight.
func (r *Registry) tryBegin(state *taskState) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if state.running {
		return false
	}
	state.running = true
	return true
}

// run executes the task body; the caller must already hold the busy flag.
func (r *Registry) run(ctx context.Context, state *taskState, trigger string) {
	claimCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	claim, err := r.claim(claimCtx, state.task, trigger)
	cancel()
	if err != nil {
		r.mu.Lock()
		state.running = false
		if !errors.Is(err, ErrTaskBusy) && !errors.Is(err, errNotDue) {
			state.lastError = err.Error()
		}
		r.mu.Unlock()
		return
	}
	r.runClaimed(ctx, state, trigger, claim)
}
func invokeTask(ctx context.Context, task Task, trigger string) (outcome string, err error) {
	defer func() {
		if recover() != nil {
			outcome = ""
			err = errors.New("task panicked; completion is unverified")
		}
	}()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return task.Run(ctx, trigger)
}
func (r *Registry) runClaimed(ctx context.Context, state *taskState, trigger string, claim *taskClaim) {
	startedAt := time.Now().UTC()
	runCtx, cancel := context.WithCancel(ctx)
	heartbeatDone := make(chan struct{})
	if claim != nil {
		go claim.heartbeat(runCtx, cancel, heartbeatDone)
	} else {
		close(heartbeatDone)
	}
	collector := &detailsCollector{}
	runCtx = context.WithValue(runCtx, detailsKey{}, collector)
	outcome, err := invokeTask(runCtx, state.task, trigger)
	details := collector.snapshot()
	// A function returning success after cancellation cannot certify completion.
	if err == nil && runCtx.Err() != nil {
		err = runCtx.Err()
	}
	cancel()
	<-heartbeatDone
	if strings.TrimSpace(outcome) == "" && err == nil {
		outcome = "completed"
	}
	if claim != nil {
		if e := claim.finish(outcome, err, details); e != nil {
			err = e
		}
	}
	r.mu.Lock()
	state.running = false
	state.lastRunAt = &startedAt
	state.details = details
	duration := max(int64(0), time.Since(startedAt).Milliseconds())
	state.durationMS = &duration
	if err == nil && details.Errors == 0 {
		finished := time.Now().UTC()
		state.lastSuccessAt = &finished
	}
	if err != nil {
		state.lastError = err.Error()
		state.lastOutcome = "failed"
	} else if details.Errors > 0 {
		state.lastError = "Worker completed with reported errors"
		state.lastOutcome = "degraded"
	} else {
		state.lastError = ""
		state.lastOutcome = strings.TrimSpace(outcome)
	}
	r.mu.Unlock()
	if trigger == "manual" {
		wakeTask(state)
	}
	if err != nil && ctx.Err() == nil {
		r.logger.Warn("task run failed", "task", state.task.ID, "trigger", trigger, "error", err)
	}
}

func (r *Registry) setNextRun(state *taskState, at time.Time) {
	r.mu.Lock()
	state.nextRunAt = &at
	r.mu.Unlock()
}

// FormatInterval renders durations the way the arr UIs do: "30m", "6h",
// "1h30m", "45s".
func FormatInterval(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}
