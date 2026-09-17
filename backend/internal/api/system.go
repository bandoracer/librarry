package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/scheduler"
)

// systemTasks lists every registered background task with its schedule and
// last-run status.
func (h *handler) systemTasks(w http.ResponseWriter, r *http.Request) {
	tasks := []scheduler.TaskStatus{}
	if h.deps.Scheduler != nil {
		var err error
		tasks, err = h.deps.Scheduler.TasksContext(r.Context())
		if err != nil {
			writeJSON(w, 503, map[string]any{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

// runSystemTask triggers a manual run. The registry's per-task busy flag maps
// to 409 while a run is in flight.
func (h *handler) runSystemTask(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scheduler == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "task scheduler is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "task id is required"})
		return
	}
	err := h.deps.Scheduler.TriggerContext(r.Context(), id)
	switch {
	case errors.Is(err, scheduler.ErrTaskUnknown):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "task not found"})
	case errors.Is(err, scheduler.ErrTaskDisabled):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
	case errors.Is(err, scheduler.ErrTaskUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
	case errors.Is(err, scheduler.ErrTaskBusy):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "task is running"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
	default:
		writeJSON(w, http.StatusAccepted, map[string]any{"started": true})
	}
}

// systemDiskspace reports disk usage for every root folder plus the book
// torrent root, deduplicated by filesystem.
func (h *handler) systemDiskspace(w http.ResponseWriter, r *http.Request) {
	paths := h.monitoredRootPaths(r.Context())
	if config, err := h.effectiveIntegrationConfig(r.Context()); err == nil {
		if root := strings.TrimSpace(config.BookTorrentRoot); root != "" {
			paths = append(paths, library.DiskPath{Path: root, Label: "Book torrents"})
		}
	}
	disks := library.DiskSpaces(paths)
	if disks == nil {
		disks = []library.DiskSpace{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"disks": disks})
}

func (h *handler) systemTaskRuns(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scheduler == nil {
		writeJSON(w, 503, map[string]any{"error": "task scheduler is unavailable"})
		return
	}
	view := r.URL.Query().Get("view")
	if view == "" {
		view = "all"
	}
	limit, offset := 100, 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid history limit"})
			return
		}
		limit = n
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid history offset"})
			return
		}
		offset = n
	}
	if (view != "all" && view != "unreviewed") || limit < 1 || limit > 100 || offset < 0 {
		writeJSON(w, 400, map[string]any{"error": "invalid task history page"})
		return
	}
	page, err := h.deps.Scheduler.RunHistoryPage(r.Context(), strings.TrimSpace(r.PathValue("id")), view, limit, offset)
	if errors.Is(err, scheduler.ErrTaskUnknown) {
		writeJSON(w, 404, map[string]any{"error": "task not found"})
		return
	}
	if err != nil {
		writeJSON(w, 503, map[string]any{"error": "task history is unavailable"})
		return
	}
	writeJSON(w, 200, page)
}

func (h *handler) reviewSystemTaskRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scheduler == nil {
		writeJSON(w, 503, map[string]any{"error": "task scheduler is unavailable"})
		return
	}
	defer r.Body.Close()
	var body struct {
		Reviewed           *bool      `json:"reviewed"`
		ExpectedState      string     `json:"expectedState"`
		ExpectedReviewedAt *time.Time `json:"expectedReviewedAt"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || body.Reviewed == nil || strings.TrimSpace(body.ExpectedState) == "" {
		writeJSON(w, 400, map[string]any{"error": "reviewed and a current run state are required"})
		return
	}
	err := h.deps.Scheduler.ReviewRun(r.Context(), r.PathValue("id"), r.PathValue("runId"), scheduler.RunReview{Reviewed: *body.Reviewed, ExpectedState: body.ExpectedState, ExpectedReviewedAt: body.ExpectedReviewedAt})
	switch {
	case errors.Is(err, scheduler.ErrRunNotFound) || errors.Is(err, scheduler.ErrTaskUnknown):
		writeJSON(w, 404, map[string]any{"error": err.Error()})
	case errors.Is(err, scheduler.ErrRunConflict):
		writeJSON(w, 409, map[string]any{"error": err.Error()})
	case err != nil:
		writeJSON(w, 503, map[string]any{"error": "task review is unavailable"})
	default:
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

func (h *handler) checkIntegration(w http.ResponseWriter, r *http.Request) {
	checker, ok := h.deps.Acquire.(interface {
		CheckIntegration(context.Context, string) (acquisition.IntegrationHealth, error)
	})
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "integration checks are unavailable"})
		return
	}
	result, err := checker.CheckIntegration(r.Context(), r.PathValue("name"))
	switch {
	case errors.Is(err, acquisition.ErrIntegrationUnknown):
		writeJSON(w, 404, map[string]any{"error": err.Error()})
	case errors.Is(err, acquisition.ErrIntegrationChanged):
		writeJSON(w, 409, map[string]any{"error": err.Error()})
	case err != nil:
		writeJSON(w, 503, map[string]any{"error": "integration check is unavailable"})
	default:
		writeJSON(w, 200, result)
	}
}
