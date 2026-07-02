// Package scheduler owns the background-worker loops. Each worker registers a
// Task (id, name, interval, run function); the registry runs the
// startup-timer/ticker loop, tracks last/next run status for the System Tasks
// view, and serializes manual triggers against scheduled runs with a per-task
// busy flag.
package scheduler

import (
	"context"
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
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Interval    string     `json:"interval"`
	LastRunAt   *time.Time `json:"lastRunAt,omitempty"`
	LastOutcome string     `json:"lastOutcome,omitempty"`
	LastError   string     `json:"lastError,omitempty"`
	NextRunAt   *time.Time `json:"nextRunAt,omitempty"`
	Running     bool       `json:"running"`
}

type taskState struct {
	task        Task
	running     bool
	lastRunAt   *time.Time
	lastOutcome string
	lastError   string
	nextRunAt   *time.Time
}

// Registry wraps every background worker with scheduling and run-status
// bookkeeping.
type Registry struct {
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
	r.tasks[task.ID] = &taskState{task: task}
	r.order = append(r.order, task.ID)
	return nil
}

// Start launches one scheduling loop per registered task. The loops stop when
// ctx is cancelled; wg tracks them for shutdown.
func (r *Registry) Start(ctx context.Context, wg *sync.WaitGroup) {
	r.mu.Lock()
	r.baseCtx = ctx
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
			LastError:   state.lastError,
			Running:     state.running,
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
func (r *Registry) Trigger(id string) error {
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
	state.running = true
	ctx := r.baseCtx
	r.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	go r.run(ctx, state, "manual")
	return nil
}

func (r *Registry) runLoop(ctx context.Context, wg *sync.WaitGroup, state *taskState) {
	defer wg.Done()
	interval := state.task.Interval
	delay := state.task.StartupDelay
	if delay <= 0 {
		delay = 15 * time.Second
	}
	startup := time.NewTimer(delay)
	ticker := time.NewTicker(interval)
	defer startup.Stop()
	defer ticker.Stop()

	nextTick := time.Now().UTC().Add(interval)
	startupAt := time.Now().UTC().Add(delay)
	if startupAt.Before(nextTick) {
		r.setNextRun(state, startupAt)
	} else {
		r.setNextRun(state, nextTick)
	}
	r.logger.Info("task scheduled", "task", state.task.ID, "interval", interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-startup.C:
			if r.tryBegin(state) {
				r.run(ctx, state, "scheduled-startup")
			}
			r.setNextRun(state, nextTick)
		case <-ticker.C:
			nextTick = time.Now().UTC().Add(interval)
			if r.tryBegin(state) {
				r.run(ctx, state, "scheduled")
			}
			r.setNextRun(state, nextTick)
		}
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
	startedAt := time.Now().UTC()
	outcome, err := state.task.Run(ctx, trigger)
	r.mu.Lock()
	state.running = false
	state.lastRunAt = &startedAt
	if err != nil {
		state.lastError = err.Error()
		state.lastOutcome = "failed"
	} else {
		state.lastError = ""
		state.lastOutcome = strings.TrimSpace(outcome)
		if state.lastOutcome == "" {
			state.lastOutcome = "completed"
		}
	}
	r.mu.Unlock()
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
