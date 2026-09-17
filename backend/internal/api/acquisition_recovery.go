package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

type acquisitionRecoveryService interface {
	AcquisitionRecovery(context.Context) ([]acquisition.AcquisitionIntent, error)
	ReconcileAcquisition(context.Context, string, string) (acquisition.DownloadStatus, error)
	ReleaseAcquisition(context.Context, string) error
}

func (h *handler) acquisitionRecovery(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Acquire.(acquisitionRecoveryService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition recovery is unavailable"})
		return
	}
	intents, err := service.AcquisitionRecovery(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if intents == nil {
		intents = []acquisition.AcquisitionIntent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"intents": intents, "limit": 200})
}
func (h *handler) resolveAcquisition(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Acquire.(acquisitionRecoveryService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition recovery is unavailable"})
		return
	}
	var request struct {
		Action     string `json:"action"`
		DownloadID string `json:"downloadId"`
		Confirmed  bool   `json:"confirmed"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid recovery request"})
		return
	}
	var status acquisition.DownloadStatus
	var err error
	switch request.Action {
	case "check":
		status, err = service.ReconcileAcquisition(r.Context(), r.PathValue("id"), "")
	case "attach":
		if !request.Confirmed || strings.TrimSpace(request.DownloadID) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "confirm the exact client download ID before attaching it"})
			return
		}
		status, err = service.ReconcileAcquisition(r.Context(), r.PathValue("id"), strings.TrimSpace(request.DownloadID))
	case "release":
		if !request.Confirmed {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "inspect the client and confirm a new attempt is safe before releasing this acquisition"})
			return
		}
		err = service.ReleaseAcquisition(r.Context(), r.PathValue("id"))
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "action must be check, attach, or release"})
		return
	}
	if err != nil {
		code := http.StatusBadGateway
		if errors.Is(err, acquisition.ErrAcquisitionBusy) || errors.Is(err, acquisition.ErrAcquisitionUncertain) || errors.Is(err, acquisition.ErrAcquisitionActive) {
			code = http.StatusConflict
		}
		if errors.Is(err, sql.ErrNoRows) {
			code = http.StatusNotFound
		}
		writeJSON(w, code, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"resolved": true, "action": request.Action, "download": status})
}
