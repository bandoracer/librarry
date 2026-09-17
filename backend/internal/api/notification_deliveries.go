package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/bandoracer/librarry/backend/internal/notify"
)

func (h *handler) listNotificationDeliveries(w http.ResponseWriter, r *http.Request) {
	service, ok := h.notificationService(w)
	if !ok {
		return
	}
	limit, offset := 25, 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			writeJSON(w, 400, map[string]any{"error": "limit must be between 1 and 100"})
			return
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		var err error
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			writeJSON(w, 400, map[string]any{"error": "offset must be nonnegative"})
			return
		}
	}
	page, err := service.Deliveries(r.Context(), limit, offset)
	if err != nil {
		writeJSON(w, 503, map[string]any{"error": "notification history is unavailable"})
		return
	}
	writeJSON(w, 200, page)
}
func (h *handler) resolveNotificationDelivery(w http.ResponseWriter, r *http.Request) {
	service, ok := h.notificationService(w)
	if !ok {
		return
	}
	defer r.Body.Close()
	var request notify.DeliveryResolution
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid notification resolution"})
		return
	}
	if request.Action != "retry" && request.Action != "accepted" && request.Action != "cancel" {
		writeJSON(w, 400, map[string]any{"error": "invalid notification resolution action"})
		return
	}
	if err := service.ResolveDelivery(r.Context(), r.PathValue("id"), request); err != nil {
		status, message := 503, "notification resolution is unavailable"
		switch {
		case errors.Is(err, notify.ErrDeliveryNotFound):
			status, message = 404, err.Error()
		case errors.Is(err, notify.ErrDeliveryConflict):
			status, message = 409, err.Error()
		case errors.Is(err, notify.ErrDeliveryConfirmation):
			status, message = 400, err.Error()
		}
		writeJSON(w, status, map[string]any{"error": message})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
