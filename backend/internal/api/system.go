package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/scheduler"
)

// systemTasks lists every registered background task with its schedule and
// last-run status.
func (h *handler) systemTasks(w http.ResponseWriter, r *http.Request) {
	tasks := []scheduler.TaskStatus{}
	if h.deps.Scheduler != nil {
		tasks = h.deps.Scheduler.Tasks()
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
	err := h.deps.Scheduler.Trigger(id)
	switch {
	case errors.Is(err, scheduler.ErrTaskUnknown):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "task not found"})
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
