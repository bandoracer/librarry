package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/backups"
)

// Native backups API (M6.6): pg_dump-based database backups.

func (h *handler) listBackups(w http.ResponseWriter, r *http.Request) {
	if h.deps.Backups == nil {
		writeJSON(w, http.StatusOK, map[string]any{"backups": []backups.Backup{}})
		return
	}
	rows, err := h.deps.Backups.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if rows == nil {
		rows = []backups.Backup{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": rows})
}

func (h *handler) createBackup(w http.ResponseWriter, r *http.Request) {
	if h.deps.Backups == nil || !h.deps.Backups.Available() {
		writeJSON(w, http.StatusNotImplemented, map[string]any{"error": "backups require a configured database (LIBRARRY_DATABASE_URL)"})
		return
	}
	backup, err := h.deps.Backups.Create(r.Context())
	if err != nil {
		if errors.Is(err, backups.ErrUnavailable) {
			writeJSON(w, http.StatusNotImplemented, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"backup": backup})
}

func (h *handler) deleteBackup(w http.ResponseWriter, r *http.Request) {
	if h.deps.Backups == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "backup not found"})
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	err := h.deps.Backups.Delete(name)
	switch {
	case errors.Is(err, backups.ErrInvalidName):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid backup name"})
	case errors.Is(err, backups.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "backup not found"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "name": name})
	}
}
