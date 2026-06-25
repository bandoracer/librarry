package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/settings"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type Dependencies struct {
	Logger   *slog.Logger
	Config   config.Config
	Metadata *metadata.Service
	Acquire  acquisitionService
	Wanted   wantedService
}

type acquisitionService interface {
	Health(ctx context.Context) []acquisition.IntegrationHealth
	Bootstrap(ctx context.Context) (acquisition.BootstrapResult, error)
	Search(ctx context.Context, query acquisition.ReleaseSearchQuery) ([]acquisition.Release, error)
	Grab(ctx context.Context, request acquisition.DownloadRequest) (acquisition.DownloadStatus, error)
	Downloads(ctx context.Context, query acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error)
	DownloadAction(ctx context.Context, request acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error)
}

type wantedService interface {
	Create(ctx context.Context, request wanted.CreateRequest) (wanted.WantedItem, error)
	List(ctx context.Context, status string) ([]wanted.WantedItem, error)
	SearchReleases(ctx context.Context, wantedID string, request wanted.SearchReleasesRequest) (wanted.SearchOutcome, error)
	ListReleases(ctx context.Context, wantedID string) (wanted.SearchOutcome, error)
	Grab(ctx context.Context, wantedID string, request wanted.GrabRequest) (acquisition.DownloadStatus, error)
	Monitor(ctx context.Context, request wanted.MonitorRequest) (wanted.MonitorRun, error)
	History(ctx context.Context, query wanted.HistoryQuery) ([]wanted.HistoryEvent, error)
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
	mux.HandleFunc("POST /api/v1/downloads/actions", handler.downloadAction)
	mux.HandleFunc("GET /api/v1/wanted", handler.listWanted)
	mux.HandleFunc("POST /api/v1/wanted", handler.createWanted)
	mux.HandleFunc("POST /api/v1/wanted/monitor", handler.monitorWanted)
	mux.HandleFunc("POST /api/v1/wanted/{id}/search", handler.searchWantedReleases)
	mux.HandleFunc("GET /api/v1/wanted/{id}/releases", handler.listWantedReleases)
	mux.HandleFunc("POST /api/v1/wanted/{id}/grab", handler.grabWanted)
	mux.HandleFunc("GET /api/v1/history", handler.history)

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
	statuses, err := h.deps.Acquire.Downloads(r.Context(), acquisition.DownloadListQuery{
		Tag:      r.URL.Query().Get("tag"),
		Category: r.URL.Query().Get("category"),
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"downloads": statuses})
}

func (h *handler) downloadAction(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request acquisition.DownloadActionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid download action payload"})
		return
	}
	result, err := h.deps.Acquire.DownloadAction(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) listWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	items, err := h.deps.Wanted.List(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"wanted": items})
}

func (h *handler) createWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid wanted payload"})
		return
	}
	item, err := h.deps.Wanted.Create(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) searchWantedReleases(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.SearchReleasesRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	outcome, err := h.deps.Wanted.SearchReleases(r.Context(), r.PathValue("id"), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) listWantedReleases(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	outcome, err := h.deps.Wanted.ListReleases(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) grabWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.GrabRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	status, err := h.deps.Wanted.Grab(r.Context(), r.PathValue("id"), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) monitorWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.MonitorRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	if request.Trigger == "" {
		request.Trigger = "manual"
	}
	run, err := h.deps.Wanted.Monitor(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "run": run})
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *handler) history(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := h.deps.Wanted.History(r.Context(), wanted.HistoryQuery{Limit: limit})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func withCORS(webOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (webOrigin == "*" || origin == webOrigin || strings.HasPrefix(origin, "http://127.0.0.1:517")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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
