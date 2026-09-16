package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bandoracer/librarry/backend/internal/library"
)

type libraryScanService interface {
	StartScan(context.Context, library.ScanRequest) (library.ScanJob, error)
	ListScans(context.Context) ([]library.ScanJob, error)
	CancelScan(context.Context, string) (library.ScanJob, error)
	RetryScan(context.Context, string) (library.ScanJob, error)
}

func (h *handler) listLibraryScans(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Library.(libraryScanService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "persisted scans are unavailable"})
		return
	}
	jobs, err := service.ListScans(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if jobs == nil {
		jobs = []library.ScanJob{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"scans": jobs, "limit": 100})
}
func (h *handler) controlLibraryScan(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Library.(libraryScanService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "persisted scans are unavailable"})
		return
	}
	defer r.Body.Close()
	var request struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid scan action"})
		return
	}
	var job library.ScanJob
	var err error
	switch request.Action {
	case "cancel":
		job, err = service.CancelScan(r.Context(), r.PathValue("id"))
	case "retry":
		job, err = service.RetryScan(r.Context(), r.PathValue("id"))
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "scan action must be cancel or retry"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *handler) startLibraryScan(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Library.(libraryScanService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "persisted scans are unavailable"})
		return
	}
	defer r.Body.Close()
	var request library.ScanRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid scan request"})
		return
	}
	job, err := service.StartScan(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

type libraryScanMoveService interface {
	ScanMoveHistory(context.Context, string, string) (library.ScanMoveHistory, error)
}

func (h *handler) libraryScanMoves(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Library.(libraryScanMoveService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "scan move history is unavailable"})
		return
	}
	history, err := service.ScanMoveHistory(r.Context(), r.PathValue("id"), r.URL.Query().Get("cursor"))
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, library.ErrScanMoveCursor) {
			status = http.StatusBadRequest
		} else if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	if history.Moves == nil {
		history.Moves = []library.ScanMoveRecord{}
	}
	writeJSON(w, http.StatusOK, history)
}
