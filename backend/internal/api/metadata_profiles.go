package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

// metadataProfileService is the optional wanted-service surface for native
// metadata profiles (Readarr-style author add-filter sets).
type metadataProfileService interface {
	ListMetadataProfiles(ctx context.Context) ([]wanted.MetadataProfile, error)
	CreateMetadataProfile(ctx context.Context, profile wanted.MetadataProfile) (wanted.MetadataProfile, error)
	UpdateMetadataProfile(ctx context.Context, id string, profile wanted.MetadataProfile) (wanted.MetadataProfile, error)
	DeleteMetadataProfile(ctx context.Context, id string) error
}

func (h *handler) metadataProfileService(w http.ResponseWriter) (metadataProfileService, bool) {
	service, ok := h.deps.Wanted.(metadataProfileService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "metadata profile service is unavailable"})
		return nil, false
	}
	return service, true
}

func (h *handler) metadataProfiles(w http.ResponseWriter, r *http.Request) {
	service, ok := h.metadataProfileService(w)
	if !ok {
		return
	}
	profiles, err := service.ListMetadataProfiles(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if profiles == nil {
		profiles = []wanted.MetadataProfile{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
}

func (h *handler) createMetadataProfile(w http.ResponseWriter, r *http.Request) {
	service, ok := h.metadataProfileService(w)
	if !ok {
		return
	}
	defer r.Body.Close()
	var profile wanted.MetadataProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid metadata profile payload"})
		return
	}
	if strings.TrimSpace(profile.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "metadata profile name is required"})
		return
	}
	created, err := service.CreateMetadataProfile(r.Context(), profile)
	if err != nil {
		writeJSON(w, metadataProfileErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": created})
}

func (h *handler) updateMetadataProfile(w http.ResponseWriter, r *http.Request) {
	service, ok := h.metadataProfileService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "metadata profile id is required"})
		return
	}
	defer r.Body.Close()
	var profile wanted.MetadataProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid metadata profile payload"})
		return
	}
	if strings.TrimSpace(profile.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "metadata profile name is required"})
		return
	}
	updated, err := service.UpdateMetadataProfile(r.Context(), id, profile)
	if err != nil {
		writeJSON(w, metadataProfileErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": updated})
}

func (h *handler) deleteMetadataProfile(w http.ResponseWriter, r *http.Request) {
	service, ok := h.metadataProfileService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "metadata profile id is required"})
		return
	}
	if err := service.DeleteMetadataProfile(r.Context(), id); err != nil {
		writeJSON(w, metadataProfileErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func metadataProfileErrorStatus(err error) int {
	switch {
	case errors.Is(err, wanted.ErrMetadataProfileInUse):
		return http.StatusConflict
	case errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound
	case strings.Contains(err.Error(), "is required"), strings.Contains(err.Error(), "already in use"):
		return http.StatusBadRequest
	default:
		return http.StatusBadGateway
	}
}
