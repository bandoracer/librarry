package api

import (
	"context"
	"encoding/json"
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
