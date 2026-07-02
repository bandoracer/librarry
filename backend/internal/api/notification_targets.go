package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/notify"
)

// notificationTargetPayload is the create/update body for native
// notification targets. Omitted triggers/enabled keep their previous (or
// default) values.
type notificationTargetPayload struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Settings map[string]string `json:"settings"`
	Triggers *notify.Triggers  `json:"triggers"`
	Enabled  *bool             `json:"enabled"`
}

func (h *handler) notificationService(w http.ResponseWriter) (*notify.Service, bool) {
	if h.deps.Notify == nil || !h.deps.Notify.Available() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "notification service is unavailable"})
		return nil, false
	}
	return h.deps.Notify, true
}

func (h *handler) listNotificationTargets(w http.ResponseWriter, r *http.Request) {
	service, ok := h.notificationService(w)
	if !ok {
		return
	}
	targets, err := service.Targets(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	redacted := make([]notify.Target, 0, len(targets))
	for _, target := range targets {
		redacted = append(redacted, notify.RedactTarget(target))
	}
	writeJSON(w, http.StatusOK, map[string]any{"targets": redacted})
}

func (h *handler) createNotificationTarget(w http.ResponseWriter, r *http.Request) {
	service, ok := h.notificationService(w)
	if !ok {
		return
	}
	defer r.Body.Close()
	var payload notificationTargetPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid notification target payload"})
		return
	}
	target := notify.Target{
		Name:     payload.Name,
		Type:     payload.Type,
		Settings: payload.Settings,
		Triggers: notify.DefaultTriggers(),
		Enabled:  true,
	}
	if payload.Triggers != nil {
		target.Triggers = *payload.Triggers
	}
	if payload.Enabled != nil {
		target.Enabled = *payload.Enabled
	}
	created, err := service.CreateTarget(r.Context(), target)
	if err != nil {
		writeJSON(w, notificationTargetErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"target": notify.RedactTarget(created)})
}

func (h *handler) updateNotificationTarget(w http.ResponseWriter, r *http.Request) {
	service, ok := h.notificationService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "notification target id is required"})
		return
	}
	defer r.Body.Close()
	var payload notificationTargetPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid notification target payload"})
		return
	}
	existing, err := service.Target(r.Context(), id)
	if err != nil {
		writeJSON(w, notificationTargetErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	target := existing
	if strings.TrimSpace(payload.Name) != "" {
		target.Name = payload.Name
	}
	if strings.TrimSpace(payload.Type) != "" {
		target.Type = payload.Type
	}
	if payload.Settings != nil {
		target.Settings = payload.Settings
	}
	if payload.Triggers != nil {
		target.Triggers = *payload.Triggers
	}
	if payload.Enabled != nil {
		target.Enabled = *payload.Enabled
	}
	// Blank (or redacted-echo) secrets mean "keep the stored credential".
	target = notify.MergeSecrets(target, existing)
	updated, err := service.UpdateTarget(r.Context(), id, target)
	if err != nil {
		writeJSON(w, notificationTargetErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"target": notify.RedactTarget(updated)})
}

func (h *handler) deleteNotificationTarget(w http.ResponseWriter, r *http.Request) {
	service, ok := h.notificationService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "notification target id is required"})
		return
	}
	if err := service.DeleteTarget(r.Context(), id); err != nil {
		writeJSON(w, notificationTargetErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

func (h *handler) testNotificationTarget(w http.ResponseWriter, r *http.Request) {
	service, ok := h.notificationService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "notification target id is required"})
		return
	}
	target, err := service.Target(r.Context(), id)
	if err != nil {
		writeJSON(w, notificationTargetErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	if err := service.Deliver(r.Context(), target, notify.TestEvent()); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func notificationTargetErrorStatus(err error) int {
	switch {
	case errors.Is(err, notify.ErrTargetNotFound):
		return http.StatusNotFound
	case strings.Contains(err.Error(), "require") || strings.Contains(err.Error(), "not supported"):
		return http.StatusBadRequest
	default:
		return http.StatusBadGateway
	}
}
