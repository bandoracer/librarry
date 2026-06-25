package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/settings"
)

type Dependencies struct {
	Logger   *slog.Logger
	Config   config.Config
	Metadata *metadata.Service
	Acquire  *acquisition.Service
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	handler := &handler{deps: deps}

	mux.HandleFunc("GET /healthz", handler.health)
	mux.HandleFunc("GET /api/v1/providers/health", handler.providerHealth)
	mux.HandleFunc("GET /api/v1/providers/diagnostics", handler.providerDiagnostics)
	mux.HandleFunc("GET /api/v1/search", handler.search)
	mux.HandleFunc("POST /api/v1/settings/validate", handler.validateSettings)
	mux.HandleFunc("GET /api/v1/integrations/health", handler.integrationHealth)
	mux.HandleFunc("POST /api/v1/integrations/bootstrap", handler.integrationBootstrap)
	mux.HandleFunc("POST /api/v1/releases/search", handler.releaseSearch)
	mux.HandleFunc("POST /api/v1/grabs", handler.grab)
	mux.HandleFunc("GET /api/v1/downloads", handler.downloads)

	return withCORS(deps.Config.WebOrigin, mux)
}

type handler struct {
	deps Dependencies
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"service":   "librarry",
		"checkedAt": time.Now().UTC(),
	})
}

func (h *handler) providerHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"providers": h.deps.Metadata.Health(r.Context()),
	})
}

func (h *handler) providerDiagnostics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"providers": h.deps.Metadata.Diagnostics(r.Context()),
	})
}

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	queryText := strings.TrimSpace(r.URL.Query().Get("query"))
	if queryText == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query is required"})
		return
	}

	query := metadata.Query{
		Query:  queryText,
		Type:   metadata.SearchType(defaultString(r.URL.Query().Get("type"), string(metadata.SearchTypeBook))),
		Format: metadata.MediaFormat(defaultString(r.URL.Query().Get("format"), string(metadata.FormatAny))),
		Limit:  10,
	}
	outcome := h.deps.Metadata.SearchDetailed(r.Context(), query)
	if len(outcome.ProviderErrors) > 0 {
		h.deps.Logger.Warn("search completed with provider errors", "errors", outcome.ProviderErrors)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query":          outcome.Query,
		"results":        outcome.Results,
		"providerErrors": outcome.ProviderErrors,
	})
}

func (h *handler) validateSettings(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input settings.Settings
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid settings payload"})
		return
	}
	writeJSON(w, http.StatusOK, settings.Validate(input))
}

func (h *handler) integrationHealth(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusOK, map[string]any{"integrations": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"integrations": h.deps.Acquire.Health(r.Context())})
}

func (h *handler) integrationBootstrap(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	result, err := h.deps.Acquire.Bootstrap(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) releaseSearch(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var query acquisition.ReleaseSearchQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid release search payload"})
		return
	}
	if strings.TrimSpace(query.Query) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query is required"})
		return
	}
	releases, err := h.deps.Acquire.Search(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"releases": releases})
}

func (h *handler) grab(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request acquisition.DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid grab payload"})
		return
	}
	status, err := h.deps.Acquire.Grab(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) downloads(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	statuses, err := h.deps.Acquire.Downloads(r.Context(), r.URL.Query().Get("tag"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"downloads": statuses})
}

func withCORS(webOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (webOrigin == "*" || origin == webOrigin || strings.HasPrefix(origin, "http://127.0.0.1:517")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
