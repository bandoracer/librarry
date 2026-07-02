package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/importlists"
)

// Native import lists API (M6.3).

func (h *handler) importListService(w http.ResponseWriter) (*importlists.Service, bool) {
	if h.deps.ImportLists == nil || !h.deps.ImportLists.Available() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "import lists require database persistence"})
		return nil, false
	}
	return h.deps.ImportLists, true
}

func (h *handler) listImportLists(w http.ResponseWriter, r *http.Request) {
	service, ok := h.importListService(w)
	if !ok {
		return
	}
	lists, err := service.Lists(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if lists == nil {
		lists = []importlists.List{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"lists": lists})
}

type importListPayload struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Enabled        *bool             `json:"enabled"`
	Settings       map[string]string `json:"settings"`
	Monitor        string            `json:"monitor"`
	QualityProfile string            `json:"qualityProfile"`
	RootFolderID   string            `json:"rootFolderId"`
	SearchOnAdd    *bool             `json:"searchOnAdd"`
}

func (payload importListPayload) toList() importlists.List {
	list := importlists.List{
		Name:           payload.Name,
		Type:           payload.Type,
		Enabled:        true,
		Settings:       payload.Settings,
		Monitor:        payload.Monitor,
		QualityProfile: payload.QualityProfile,
		RootFolderID:   payload.RootFolderID,
	}
	if payload.Enabled != nil {
		list.Enabled = *payload.Enabled
	}
	if payload.SearchOnAdd != nil {
		list.SearchOnAdd = *payload.SearchOnAdd
	}
	return list
}

func (h *handler) createImportList(w http.ResponseWriter, r *http.Request) {
	service, ok := h.importListService(w)
	if !ok {
		return
	}
	defer r.Body.Close()
	var payload importListPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid import list payload"})
		return
	}
	created, err := service.Store().CreateList(r.Context(), payload.toList())
	if err != nil {
		writeJSON(w, importListErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"list": created})
}

func (h *handler) updateImportList(w http.ResponseWriter, r *http.Request) {
	service, ok := h.importListService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "import list id is required"})
		return
	}
	defer r.Body.Close()
	var payload importListPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid import list payload"})
		return
	}
	updated, err := service.Store().UpdateList(r.Context(), id, payload.toList())
	if err != nil {
		writeJSON(w, importListErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"list": updated})
}

func (h *handler) deleteImportList(w http.ResponseWriter, r *http.Request) {
	service, ok := h.importListService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "import list id is required"})
		return
	}
	if err := service.Store().DeleteList(r.Context(), id); err != nil {
		writeJSON(w, importListErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

// syncImportList runs one list synchronously and returns the outcome (status
// + message + per-entry items).
func (h *handler) syncImportList(w http.ResponseWriter, r *http.Request) {
	service, ok := h.importListService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "import list id is required"})
		return
	}
	if _, err := service.Store().GetList(r.Context(), id); err != nil {
		writeJSON(w, importListErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	outcome, err := service.Sync(r.Context(), []string{id}, "manual")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "outcome": outcome})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) listImportListExclusions(w http.ResponseWriter, r *http.Request) {
	service, ok := h.importListService(w)
	if !ok {
		return
	}
	exclusions, err := service.Exclusions(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if exclusions == nil {
		exclusions = []importlists.Exclusion{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"exclusions": exclusions})
}

func (h *handler) createImportListExclusion(w http.ResponseWriter, r *http.Request) {
	service, ok := h.importListService(w)
	if !ok {
		return
	}
	defer r.Body.Close()
	var payload importlists.Exclusion
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid import list exclusion payload"})
		return
	}
	created, err := service.Store().CreateExclusion(r.Context(), payload)
	if err != nil {
		writeJSON(w, importListErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"exclusion": created})
}

func (h *handler) deleteImportListExclusion(w http.ResponseWriter, r *http.Request) {
	service, ok := h.importListService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "exclusion id is required"})
		return
	}
	if err := service.Store().DeleteExclusion(r.Context(), id); err != nil {
		writeJSON(w, importListErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

func importListErrorStatus(err error) int {
	switch {
	case errors.Is(err, importlists.ErrListNotFound), errors.Is(err, importlists.ErrExclusionNotFound):
		return http.StatusNotFound
	case strings.Contains(err.Error(), "required"):
		return http.StatusBadRequest
	default:
		return http.StatusBadGateway
	}
}
