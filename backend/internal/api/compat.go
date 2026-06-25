package api

import (
	"encoding/json"
	"hash/fnv"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func (h *handler) compatPing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte("pong"))
}

func (h *handler) compatSystemStatus(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	writeJSON(w, http.StatusOK, map[string]any{
		"appName":           "Librarry",
		"instanceName":      "Librarry",
		"version":           "0.1.0",
		"buildTime":         now.Format(time.RFC3339),
		"isDebug":           false,
		"isProduction":      true,
		"isAdmin":           false,
		"isUserInteractive": false,
		"startupPath":       mustGetwd(),
		"appData":           "/config",
		"osName":            runtime.GOOS,
		"osVersion":         runtime.GOOS,
		"isMonoRuntime":     false,
		"isNetCore":         true,
		"isLinux":           runtime.GOOS == "linux",
		"isOsx":             runtime.GOOS == "darwin",
		"isWindows":         runtime.GOOS == "windows",
		"mode":              "console",
		"branch":            "main",
		"authentication":    "none",
		"migrationVersion":  0,
		"urlBase":           "",
		"runtimeVersion":    runtime.Version(),
		"runtimeName":       "go",
		"databaseType":      databaseType(h.deps.Config.DatabaseURL),
	})
}

func (h *handler) compatSystemRoutes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{
		{"method": "GET", "path": "/ping"},
		{"method": "GET", "path": "/api/v1/system/status"},
		{"method": "GET", "path": "/api/v1/health"},
		{"method": "GET", "path": "/api/v1/diskspace"},
		{"method": "GET", "path": "/api/v1/config/naming"},
		{"method": "GET", "path": "/api/v1/config/mediamanagement"},
		{"method": "GET", "path": "/api/v1/config/host"},
		{"method": "GET", "path": "/api/v1/config/ui"},
		{"method": "GET", "path": "/api/v1/config/downloadclient"},
		{"method": "GET", "path": "/api/v1/config/indexer"},
		{"method": "GET", "path": "/api/v1/calendar"},
		{"method": "GET", "path": "/api/v1/history"},
		{"method": "GET", "path": "/api/v1/history/since"},
		{"method": "GET", "path": "/api/v1/history/author"},
		{"method": "GET", "path": "/api/v1/history/book"},
		{"method": "GET", "path": "/api/v1/parse"},
		{"method": "GET", "path": "/api/v1/rootfolder"},
		{"method": "GET", "path": "/api/v1/queue"},
		{"method": "GET", "path": "/api/v1/queue/status"},
		{"method": "GET", "path": "/api/v1/blocklist"},
		{"method": "DELETE", "path": "/api/v1/blocklist/{id}"},
		{"method": "DELETE", "path": "/api/v1/blocklist/bulk"},
		{"method": "GET", "path": "/api/v1/blacklist"},
		{"method": "DELETE", "path": "/api/v1/blacklist/{id}"},
		{"method": "DELETE", "path": "/api/v1/blacklist/bulk"},
		{"method": "GET", "path": "/api/v1/author"},
		{"method": "GET", "path": "/api/v1/author/lookup"},
		{"method": "GET", "path": "/api/v1/book"},
		{"method": "GET", "path": "/api/v1/book/lookup"},
		{"method": "GET", "path": "/api/v1/bookfile"},
		{"method": "GET", "path": "/api/v1/bookfile/{id}"},
		{"method": "PUT", "path": "/api/v1/bookfile/{id}"},
		{"method": "DELETE", "path": "/api/v1/bookfile/{id}"},
		{"method": "DELETE", "path": "/api/v1/bookfile/bulk"},
		{"method": "GET", "path": "/api/v1/rename"},
		{"method": "GET", "path": "/api/v1/wanted/missing"},
		{"method": "GET", "path": "/api/v1/qualityprofile"},
		{"method": "GET", "path": "/api/v1/delayprofile"},
		{"method": "GET", "path": "/api/v1/qualitydefinition"},
		{"method": "GET", "path": "/api/v1/languageprofile"},
		{"method": "GET", "path": "/api/v1/metadataprofile"},
		{"method": "GET", "path": "/api/v1/customformat"},
		{"method": "GET", "path": "/api/v1/tag"},
		{"method": "GET", "path": "/api/v1/restriction"},
		{"method": "GET", "path": "/api/v1/notification"},
		{"method": "GET", "path": "/api/v1/importlist"},
		{"method": "GET", "path": "/api/v1/remotepathmapping"},
		{"method": "GET", "path": "/api/v1/downloadclient"},
		{"method": "GET", "path": "/api/v1/indexer"},
		{"method": "GET", "path": "/api/v1/release"},
		{"method": "POST", "path": "/api/v1/release"},
		{"method": "GET", "path": "/api/v1/manualimport"},
		{"method": "POST", "path": "/api/v1/manualimport"},
		{"method": "GET", "path": "/api/v1/command"},
		{"method": "POST", "path": "/api/v1/command"},
		{"method": "GET", "path": "/api/v1/system/task"},
	})
}

func (h *handler) compatSystemDuplicateRoutes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{})
}

func (h *handler) compatHealth(w http.ResponseWriter, r *http.Request) {
	var records []map[string]any
	if h.deps.Metadata != nil {
		for _, provider := range h.deps.Metadata.Health(r.Context()) {
			if provider.Status != "ready" && provider.Status != "stub" {
				records = append(records, map[string]any{
					"source":  provider.Name,
					"type":    "warning",
					"message": provider.Message,
				})
			}
		}
	}
	if h.deps.Acquire != nil {
		for _, integration := range h.deps.Acquire.Health(r.Context()) {
			if !integration.Configured || integration.Status == "ready" {
				continue
			}
			records = append(records, map[string]any{
				"source":  integration.Name,
				"type":    "error",
				"message": integration.Message,
			})
		}
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatDiskspace(w http.ResponseWriter, r *http.Request) {
	roots := h.compatRootFolderRecords()
	records := make([]map[string]any, 0, len(roots))
	for _, root := range roots {
		records = append(records, map[string]any{
			"path":       root["path"],
			"label":      root["name"],
			"freeSpace":  root["freeSpace"],
			"totalSpace": root["totalSpace"],
		})
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatNamingConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.compatNamingConfigRecord(nil))
}

func (h *handler) compatUpdateNamingConfig(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var payload map[string]any
	_ = json.NewDecoder(r.Body).Decode(&payload)
	writeJSON(w, http.StatusOK, h.compatNamingConfigRecord(payload))
}

func (h *handler) compatNamingConfigExamples(w http.ResponseWriter, r *http.Request) {
	record := h.compatNamingConfigRecord(nil)
	author := "Andy Weir"
	title := "Project Hail Mary"
	format := "ebook"
	ext := ".epub"
	writeJSON(w, http.StatusOK, map[string]any{
		"singleBookExample": renderCompatNamingExample(record, author, title, format, ext),
		"examples": []map[string]any{{
			"author":      author,
			"title":       title,
			"mediaFormat": format,
			"extension":   ext,
			"path":        renderCompatNamingExample(record, author, title, format, ext),
		}},
	})
}

func (h *handler) compatMediaManagementConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.compatMediaManagementConfigRecord(nil))
}

func (h *handler) compatUpdateMediaManagementConfig(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var payload map[string]any
	_ = json.NewDecoder(r.Body).Decode(&payload)
	writeJSON(w, http.StatusOK, h.compatMediaManagementConfigRecord(payload))
}

func (h *handler) compatHostConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.compatHostConfigRecord(nil))
}

func (h *handler) compatUpdateHostConfig(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "host config")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.compatHostConfigRecord(payload))
}

func (h *handler) compatUIConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.compatUIConfigRecord(nil))
}

func (h *handler) compatUpdateUIConfig(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "UI config")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.compatUIConfigRecord(payload))
}

func (h *handler) compatDownloadClientConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.compatDownloadClientConfigRecord(nil))
}

func (h *handler) compatUpdateDownloadClientConfig(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "download client config")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.compatDownloadClientConfigRecord(payload))
}

func (h *handler) compatIndexerConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.compatIndexerConfigRecord(nil))
}

func (h *handler) compatUpdateIndexerConfig(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "indexer config")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.compatIndexerConfigRecord(payload))
}

func (h *handler) compatCalendar(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	items, err := h.deps.Wanted.List(r.Context(), "")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	start := parseTimeQuery(r.URL.Query().Get("start"))
	end := parseTimeQuery(r.URL.Query().Get("end"))
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		record := compatCalendarRecord(item)
		airDate := calendarDateForItem(item)
		if !timeInRange(airDate, start, end) {
			continue
		}
		records = append(records, record)
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatHistory(w http.ResponseWriter, r *http.Request) {
	events, ok := h.compatHistoryEvents(w, r)
	if !ok {
		return
	}
	records := compatHistoryRecords(events)
	page, pageSize := pageParams(r, len(records))
	writeJSON(w, http.StatusOK, map[string]any{
		"page":          page,
		"pageSize":      pageSize,
		"sortKey":       defaultString(r.URL.Query().Get("sortKey"), "date"),
		"sortDirection": defaultString(r.URL.Query().Get("sortDirection"), "descending"),
		"totalRecords":  len(records),
		"records":       pageRecords(records, page, pageSize),
	})
}

func (h *handler) compatHistorySince(w http.ResponseWriter, r *http.Request) {
	events, ok := h.compatHistoryEvents(w, r)
	if !ok {
		return
	}
	since := parseTimeQuery(firstNonEmptyString(r.URL.Query().Get("date"), r.URL.Query().Get("since")))
	records := make([]map[string]any, 0, len(events))
	for _, event := range events {
		if !since.IsZero() && event.CreatedAt.Before(since) {
			continue
		}
		records = append(records, compatHistoryRecord(event))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatHistoryAuthors(w http.ResponseWriter, r *http.Request) {
	events, ok := h.compatHistoryEvents(w, r)
	if !ok {
		return
	}
	seen := map[int]map[string]any{}
	for _, event := range events {
		record := compatHistoryRecord(event)
		author, _ := record["author"].(map[string]any)
		id, _ := author["id"].(int)
		if id == 0 {
			continue
		}
		seen[id] = author
	}
	writeJSON(w, http.StatusOK, mapsByIntKey(seen))
}

func (h *handler) compatHistoryBooks(w http.ResponseWriter, r *http.Request) {
	events, ok := h.compatHistoryEvents(w, r)
	if !ok {
		return
	}
	seen := map[int]map[string]any{}
	for _, event := range events {
		record := compatHistoryRecord(event)
		book, _ := record["book"].(map[string]any)
		id, _ := book["id"].(int)
		if id == 0 {
			continue
		}
		seen[id] = book
	}
	writeJSON(w, http.StatusOK, mapsByIntKey(seen))
}

func (h *handler) compatParse(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(firstNonEmptyString(r.URL.Query().Get("title"), r.URL.Query().Get("term")))
	if title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "title is required"})
		return
	}
	parsedTitle, parsedAuthor := parseCompatReleaseTitle(title)
	format := firstNonEmptyString(r.URL.Query().Get("format"), "ebook")
	writeJSON(w, http.StatusOK, map[string]any{
		"title":          title,
		"parsedTitle":    parsedTitle,
		"authorTitle":    parsedAuthor,
		"author":         map[string]any{"id": stableInt(parsedAuthor), "authorName": parsedAuthor, "titleSlug": slug(parsedAuthor)},
		"book":           map[string]any{"id": stableInt(parsedTitle), "title": parsedTitle, "authorTitle": parsedAuthor, "titleSlug": slug(parsedTitle)},
		"books":          []map[string]any{{"id": stableInt(parsedTitle), "title": parsedTitle, "authorTitle": parsedAuthor, "titleSlug": slug(parsedTitle)}},
		"quality":        map[string]any{"quality": map[string]any{"id": stableInt(format), "name": format}, "revision": map[string]any{"version": 1, "real": 0, "isRepack": false}},
		"languages":      []map[string]any{{"id": 1, "name": "English"}},
		"releaseTitle":   title,
		"librarryParsed": true,
	})
}

func (h *handler) compatRootFolders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.compatRootFolderRecords())
}

func (h *handler) compatRootFolder(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	for _, root := range h.compatRootFolderRecords() {
		if root["id"] == id {
			writeJSON(w, http.StatusOK, root)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "root folder not found"})
}

func (h *handler) compatCreateRootFolder(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var payload struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	path := strings.TrimSpace(payload.Path)
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "path is required"})
		return
	}
	root := compatRootFolderRecord(stableInt(path), defaultString(payload.Name, "Books"), path)
	writeJSON(w, http.StatusCreated, root)
}

func (h *handler) compatDeleteRootFolder(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) compatQueue(w http.ResponseWriter, r *http.Request) {
	downloads, ok := h.compatDownloads(w, r)
	if !ok {
		return
	}
	records := compatQueueRecords(downloads)
	page, pageSize := pageParams(r, len(records))
	writeJSON(w, http.StatusOK, map[string]any{
		"page":          page,
		"pageSize":      pageSize,
		"sortKey":       defaultString(r.URL.Query().Get("sortKey"), "timeleft"),
		"sortDirection": defaultString(r.URL.Query().Get("sortDirection"), "ascending"),
		"totalRecords":  len(records),
		"records":       pageRecords(records, page, pageSize),
	})
}

func (h *handler) compatQueueDetails(w http.ResponseWriter, r *http.Request) {
	downloads, ok := h.compatDownloads(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatQueueRecords(downloads))
}

func (h *handler) compatQueueStatus(w http.ResponseWriter, r *http.Request) {
	downloads, ok := h.compatDownloads(w, r)
	if !ok {
		return
	}
	var totalCount, count, unknownCount, errors, warnings int
	for _, download := range downloads {
		totalCount++
		if download.Progress < 1 {
			count++
		}
		switch stateTone(download.State) {
		case "error":
			errors++
		case "warning":
			warnings++
		case "unknown":
			unknownCount++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"totalCount":   totalCount,
		"count":        count,
		"unknownCount": unknownCount,
		"errors":       errors,
		"warnings":     warnings,
	})
}

func (h *handler) compatDeleteQueue(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	removeFromClient := parseBoolDefault(r.URL.Query().Get("removeFromClient"), true)
	_, err := h.deps.Acquire.DownloadAction(r.Context(), acquisition.DownloadActionRequest{
		Action:      acquisition.DownloadActionDelete,
		IDs:         []string{r.PathValue("id")},
		DeleteFiles: parseBoolDefault(r.URL.Query().Get("blocklist"), false) || removeFromClient,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) compatDeleteQueueBulk(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var payload struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid queue bulk payload"})
		return
	}
	_, err := h.deps.Acquire.DownloadAction(r.Context(), acquisition.DownloadActionRequest{
		Action:      acquisition.DownloadActionDelete,
		IDs:         payload.IDs,
		DeleteFiles: parseBoolDefault(r.URL.Query().Get("removeFromClient"), true),
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) compatBlocklist(w http.ResponseWriter, r *http.Request) {
	records, ok := h.compatBlocklistRecords(w, r)
	if !ok {
		return
	}
	page, pageSize := pageParams(r, len(records))
	writeJSON(w, http.StatusOK, map[string]any{
		"page":          page,
		"pageSize":      pageSize,
		"sortKey":       defaultString(r.URL.Query().Get("sortKey"), "date"),
		"sortDirection": defaultString(r.URL.Query().Get("sortDirection"), "descending"),
		"totalRecords":  len(records),
		"records":       pageRecords(records, page, pageSize),
	})
}

func (h *handler) compatDeleteBlocklist(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) compatDeleteBlocklistBulk(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) compatAuthors(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	subscriptions, err := h.deps.Wanted.ListAuthorSubscriptions(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	books := h.compatWantedItems(r)
	records := make([]map[string]any, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		records = append(records, compatAuthorRecord(subscription, booksForAuthor(books, subscription.AuthorName)))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatAuthor(w http.ResponseWriter, r *http.Request) {
	subscription, books, ok := h.compatFindAuthor(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatAuthorRecord(subscription, booksForAuthor(books, subscription.AuthorName)))
}

func (h *handler) compatCreateAuthor(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var payload map[string]any
	_ = json.NewDecoder(r.Body).Decode(&payload)
	request := wanted.AuthorSubscribeRequest{
		AuthorName:     firstNonEmptyString(payloadString(payload, "authorName"), payloadString(payload, "title"), payloadString(payload, "name")),
		Provider:       firstNonEmptyString(payloadString(payload, "provider"), "readarr-api"),
		ProviderKey:    firstNonEmptyString(payloadString(payload, "foreignAuthorId"), payloadString(payload, "id")),
		Format:         firstNonEmptyString(payloadString(payload, "format"), payloadString(payload, "wantedFormat"), "ebook"),
		QualityProfile: firstNonEmptyString(payloadString(payload, "qualityProfile"), nestedString(payload, "qualityProfile", "name"), "standard"),
	}
	monitor := true
	if monitored, ok := payload["monitored"].(bool); ok {
		monitor = monitored
	}
	request.MonitorNewItems = &monitor
	if request.AuthorName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "authorName is required"})
		return
	}
	subscription, err := h.deps.Wanted.SubscribeAuthor(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, compatAuthorRecord(subscription, nil))
}

func (h *handler) compatUpdateAuthor(w http.ResponseWriter, r *http.Request) {
	subscription, books, ok := h.compatFindAuthor(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatAuthorRecord(subscription, booksForAuthor(books, subscription.AuthorName)))
}

func (h *handler) compatDeleteAuthor(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) compatAuthorLookup(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(firstNonEmptyString(r.URL.Query().Get("term"), r.URL.Query().Get("query")))
	if term == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "term is required"})
		return
	}
	if h.deps.Metadata == nil {
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}
	outcome := h.deps.Metadata.SearchDetailed(r.Context(), metadata.Query{
		Query:  term,
		Type:   metadata.SearchTypeAuthor,
		Format: metadata.FormatAny,
		Limit:  20,
	})
	records := make([]map[string]any, 0, len(outcome.Results))
	for _, result := range outcome.Results {
		records = append(records, compatAuthorLookupRecord(result))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatBooks(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	items, err := h.deps.Wanted.List(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		records = append(records, compatBookRecord(item))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatBook(w http.ResponseWriter, r *http.Request) {
	item, ok := h.compatFindBook(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatBookRecord(item))
}

func (h *handler) compatBookOverview(w http.ResponseWriter, r *http.Request) {
	item, ok := h.compatFindBook(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatBookRecord(item))
}

func (h *handler) compatCreateBook(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var payload map[string]any
	_ = json.NewDecoder(r.Body).Decode(&payload)
	title := firstNonEmptyString(payloadString(payload, "title"), nestedString(payload, "book", "title"))
	authorName := firstNonEmptyString(payloadString(payload, "authorName"), payloadString(payload, "authorTitle"), nestedString(payload, "author", "authorName"))
	if title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "title is required"})
		return
	}
	request := wanted.CreateRequest{
		Result: metadata.SearchResult{
			Provider:   firstNonEmptyString(payloadString(payload, "provider"), "readarr-api"),
			Kind:       metadata.SearchTypeBook,
			Score:      1,
			Confidence: "manual",
			MatchedOn:  []string{"readarr-api"},
			Work: metadata.Work{
				ID:       firstNonEmptyString(payloadString(payload, "foreignBookId"), payloadString(payload, "id"), "readarr:book:"+slug(title)),
				Title:    title,
				CoverURL: payloadString(payload, "remoteCover"),
				Authors:  []metadata.Author{{ID: firstNonEmptyString(nestedString(payload, "author", "foreignAuthorId"), "readarr:author:"+slug(authorName)), Name: authorName}},
			},
			Edition: metadata.Edition{
				ID:     firstNonEmptyString(payloadString(payload, "editionId"), "readarr:edition:"+slug(title)),
				Title:  title,
				Format: metadata.MediaFormat(firstNonEmptyString(payloadString(payload, "format"), "ebook")),
			},
		},
		Format:         firstNonEmptyString(payloadString(payload, "format"), "ebook"),
		QualityProfile: firstNonEmptyString(payloadString(payload, "qualityProfile"), nestedString(payload, "qualityProfile", "name"), "standard"),
	}
	item, err := h.deps.Wanted.Create(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, compatBookRecord(item))
}

func (h *handler) compatUpdateBook(w http.ResponseWriter, r *http.Request) {
	item, ok := h.compatFindBook(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatBookRecord(item))
}

func (h *handler) compatMonitorBooks(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	items, err := h.deps.Wanted.List(r.Context(), "")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		records = append(records, compatBookRecord(item))
	}
	writeJSON(w, http.StatusAccepted, records)
}

func (h *handler) compatDeleteBook(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) compatBookFiles(w http.ResponseWriter, r *http.Request) {
	files, items, ok := h.compatBookFileSource(w, r)
	if !ok {
		return
	}
	records := make([]map[string]any, 0, len(files))
	for _, file := range files {
		item := wantedItemForFile(file, items)
		if !bookFileMatchesQuery(r, file, item) {
			continue
		}
		records = append(records, compatBookFileRecord(file, item))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatBookFile(w http.ResponseWriter, r *http.Request) {
	file, item, ok := h.compatFindBookFile(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatBookFileRecord(file, item))
}

func (h *handler) compatUpdateBookFile(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	file, item, ok := h.compatFindBookFile(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	var payload map[string]any
	_ = json.NewDecoder(r.Body).Decode(&payload)
	record := compatBookFileRecord(file, item)
	if payload != nil {
		if qualityName := payloadQualityName(payload); qualityName != "" {
			record["quality"] = compatReleaseQuality(qualityName)
		}
	}
	record["librarryCompatibilityNote"] = "bookfile update is accepted for Readarr API compatibility; Librarry does not mutate library files through this endpoint yet"
	writeJSON(w, http.StatusOK, record)
}

func (h *handler) compatDeleteBookFile(w http.ResponseWriter, r *http.Request) {
	file, _, ok := h.compatFindBookFile(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	outcome, err := h.deps.Library.DeleteFiles(r.Context(), library.DeleteFilesRequest{
		IDs:         []string{file.ID},
		Paths:       []string{file.Path},
		DeleteFiles: parseBoolDefault(r.URL.Query().Get("deleteFiles"), false),
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "outcome": outcome})
		return
	}
	if outcome.Errored > 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "book file delete failed", "outcome": outcome})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) compatDeleteBookFileBulk(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	var payload map[string]any
	_ = json.NewDecoder(r.Body).Decode(&payload)
	files, _, ok := h.compatBookFileSource(w, r)
	if !ok {
		return
	}
	ids := bookFileDeleteIDs(payload)
	if len(ids) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var deleteIDs []string
	var deletePaths []string
	for _, file := range files {
		if bookFileMatchesAnyID(file, ids) {
			deleteIDs = append(deleteIDs, file.ID)
			deletePaths = append(deletePaths, file.Path)
		}
	}
	if len(deleteIDs) == 0 && len(deletePaths) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	outcome, err := h.deps.Library.DeleteFiles(r.Context(), library.DeleteFilesRequest{
		IDs:         deleteIDs,
		Paths:       deletePaths,
		DeleteFiles: payloadBoolDefault(payload, "deleteFiles", parseBoolDefault(r.URL.Query().Get("deleteFiles"), false)),
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "outcome": outcome})
		return
	}
	if outcome.Errored > 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "book file delete failed", "outcome": outcome})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) compatBookFileSource(w http.ResponseWriter, r *http.Request) ([]library.FileRecord, []wanted.WantedItem, bool) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return nil, nil, false
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	files, err := h.deps.Library.ListFiles(r.Context(), library.FileListQuery{
		Format: r.URL.Query().Get("format"),
		Status: r.URL.Query().Get("status"),
		Limit:  limit,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return nil, nil, false
	}
	return files, h.compatWantedItemsBestEffort(r), true
}

func (h *handler) compatFindBookFile(w http.ResponseWriter, r *http.Request, id string) (library.FileRecord, *wanted.WantedItem, bool) {
	files, items, ok := h.compatBookFileSource(w, r)
	if !ok {
		return library.FileRecord{}, nil, false
	}
	for _, file := range files {
		if compatIDMatches(id, file.ID, file.Path, file.Title, filepath.Base(file.Path)) {
			return file, wantedItemForFile(file, items), true
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "book file not found"})
	return library.FileRecord{}, nil, false
}

func (h *handler) compatWantedItemsBestEffort(r *http.Request) []wanted.WantedItem {
	if h.deps.Wanted == nil {
		return nil
	}
	items, err := h.deps.Wanted.List(r.Context(), "")
	if err != nil {
		return nil
	}
	return items
}

func (h *handler) compatRenamePreview(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	files, items, ok := h.compatBookFileSource(w, r)
	if !ok {
		return
	}
	request := renameRequestFromCompatFiles(r, files, items)
	if len(request.IDs) == 0 && len(request.Paths) == 0 {
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}
	outcome, err := h.deps.Library.PreviewRenameFiles(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "outcome": outcome})
		return
	}
	records := make([]map[string]any, 0, len(outcome.Previews))
	for _, preview := range outcome.Previews {
		records = append(records, compatRenamePreviewRecord(preview, wantedItemForFile(preview.File, items)))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatBookLookup(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(firstNonEmptyString(r.URL.Query().Get("term"), r.URL.Query().Get("query")))
	if term == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "term is required"})
		return
	}
	if h.deps.Metadata == nil {
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}
	outcome := h.deps.Metadata.SearchDetailed(r.Context(), metadata.Query{
		Query:  term,
		Type:   metadata.SearchTypeBook,
		Format: metadata.FormatAny,
		Limit:  20,
	})
	records := make([]map[string]any, 0, len(outcome.Results))
	for _, result := range outcome.Results {
		records = append(records, compatBookLookupRecord(result))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatWantedMissing(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	items, err := h.deps.Wanted.List(r.Context(), "wanted")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		records = append(records, compatMissingRecord(item))
	}
	page, pageSize := pageParams(r, len(records))
	writeJSON(w, http.StatusOK, map[string]any{
		"page":          page,
		"pageSize":      pageSize,
		"sortKey":       defaultString(r.URL.Query().Get("sortKey"), "title"),
		"sortDirection": defaultString(r.URL.Query().Get("sortDirection"), "ascending"),
		"totalRecords":  len(records),
		"records":       pageRecords(records, page, pageSize),
	})
}

func (h *handler) compatQualityProfiles(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	profiles, err := h.deps.Wanted.ListQualityProfiles(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	records := make([]map[string]any, 0, len(profiles))
	for i, profile := range profiles {
		records = append(records, compatQualityProfileRecord(i+1, profile))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatDelayProfiles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{compatDelayProfileRecord(nil, 1)})
}

func (h *handler) compatDelayProfile(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, compatDelayProfileRecord(map[string]any{"id": r.PathValue("id")}, pathValueInt(r, "id")))
}

func (h *handler) compatCreateDelayProfile(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "delay profile")
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, compatDelayProfileRecord(payload, stablePayloadID(payload, "delay-profile")))
}

func (h *handler) compatUpdateDelayProfile(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "delay profile")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatDelayProfileRecord(payload, pathValueInt(r, "id")))
}

func (h *handler) compatQualityDefinitions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, compatQualityDefinitionRecords())
}

func (h *handler) compatUpdateQualityDefinition(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "quality definition")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatQualityDefinitionRecordFromPayload(pathValueInt(r, "id"), payload))
}

func (h *handler) compatLanguageProfiles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{compatLanguageProfileRecord(nil, 1)})
}

func (h *handler) compatLanguageProfile(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, compatLanguageProfileRecord(map[string]any{"id": r.PathValue("id")}, pathValueInt(r, "id")))
}

func (h *handler) compatCreateLanguageProfile(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "language profile")
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, compatLanguageProfileRecord(payload, stablePayloadID(payload, "language-profile")))
}

func (h *handler) compatUpdateLanguageProfile(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "language profile")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatLanguageProfileRecord(payload, pathValueInt(r, "id")))
}

func (h *handler) compatMetadataProfiles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{compatMetadataProfileRecord(nil, 1)})
}

func (h *handler) compatMetadataProfile(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, compatMetadataProfileRecord(map[string]any{"id": r.PathValue("id")}, pathValueInt(r, "id")))
}

func (h *handler) compatCreateMetadataProfile(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "metadata profile")
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, compatMetadataProfileRecord(payload, stablePayloadID(payload, "metadata-profile")))
}

func (h *handler) compatUpdateMetadataProfile(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "metadata profile")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatMetadataProfileRecord(payload, pathValueInt(r, "id")))
}

func (h *handler) compatCustomFormats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{})
}

func (h *handler) compatCustomFormat(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, compatCustomFormatRecord(map[string]any{"id": r.PathValue("id")}, pathValueInt(r, "id")))
}

func (h *handler) compatCreateCustomFormat(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "custom format")
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, compatCustomFormatRecord(payload, stablePayloadID(payload, "custom-format")))
}

func (h *handler) compatUpdateCustomFormat(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "custom format")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatCustomFormatRecord(payload, pathValueInt(r, "id")))
}

func (h *handler) compatTags(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{compatTagRecord(map[string]any{"label": "librarry"}, stableInt("librarry"))})
}

func (h *handler) compatTag(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, compatTagRecord(map[string]any{"id": r.PathValue("id"), "label": "librarry"}, pathValueInt(r, "id")))
}

func (h *handler) compatCreateTag(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "tag")
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, compatTagRecord(payload, stablePayloadID(payload, "tag")))
}

func (h *handler) compatUpdateTag(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "tag")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatTagRecord(payload, pathValueInt(r, "id")))
}

func (h *handler) compatRestrictions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{})
}

func (h *handler) compatRestriction(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, compatRestrictionRecord(map[string]any{"id": r.PathValue("id")}, pathValueInt(r, "id")))
}

func (h *handler) compatCreateRestriction(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "restriction")
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, compatRestrictionRecord(payload, stablePayloadID(payload, "restriction")))
}

func (h *handler) compatUpdateRestriction(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "restriction")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatRestrictionRecord(payload, pathValueInt(r, "id")))
}

func (h *handler) compatNotifications(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{})
}

func (h *handler) compatNotification(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, compatNotificationRecord(map[string]any{"id": r.PathValue("id")}, pathValueInt(r, "id")))
}

func (h *handler) compatCreateNotification(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "notification")
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, compatNotificationRecord(payload, stablePayloadID(payload, "notification")))
}

func (h *handler) compatUpdateNotification(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "notification")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatNotificationRecord(payload, pathValueInt(r, "id")))
}

func (h *handler) compatImportLists(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{})
}

func (h *handler) compatImportList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, compatImportListRecord(map[string]any{"id": r.PathValue("id")}, pathValueInt(r, "id")))
}

func (h *handler) compatCreateImportList(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "import list")
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, compatImportListRecord(payload, stablePayloadID(payload, "import-list")))
}

func (h *handler) compatUpdateImportList(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "import list")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatImportListRecord(payload, pathValueInt(r, "id")))
}

func (h *handler) compatRemotePathMappings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{})
}

func (h *handler) compatRemotePathMapping(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, compatRemotePathMappingRecord(map[string]any{"id": r.PathValue("id")}, pathValueInt(r, "id")))
}

func (h *handler) compatCreateRemotePathMapping(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "remote path mapping")
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, compatRemotePathMappingRecord(payload, stablePayloadID(payload, "remote-path-mapping")))
}

func (h *handler) compatUpdateRemotePathMapping(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeCompatObjectPayload(w, r, "remote path mapping")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, compatRemotePathMappingRecord(payload, pathValueInt(r, "id")))
}

func (h *handler) compatDeleteResource(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) compatDownloadClients(w http.ResponseWriter, r *http.Request) {
	var records []map[string]any
	if strings.TrimSpace(h.deps.Config.QBittorrentURL) != "" {
		records = append(records, compatDownloadClientRecord(1, "qBittorrent", "torrent", h.deps.Config.QBittorrentURL, h.deps.Config.EbookCategory))
	}
	if strings.TrimSpace(h.deps.Config.SABnzbdURL) != "" {
		records = append(records, compatDownloadClientRecord(2, "SABnzbd", "usenet", h.deps.Config.SABnzbdURL, h.deps.Config.EbookCategory))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatIndexers(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(h.deps.Config.ProwlarrURL) == "" {
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, []map[string]any{{
		"id":                      1,
		"name":                    "Prowlarr",
		"implementation":          "Torznab",
		"implementationName":      "Prowlarr",
		"protocol":                "torrent",
		"enableRss":               true,
		"enableAutomaticSearch":   true,
		"enableInteractiveSearch": true,
		"priority":                25,
		"fields": []map[string]any{
			{"name": "baseUrl", "value": h.deps.Config.ProwlarrURL},
			{"name": "apiKey", "value": maskedValue(h.deps.Config.ProwlarrAPIKey)},
		},
	}})
}

func (h *handler) compatReleases(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	query, item := h.compatReleaseSearchQuery(r)
	if strings.TrimSpace(query.Query) == "" && strings.TrimSpace(query.ISBN) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "term, query, isbn, or bookId is required"})
		return
	}
	releases, err := h.deps.Acquire.Search(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	records := make([]map[string]any, 0, len(releases))
	for _, release := range releases {
		records = append(records, compatReleaseRecord(release, item))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatGrabRelease(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil && h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid release grab payload"})
		return
	}

	wantedID := h.compatWantedIDForRelease(r, payload)
	releaseID := firstNonEmptyString(payloadString(payload, "releaseId"), payloadString(payload, "id"), payloadString(payload, "guid"), payloadString(payload, "sourceId"))
	paused := payloadBoolDefault(payload, "paused", true)
	if h.deps.Wanted != nil && wantedID != "" && releaseID != "" && compatReleaseURLFromPayload(payload) == "" {
		status, err := h.deps.Wanted.Grab(r.Context(), wantedID, wanted.GrabRequest{ReleaseID: releaseID, Paused: paused})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, compatGrabbedReleaseRecord(status, wantedID))
		return
	}

	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	releaseURL := compatReleaseURLFromPayload(payload)
	if releaseURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "downloadUrl, releaseUrl, or guid is required"})
		return
	}
	format := firstNonEmptyString(payloadString(payload, "format"), payloadString(payload, "mediaFormat"), nestedString(payload, "book", "librarryFormat"))
	tags := []string{"librarry", "readarr-api"}
	if wantedID != "" {
		tags = append(tags, "wanted:"+wantedID)
	}
	request := acquisition.DownloadRequest{
		ReleaseURL: releaseURL,
		InfoHash:   firstNonEmptyString(payloadString(payload, "infoHash"), nestedString(payload, "librarryRelease", "infoHash")),
		Title:      firstNonEmptyString(payloadString(payload, "title"), payloadString(payload, "releaseTitle"), nestedString(payload, "librarryRelease", "title")),
		Protocol:   compatProtocolFromPayload(payload, releaseURL),
		Category:   firstNonEmptyString(payloadString(payload, "category"), h.compatCategoryForFormat(format)),
		SavePath:   firstNonEmptyString(payloadString(payload, "savePath"), h.deps.Config.BookTorrentRoot, acquisition.DefaultTorrentRoot),
		Paused:     paused,
		Tags:       tags,
	}
	status, err := h.deps.Acquire.Grab(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, compatGrabbedReleaseRecord(status, wantedID))
}

func (h *handler) compatManualImport(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	folder := strings.TrimSpace(firstNonEmptyString(r.URL.Query().Get("folder"), r.URL.Query().Get("path")))
	var records []map[string]any
	if folder != "" {
		outcome, err := h.deps.Library.Scan(r.Context(), library.ScanRequest{Root: folder, Limit: limit, Format: r.URL.Query().Get("format")})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		for _, file := range outcome.Files {
			records = append(records, compatManualImportFileRecord(file, "folderScan"))
		}
		writeJSON(w, http.StatusOK, records)
		return
	}
	reviews, err := h.deps.Library.ListImportReviews(r.Context(), library.ReviewListQuery{Status: defaultString(r.URL.Query().Get("status"), "pending"), Limit: limit})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	downloadID := strings.TrimSpace(r.URL.Query().Get("downloadId"))
	for _, review := range reviews {
		if downloadID != "" && review.DownloadID != downloadID {
			continue
		}
		records = append(records, compatManualImportReviewRecord(review))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatCreateManualImport(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	defer r.Body.Close()
	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid manual import payload"})
		return
	}
	items := manualImportPayloads(payload)
	if len(items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "at least one manual import item is required"})
		return
	}
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		sourcePath := firstNonEmptyString(payloadString(item, "path"), payloadString(item, "sourcePath"))
		if sourcePath == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "path is required"})
			return
		}
		request := library.ImportRequest{
			SourcePath: sourcePath,
			WantedID:   h.compatWantedIDForManualImport(r, item),
			DownloadID: firstNonEmptyString(payloadString(item, "downloadId"), payloadString(item, "downloadID")),
			Format:     firstNonEmptyString(payloadString(item, "mediaFormat"), payloadString(item, "format"), nestedString(item, "quality", "name"), nestedString(item, "book", "librarryFormat")),
			Move:       manualImportMove(item),
		}
		outcome, err := h.deps.Library.Import(r.Context(), request)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "path": sourcePath})
			return
		}
		records = append(records, compatManualImportOutcomeRecord(outcome, request))
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *handler) compatCommands(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{})
}

func (h *handler) compatCreateCommand(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	payload := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	name := firstNonEmptyString(payloadString(payload, "name"), payloadString(payload, "commandName"), "Unknown")
	command := map[string]any{
		"id":          stableInt(name + time.Now().UTC().Format(time.RFC3339Nano)),
		"name":        name,
		"commandName": name,
		"status":      "completed",
		"queued":      time.Now().UTC(),
		"started":     time.Now().UTC(),
		"ended":       time.Now().UTC(),
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "rsssync":
		if h.deps.Wanted != nil {
			run, err := h.deps.Wanted.FeedSync(r.Context(), wanted.FeedSyncRequest{Trigger: "api"})
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "command": command})
				return
			}
			command["body"] = run
		}
	case "missingbooksearch":
		if h.deps.Wanted != nil {
			run, err := h.deps.Wanted.Monitor(r.Context(), wanted.MonitorRequest{Trigger: "api", Force: true})
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "command": command})
				return
			}
			command["body"] = run
		}
	case "refreshauthor":
		if h.deps.Wanted != nil {
			run, err := h.deps.Wanted.MonitorAuthors(r.Context(), wanted.AuthorMonitorRequest{Trigger: "api", Force: true})
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "command": command})
				return
			}
			command["body"] = run
		}
	case "renamefiles", "renamebookfiles", "renamebooks":
		if h.deps.Library != nil {
			request, ok := h.renameRequestFromCommandPayload(w, r, payload)
			if !ok {
				return
			}
			if len(request.IDs) == 0 && len(request.Paths) == 0 {
				command["body"] = library.RenameFilesOutcome{}
				break
			}
			run, err := h.deps.Library.RenameFiles(r.Context(), request)
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "command": command})
				return
			}
			command["body"] = run
		}
	case "rescanfolders":
		if h.deps.Library != nil {
			run, err := h.deps.Library.Scan(r.Context(), library.ScanRequest{})
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "command": command})
				return
			}
			command["body"] = run
		}
	}
	writeJSON(w, http.StatusCreated, command)
}

func (h *handler) compatSystemTasks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.compatSystemTaskRecords())
}

func (h *handler) compatSystemTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	for _, task := range h.compatSystemTaskRecords() {
		if compatIDMatches(id, payloadString(task, "name"), payloadString(task, "taskName")) || payloadString(task, "id") == id {
			writeJSON(w, http.StatusOK, task)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "task not found"})
}

func (h *handler) compatDownloads(w http.ResponseWriter, r *http.Request) ([]acquisition.DownloadStatus, bool) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return nil, false
	}
	downloads, err := h.deps.Acquire.Downloads(r.Context(), acquisition.DownloadListQuery{Tag: "librarry"})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return nil, false
	}
	return downloads, true
}

func (h *handler) compatBlocklistRecords(w http.ResponseWriter, r *http.Request) ([]map[string]any, bool) {
	items := h.compatWantedItems(r)
	recordsByID := map[int]map[string]any{}
	if h.deps.Acquire != nil {
		downloads, err := h.deps.Acquire.Downloads(r.Context(), acquisition.DownloadListQuery{Tag: "librarry"})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return nil, false
		}
		for _, download := range downloads {
			if !downloadIsBlocklisted(download) {
				continue
			}
			record := compatBlocklistDownloadRecord(download, items)
			if id, ok := record["id"].(int); ok {
				recordsByID[id] = record
			}
		}
	}
	if h.deps.Wanted != nil {
		limit, _ := strconv.Atoi(firstNonEmptyString(r.URL.Query().Get("limit"), r.URL.Query().Get("pageSize")))
		if limit <= 0 {
			limit = 100
		}
		events, err := h.deps.Wanted.History(r.Context(), wanted.HistoryQuery{Limit: limit})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return nil, false
		}
		for _, event := range events {
			if !historyEventIsBlocklisted(event) {
				continue
			}
			record := compatBlocklistHistoryRecord(event, items)
			if id, ok := record["id"].(int); ok {
				recordsByID[id] = record
			}
		}
	}
	records := mapsByIntKey(recordsByID)
	sort.SliceStable(records, func(i, j int) bool {
		return compatRecordDate(records[i]).After(compatRecordDate(records[j]))
	})
	return records, true
}

func (h *handler) compatHistoryEvents(w http.ResponseWriter, r *http.Request) ([]wanted.HistoryEvent, bool) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return nil, false
	}
	limit, _ := strconv.Atoi(firstNonEmptyString(r.URL.Query().Get("limit"), r.URL.Query().Get("pageSize")))
	if limit <= 0 {
		limit = 100
	}
	events, err := h.deps.Wanted.History(r.Context(), wanted.HistoryQuery{Limit: limit})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return nil, false
	}
	return events, true
}

func (h *handler) compatWantedItems(r *http.Request) []wanted.WantedItem {
	if h.deps.Wanted == nil {
		return nil
	}
	items, err := h.deps.Wanted.List(r.Context(), "")
	if err != nil {
		return nil
	}
	return items
}

func (h *handler) compatFindAuthor(w http.ResponseWriter, r *http.Request, id string) (wanted.AuthorSubscription, []wanted.WantedItem, bool) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return wanted.AuthorSubscription{}, nil, false
	}
	subscriptions, err := h.deps.Wanted.ListAuthorSubscriptions(r.Context(), "")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return wanted.AuthorSubscription{}, nil, false
	}
	for _, subscription := range subscriptions {
		if compatIDMatches(id, subscription.ID, subscription.ProviderKey, subscription.AuthorName) {
			return subscription, h.compatWantedItems(r), true
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "author not found"})
	return wanted.AuthorSubscription{}, nil, false
}

func (h *handler) compatFindBook(w http.ResponseWriter, r *http.Request, id string) (wanted.WantedItem, bool) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return wanted.WantedItem{}, false
	}
	items, err := h.deps.Wanted.List(r.Context(), "")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return wanted.WantedItem{}, false
	}
	for _, item := range items {
		if compatIDMatches(id, item.ID, item.WorkID, item.SourceKey, item.Title) {
			return item, true
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "book not found"})
	return wanted.WantedItem{}, false
}

func (h *handler) compatWantedIDForManualImport(r *http.Request, payload map[string]any) string {
	wantedID := firstNonEmptyString(payloadString(payload, "wantedId"), payloadString(payload, "librarryWantedId"))
	if wantedID != "" {
		return wantedID
	}
	bookID := firstNonEmptyString(payloadString(payload, "bookId"), payloadString(payload, "bookID"), nestedString(payload, "book", "id"), nestedString(payload, "book", "librarryId"))
	if bookID == "" {
		return ""
	}
	for _, item := range h.compatWantedItems(r) {
		if compatIDMatches(bookID, item.ID, item.WorkID, item.SourceKey, item.Title) {
			return item.ID
		}
	}
	return ""
}

func (h *handler) compatReleaseSearchQuery(r *http.Request) (acquisition.ReleaseSearchQuery, *wanted.WantedItem) {
	limit, _ := strconv.Atoi(firstNonEmptyString(r.URL.Query().Get("limit"), r.URL.Query().Get("pageSize")))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	item := h.compatFindWantedItemForID(r, firstNonEmptyString(
		r.URL.Query().Get("wantedId"),
		r.URL.Query().Get("librarryWantedId"),
		r.URL.Query().Get("bookId"),
		r.URL.Query().Get("bookID"),
	))
	queryText := firstNonEmptyString(r.URL.Query().Get("term"), r.URL.Query().Get("query"), r.URL.Query().Get("title"))
	author := firstNonEmptyString(r.URL.Query().Get("author"), r.URL.Query().Get("authorName"), r.URL.Query().Get("authorTitle"))
	format := firstNonEmptyString(r.URL.Query().Get("format"), r.URL.Query().Get("mediaFormat"))
	if item != nil {
		queryText = firstNonEmptyString(queryText, item.Title)
		author = firstNonEmptyString(author, item.AuthorName)
		format = firstNonEmptyString(format, item.Format)
	}
	isbn := firstNonEmptyString(r.URL.Query().Get("isbn"), r.URL.Query().Get("isbn13"), r.URL.Query().Get("isbn10"))
	return acquisition.ReleaseSearchQuery{
		Query:  firstNonEmptyString(queryText, isbn),
		Author: author,
		ISBN:   isbn,
		Format: format,
		Limit:  limit,
	}, item
}

func (h *handler) compatWantedIDForRelease(r *http.Request, payload map[string]any) string {
	wantedID := firstNonEmptyString(payloadString(payload, "wantedId"), payloadString(payload, "librarryWantedId"))
	if wantedID != "" {
		return wantedID
	}
	bookID := firstNonEmptyString(payloadString(payload, "bookId"), payloadString(payload, "bookID"), nestedString(payload, "book", "id"), nestedString(payload, "book", "librarryId"))
	if item := h.compatFindWantedItemForID(r, bookID); item != nil {
		return item.ID
	}
	return ""
}

func (h *handler) compatFindWantedItemForID(r *http.Request, id string) *wanted.WantedItem {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	for _, item := range h.compatWantedItems(r) {
		if compatIDMatches(id, item.ID, item.WorkID, item.SourceKey, item.Title) {
			matched := item
			return &matched
		}
	}
	return nil
}

func (h *handler) compatCategoryForFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "audiobook", "audio":
		return firstNonEmptyString(h.deps.Config.AudiobookCategory, acquisition.CategoryBooksAudiobook)
	default:
		return firstNonEmptyString(h.deps.Config.EbookCategory, acquisition.CategoryBooksEbook)
	}
}

func (h *handler) compatRootFolderRecords() []map[string]any {
	return []map[string]any{
		compatRootFolderRecord(1, "Ebooks", defaultString(h.deps.Config.EbookLibraryRoot, "/data/media/books/ebooks")),
		compatRootFolderRecord(2, "Audiobooks", defaultString(h.deps.Config.AudiobookLibraryRoot, "/data/media/books/audiobooks")),
	}
}

func (h *handler) compatNamingConfigRecord(overrides map[string]any) map[string]any {
	authorFolder := firstNonEmptyString(payloadString(overrides, "authorFolderFormat"), payloadString(overrides, "authorFolderTemplate"), h.deps.Config.NamingAuthorFolder)
	bookFolder := firstNonEmptyString(payloadString(overrides, "bookFolderFormat"), payloadString(overrides, "bookFolderTemplate"), h.deps.Config.NamingBookFolder)
	fileName := firstNonEmptyString(payloadString(overrides, "standardBookFormat"), payloadString(overrides, "fileNameFormat"), h.deps.Config.NamingFileName)
	spaceReplacement := firstNonEmptyString(payloadString(overrides, "replaceSpacesWith"), h.deps.Config.NamingSpaceReplacement)
	replaceSpaces := payloadBoolDefault(overrides, "replaceSpaces", spaceReplacement != "")
	return map[string]any{
		"id":                           1,
		"renameBooks":                  payloadBoolDefault(overrides, "renameBooks", true),
		"replaceIllegalCharacters":     payloadBoolDefault(overrides, "replaceIllegalCharacters", true),
		"colonReplacementFormat":       firstNonEmptyString(payloadString(overrides, "colonReplacementFormat"), "delete"),
		"standardBookFormat":           defaultString(fileName, "{Title}{Ext}"),
		"authorFolderFormat":           defaultString(authorFolder, "{Author}"),
		"bookFolderFormat":             defaultString(bookFolder, "{Title}"),
		"includeAuthorName":            payloadBoolDefault(overrides, "includeAuthorName", true),
		"includeBookTitle":             payloadBoolDefault(overrides, "includeBookTitle", true),
		"includeQuality":               payloadBoolDefault(overrides, "includeQuality", false),
		"replaceSpaces":                replaceSpaces,
		"replaceSpacesWith":            spaceReplacement,
		"multiAuthorStyle":             firstNonEmptyString(payloadString(overrides, "multiAuthorStyle"), "standard"),
		"librarryAuthorFolderTemplate": defaultString(authorFolder, "{Author}"),
		"librarryBookFolderTemplate":   defaultString(bookFolder, "{Title}"),
		"librarryFileNameTemplate":     defaultString(fileName, "{Title}{Ext}"),
	}
}

func (h *handler) compatMediaManagementConfigRecord(overrides map[string]any) map[string]any {
	return map[string]any{
		"id":                                     1,
		"autoUnmonitorPreviouslyDownloadedBooks": payloadBoolDefault(overrides, "autoUnmonitorPreviouslyDownloadedBooks", false),
		"recycleBin":                             payloadString(overrides, "recycleBin"),
		"recycleBinCleanupDays":                  payloadIntDefault(overrides, "recycleBinCleanupDays", 7),
		"downloadPropersAndRepacks":              firstNonEmptyString(payloadString(overrides, "downloadPropersAndRepacks"), "preferAndUpgrade"),
		"createEmptyAuthorFolders":               payloadBoolDefault(overrides, "createEmptyAuthorFolders", false),
		"deleteEmptyFolders":                     payloadBoolDefault(overrides, "deleteEmptyFolders", true),
		"fileDate":                               firstNonEmptyString(payloadString(overrides, "fileDate"), "none"),
		"rescanAfterRefresh":                     firstNonEmptyString(payloadString(overrides, "rescanAfterRefresh"), "always"),
		"setPermissionsLinux":                    payloadBoolDefault(overrides, "setPermissionsLinux", false),
		"chmodFolder":                            firstNonEmptyString(payloadString(overrides, "chmodFolder"), "755"),
		"chownGroup":                             payloadString(overrides, "chownGroup"),
		"skipFreeSpaceCheckWhenImporting":        payloadBoolDefault(overrides, "skipFreeSpaceCheckWhenImporting", false),
		"minimumFreeSpaceWhenImporting":          payloadIntDefault(overrides, "minimumFreeSpaceWhenImporting", 100),
		"copyUsingHardlinks":                     payloadBoolDefault(overrides, "copyUsingHardlinks", false),
		"importExtraFiles":                       payloadBoolDefault(overrides, "importExtraFiles", false),
		"extraFileExtensions":                    payloadString(overrides, "extraFileExtensions"),
		"enableMediaInfo":                        payloadBoolDefault(overrides, "enableMediaInfo", true),
		"ebookRootFolderPath":                    defaultString(h.deps.Config.EbookLibraryRoot, "/data/media/books/ebooks"),
		"audiobookRootFolderPath":                defaultString(h.deps.Config.AudiobookLibraryRoot, "/data/media/books/audiobooks"),
	}
}

func (h *handler) compatHostConfigRecord(overrides map[string]any) map[string]any {
	return map[string]any{
		"id":                         1,
		"bindAddress":                firstNonEmptyString(payloadString(overrides, "bindAddress"), "*"),
		"port":                       payloadIntDefault(overrides, "port", compatListenPort(h.deps.Config.ListenAddr, 8080)),
		"sslPort":                    payloadIntDefault(overrides, "sslPort", 9898),
		"enableSsl":                  payloadBoolDefault(overrides, "enableSsl", false),
		"launchBrowser":              payloadBoolDefault(overrides, "launchBrowser", false),
		"authenticationMethod":       firstNonEmptyString(payloadString(overrides, "authenticationMethod"), "none"),
		"authenticationRequired":     firstNonEmptyString(payloadString(overrides, "authenticationRequired"), "disabledForLocalAddresses"),
		"username":                   payloadString(overrides, "username"),
		"password":                   "",
		"logLevel":                   firstNonEmptyString(payloadString(overrides, "logLevel"), "info"),
		"consoleLogLevel":            firstNonEmptyString(payloadString(overrides, "consoleLogLevel"), "info"),
		"branch":                     firstNonEmptyString(payloadString(overrides, "branch"), "main"),
		"apiKey":                     payloadString(overrides, "apiKey"),
		"sslCertPath":                payloadString(overrides, "sslCertPath"),
		"sslCertPassword":            "",
		"urlBase":                    payloadString(overrides, "urlBase"),
		"instanceName":               firstNonEmptyString(payloadString(overrides, "instanceName"), "Librarry"),
		"applicationUrl":             payloadString(overrides, "applicationUrl"),
		"updateAutomatically":        payloadBoolDefault(overrides, "updateAutomatically", false),
		"analyticsEnabled":           payloadBoolDefault(overrides, "analyticsEnabled", false),
		"proxyEnabled":               payloadBoolDefault(overrides, "proxyEnabled", false),
		"proxyType":                  firstNonEmptyString(payloadString(overrides, "proxyType"), "http"),
		"proxyHostname":              payloadString(overrides, "proxyHostname"),
		"proxyPort":                  payloadIntDefault(overrides, "proxyPort", 0),
		"proxyUsername":              payloadString(overrides, "proxyUsername"),
		"proxyPassword":              "",
		"proxyBypassFilter":          payloadString(overrides, "proxyBypassFilter"),
		"proxyBypassLocalAddresses":  payloadBoolDefault(overrides, "proxyBypassLocalAddresses", true),
		"backupFolder":               firstNonEmptyString(payloadString(overrides, "backupFolder"), "Backups"),
		"backupInterval":             payloadIntDefault(overrides, "backupInterval", 7),
		"backupRetention":            payloadIntDefault(overrides, "backupRetention", 28),
		"librarryConfigSource":       "env",
		"librarryPersistedViaNative": false,
	}
}

func (h *handler) compatUIConfigRecord(overrides map[string]any) map[string]any {
	return map[string]any{
		"id":                         1,
		"firstDayOfWeek":             payloadIntDefault(overrides, "firstDayOfWeek", 0),
		"calendarWeekColumnHeader":   firstNonEmptyString(payloadString(overrides, "calendarWeekColumnHeader"), "ddd M/D"),
		"shortDateFormat":            firstNonEmptyString(payloadString(overrides, "shortDateFormat"), "yyyy-MM-dd"),
		"longDateFormat":             firstNonEmptyString(payloadString(overrides, "longDateFormat"), "yyyy-MM-dd HH:mm"),
		"timeFormat":                 firstNonEmptyString(payloadString(overrides, "timeFormat"), "HH:mm"),
		"showRelativeDates":          payloadBoolDefault(overrides, "showRelativeDates", true),
		"enableColorImpairedMode":    payloadBoolDefault(overrides, "enableColorImpairedMode", false),
		"theme":                      firstNonEmptyString(payloadString(overrides, "theme"), "auto"),
		"uiLanguage":                 payloadIntDefault(overrides, "uiLanguage", 1),
		"expandBookByDefault":        payloadBoolDefault(overrides, "expandBookByDefault", false),
		"librarryPersistedViaNative": false,
	}
}

func (h *handler) compatDownloadClientConfigRecord(overrides map[string]any) map[string]any {
	return map[string]any{
		"id":                               1,
		"downloadClientWorkingFolders":     payloadString(overrides, "downloadClientWorkingFolders"),
		"enableCompletedDownloadHandling":  payloadBoolDefault(overrides, "enableCompletedDownloadHandling", true),
		"removeCompletedDownloads":         payloadBoolDefault(overrides, "removeCompletedDownloads", false),
		"autoRedownloadFailed":             payloadBoolDefault(overrides, "autoRedownloadFailed", h.deps.Config.FailedDownloadAutoGrab),
		"checkForFinishedDownloadInterval": payloadIntDefault(overrides, "checkForFinishedDownloadInterval", 1),
		"librarryTorrentRoot":              defaultString(h.deps.Config.BookTorrentRoot, acquisition.DefaultTorrentRoot),
		"librarryEbookCategory":            defaultString(h.deps.Config.EbookCategory, acquisition.CategoryBooksEbook),
		"librarryAudiobookCategory":        defaultString(h.deps.Config.AudiobookCategory, acquisition.CategoryBooksAudiobook),
		"librarryPersistedViaNative":       false,
	}
}

func (h *handler) compatIndexerConfigRecord(overrides map[string]any) map[string]any {
	return map[string]any{
		"id":                         1,
		"minimumAge":                 payloadIntDefault(overrides, "minimumAge", 0),
		"retention":                  payloadIntDefault(overrides, "retention", 0),
		"maximumSize":                payloadIntDefault(overrides, "maximumSize", 0),
		"rssSyncInterval":            payloadIntDefault(overrides, "rssSyncInterval", durationMinutes(h.deps.Config.FeedSyncInterval, 15)),
		"preferIndexerFlags":         payloadBoolDefault(overrides, "preferIndexerFlags", false),
		"allowHardcodedSubs":         payloadBoolDefault(overrides, "allowHardcodedSubs", true),
		"availabilityDelay":          payloadIntDefault(overrides, "availabilityDelay", 0),
		"librarryProwlarrUrl":        h.deps.Config.ProwlarrURL,
		"librarryPersistedViaNative": false,
	}
}

func (h *handler) compatSystemTaskRecords() []map[string]any {
	now := time.Now().UTC()
	return []map[string]any{
		compatSystemTaskRecord(1, "RssSync", "Prowlarr feed sync", h.deps.Config.FeedSyncInterval, h.deps.Config.FeedSyncEnabled, now),
		compatSystemTaskRecord(2, "MissingBookSearch", "Wanted item monitor", h.deps.Config.MonitorInterval, h.deps.Config.MonitorEnabled, now),
		compatSystemTaskRecord(3, "RefreshAuthor", "Author metadata monitor", h.deps.Config.AuthorMonitorInterval, h.deps.Config.AuthorMonitorEnabled, now),
		compatSystemTaskRecord(4, "FailedDownloadCheck", "Failed download recovery", h.deps.Config.FailedDownloadInterval, h.deps.Config.FailedDownloadEnabled, now),
		compatSystemTaskRecord(5, "UpgradeSearch", "Wanted upgrade search", h.deps.Config.UpgradeSearchInterval, h.deps.Config.UpgradeSearchEnabled, now),
	}
}

func compatRootFolderRecord(id int, name string, path string) map[string]any {
	total, free, accessible := statPath(path)
	return map[string]any{
		"id":              id,
		"name":            name,
		"path":            path,
		"accessible":      accessible,
		"freeSpace":       free,
		"totalSpace":      total,
		"unmappedFolders": []any{},
	}
}

func compatCalendarRecord(item wanted.WantedItem) map[string]any {
	date := calendarDateForItem(item)
	return map[string]any{
		"id":             stableInt(item.ID),
		"title":          item.Title,
		"authorTitle":    item.AuthorName,
		"releaseDate":    date,
		"airDate":        date.Format("2006-01-02"),
		"airDateUtc":     date,
		"monitored":      item.Status != "ignored",
		"anyEditionOk":   true,
		"qualityProfile": item.QualityProfile,
		"status":         item.Status,
		"author": map[string]any{
			"id":         stableInt(item.AuthorName),
			"authorName": item.AuthorName,
			"titleSlug":  slug(item.AuthorName),
			"monitored":  true,
		},
		"book":           compatBookRecord(item),
		"librarryId":     item.ID,
		"librarryFormat": item.Format,
	}
}

func calendarDateForItem(item wanted.WantedItem) time.Time {
	switch {
	case item.LastSearchAt != nil:
		return item.LastSearchAt.UTC()
	case item.LastUpgradeSearchAt != nil:
		return item.LastUpgradeSearchAt.UTC()
	case !item.UpdatedAt.IsZero():
		return item.UpdatedAt.UTC()
	case !item.CreatedAt.IsZero():
		return item.CreatedAt.UTC()
	default:
		return time.Now().UTC()
	}
}

func compatHistoryRecords(events []wanted.HistoryEvent) []map[string]any {
	records := make([]map[string]any, 0, len(events))
	for _, event := range events {
		records = append(records, compatHistoryRecord(event))
	}
	return records
}

func compatHistoryRecord(event wanted.HistoryEvent) map[string]any {
	title := historyTitle(event)
	authorName := historyAuthor(event)
	bookID := stableInt(firstNonEmptyString(historyDataString(event, "wantedId"), event.EntityID, title))
	record := map[string]any{
		"id":          stableInt(firstNonEmptyString(event.ID, event.EventType+event.CreatedAt.Format(time.RFC3339Nano))),
		"eventType":   compatHistoryEventType(event.EventType),
		"sourceTitle": title,
		"date":        event.CreatedAt,
		"quality": map[string]any{
			"quality":  map[string]any{"id": stableInt(historyDataString(event, "format")), "name": firstNonEmptyString(historyDataString(event, "format"), "ebook")},
			"revision": map[string]any{"version": 1, "real": 0, "isRepack": false},
		},
		"languages": []map[string]any{{"id": 1, "name": "English"}},
		"data":      historyData(event),
		"author": map[string]any{
			"id":         stableInt(authorName),
			"authorName": authorName,
			"titleSlug":  slug(authorName),
		},
		"book": map[string]any{
			"id":          bookID,
			"title":       title,
			"authorTitle": authorName,
			"titleSlug":   slug(title),
		},
		"librarryEventType": event.EventType,
		"librarrySeverity":  event.Severity,
		"librarryMessage":   event.Message,
	}
	if downloadID := historyDataString(event, "downloadId"); downloadID != "" {
		record["downloadId"] = downloadID
	}
	if releaseID := historyDataString(event, "releaseId"); releaseID != "" {
		record["releaseId"] = releaseID
	}
	return record
}

func compatManualImportReviewRecord(review library.ImportReview) map[string]any {
	record := compatManualImportBaseRecord(stableInt(firstNonEmptyString(review.ID, review.SourcePath)), review.SourcePath, review.MediaFormat, review.Title, review.AuthorName, review.SizeBytes)
	record["downloadId"] = review.DownloadID
	record["librarryReviewId"] = review.ID
	record["librarryStatus"] = review.Status
	record["librarryReason"] = review.Reason
	if strings.TrimSpace(review.WantedID) != "" {
		record["librarryWantedId"] = review.WantedID
		record["bookId"] = stableInt(review.WantedID)
	}
	if strings.TrimSpace(review.Reason) != "" {
		record["rejections"] = []map[string]any{{"reason": review.Reason, "type": "warning"}}
	}
	return record
}

func compatManualImportFileRecord(file library.FileRecord, source string) map[string]any {
	record := compatManualImportBaseRecord(stableInt(firstNonEmptyString(file.ID, file.Path)), file.Path, file.MediaFormat, file.Title, file.AuthorName, file.SizeBytes)
	record["librarryFileId"] = file.ID
	record["librarryImportStatus"] = file.ImportStatus
	record["librarrySource"] = source
	return record
}

func compatManualImportOutcomeRecord(outcome library.ImportOutcome, request library.ImportRequest) map[string]any {
	record := compatManualImportFileRecord(outcome.File, "manualImport")
	record["imported"] = true
	record["destinationPath"] = outcome.DestinationPath
	record["importMode"] = map[bool]string{true: "move", false: "copy"}[outcome.Moved]
	record["downloadId"] = request.DownloadID
	if strings.TrimSpace(request.WantedID) != "" {
		record["librarryWantedId"] = request.WantedID
		record["bookId"] = stableInt(request.WantedID)
	}
	return record
}

func compatManualImportBaseRecord(id int, path string, mediaFormat string, title string, authorName string, sizeBytes int64) map[string]any {
	mediaFormat = defaultString(mediaFormat, "ebook")
	title = defaultString(title, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	qualityName := strings.ToUpper(mediaFormat[:1]) + mediaFormat[1:]
	return map[string]any{
		"id":                   id,
		"path":                 path,
		"relativePath":         filepath.Base(path),
		"folderName":           filepath.Dir(path),
		"name":                 filepath.Base(path),
		"size":                 sizeBytes,
		"qualityWeight":        1,
		"downloadId":           "",
		"additionalFile":       false,
		"replaceExistingFiles": false,
		"disableReleaseSwitch": false,
		"importMode":           "copy",
		"rejections":           []map[string]any{},
		"author": map[string]any{
			"id":         stableInt(authorName),
			"authorName": authorName,
			"titleSlug":  slug(authorName),
		},
		"book": map[string]any{
			"id":             stableInt(title),
			"title":          title,
			"authorTitle":    authorName,
			"titleSlug":      slug(title),
			"librarryFormat": mediaFormat,
		},
		"quality": map[string]any{
			"quality": map[string]any{
				"id":   stableInt(mediaFormat),
				"name": qualityName,
			},
			"revision": map[string]any{
				"version":  1,
				"real":     0,
				"isRepack": false,
			},
		},
		"languages": []map[string]any{{"id": 1, "name": "English"}},
	}
}

func compatBookFileRecord(file library.FileRecord, item *wanted.WantedItem) map[string]any {
	recordID := stableInt(firstNonEmptyString(file.ID, file.Path))
	title := defaultString(file.Title, strings.TrimSuffix(filepath.Base(file.Path), filepath.Ext(file.Path)))
	authorName := file.AuthorName
	if item != nil {
		title = defaultString(item.Title, title)
		authorName = defaultString(item.AuthorName, authorName)
	}
	format := blocklistFormat(file.MediaFormat, file.Extension, file.Path, item)
	bookID := stableInt(title)
	authorID := stableInt(authorName)
	if item != nil {
		bookID = stableInt(item.ID)
		authorID = stableInt(item.AuthorName)
	}
	editionID := stableInt(firstNonEmptyString(file.EditionID, title, file.Path))
	record := map[string]any{
		"id":                  recordID,
		"librarryId":          file.ID,
		"bookId":              bookID,
		"authorId":            authorID,
		"editionId":           editionID,
		"path":                file.Path,
		"relativePath":        filepath.Base(file.Path),
		"folderName":          filepath.Dir(file.Path),
		"size":                file.SizeBytes,
		"dateAdded":           file.CreatedAt,
		"modified":            file.ModifiedAt,
		"quality":             compatReleaseQuality(format),
		"qualityCutoffNotMet": false,
		"qualityWeight":       1,
		"language":            map[string]any{"id": 1, "name": "English"},
		"languages":           []map[string]any{{"id": 1, "name": "English"}},
		"mediaInfo":           map[string]any{},
		"book":                compatBookFileBookRecord(title, authorName, format, item),
		"author":              compatBookFileAuthorRecord(authorName, item),
		"edition": map[string]any{
			"id":               editionID,
			"foreignEditionId": firstNonEmptyString(file.EditionID, fileMetadataString(file, "editionId")),
			"title":            title,
			"format":           format,
		},
		"bookFileType":          format,
		"calibreId":             0,
		"partCount":             1,
		"sceneName":             "",
		"releaseGroup":          "",
		"librarryMediaFormat":   file.MediaFormat,
		"librarryImportStatus":  file.ImportStatus,
		"librarrySourcePath":    file.SourcePath,
		"librarryChecksum":      file.Checksum,
		"librarryCompatibility": "readarr-bookfile",
	}
	if !file.UpdatedAt.IsZero() {
		record["librarryUpdatedAt"] = file.UpdatedAt
	}
	return record
}

func compatBookFileBookRecord(title string, authorName string, format string, item *wanted.WantedItem) map[string]any {
	if item != nil {
		return compatBookRecord(*item)
	}
	return map[string]any{
		"id":             stableInt(title),
		"title":          title,
		"authorTitle":    authorName,
		"authorId":       stableInt(authorName),
		"titleSlug":      slug(title),
		"monitored":      false,
		"anyEditionOk":   true,
		"statistics":     map[string]any{"bookFileCount": 1},
		"librarryFormat": format,
	}
}

func compatBookFileAuthorRecord(authorName string, item *wanted.WantedItem) map[string]any {
	if item != nil {
		return map[string]any{
			"id":         stableInt(item.AuthorName),
			"authorName": item.AuthorName,
			"titleSlug":  slug(item.AuthorName),
			"monitored":  true,
		}
	}
	return map[string]any{
		"id":         stableInt(authorName),
		"authorName": authorName,
		"titleSlug":  slug(authorName),
		"monitored":  false,
	}
}

func compatRenamePreviewRecord(preview library.RenameFilePreview, item *wanted.WantedItem) map[string]any {
	bookFile := compatBookFileRecord(preview.File, item)
	bookID := stableInt(preview.File.Title)
	authorID := stableInt(preview.File.AuthorName)
	if item != nil {
		bookID = stableInt(item.ID)
		authorID = stableInt(item.AuthorName)
	}
	return map[string]any{
		"id":              stableInt(preview.File.ID + preview.DestinationPath),
		"authorId":        authorID,
		"bookId":          bookID,
		"bookFileId":      stableInt(firstNonEmptyString(preview.File.ID, preview.File.Path)),
		"existingPath":    preview.SourcePath,
		"newPath":         preview.DestinationPath,
		"path":            preview.SourcePath,
		"newPathState":    map[bool]string{true: "exists", false: "available"}[preview.Exists],
		"proper":          false,
		"noop":            preview.Noop,
		"bookFile":        bookFile,
		"book":            bookFile["book"],
		"author":          bookFile["author"],
		"quality":         bookFile["quality"],
		"librarryPreview": preview,
	}
}

func wantedItemForFile(file library.FileRecord, items []wanted.WantedItem) *wanted.WantedItem {
	for _, item := range items {
		if compatIDMatches(firstNonEmptyString(file.EditionID, fileMetadataString(file, "editionId")), item.EditionID, item.WorkID, item.ID, item.SourceKey) {
			matched := item
			return &matched
		}
	}
	fileTitle := strings.TrimSpace(file.Title)
	fileAuthor := strings.TrimSpace(file.AuthorName)
	fileFormat := blocklistFormat(file.MediaFormat, file.Extension, file.Path)
	for _, item := range items {
		if fileTitle != "" && !strings.EqualFold(fileTitle, strings.TrimSpace(item.Title)) {
			continue
		}
		if fileAuthor != "" && !strings.EqualFold(fileAuthor, strings.TrimSpace(item.AuthorName)) {
			continue
		}
		if strings.TrimSpace(item.Format) != "" && !strings.EqualFold(blocklistFormat(&item), fileFormat) {
			continue
		}
		matched := item
		return &matched
	}
	return nil
}

func renameRequestFromCompatFiles(r *http.Request, files []library.FileRecord, items []wanted.WantedItem) library.RenameFilesRequest {
	request := library.RenameFilesRequest{
		Overwrite: parseBoolDefault(r.URL.Query().Get("overwrite"), false),
	}
	for _, file := range files {
		item := wantedItemForFile(file, items)
		if bookFileMatchesQuery(r, file, item) {
			request.IDs = append(request.IDs, file.ID)
		}
	}
	return request
}

func (h *handler) renameRequestFromCommandPayload(w http.ResponseWriter, r *http.Request, payload map[string]any) (library.RenameFilesRequest, bool) {
	request := library.RenameFilesRequest{
		Overwrite: payloadBoolDefault(payload, "overwrite", parseBoolDefault(r.URL.Query().Get("overwrite"), false)),
	}
	files, items, ok := h.compatBookFileSource(w, r)
	if !ok {
		return request, false
	}
	ids := renameCommandFileIDs(payload)
	paths := renameCommandPaths(payload)
	seen := map[string]bool{}
	matchedPaths := map[string]bool{}
	for _, file := range files {
		item := wantedItemForFile(file, items)
		matchesID := len(ids) > 0 && bookFileMatchesAnyID(file, ids)
		matchesPath := len(paths) > 0 && bookFileMatchesAnyPath(file, paths)
		matchesSelection := len(ids) == 0 && len(paths) == 0 && (bookFileMatchesPayloadSelection(file, item, payload) || bookFileQueryHasFilter(r) && bookFileMatchesQuery(r, file, item))
		if !matchesID && !matchesPath && !matchesSelection {
			continue
		}
		if !seen[file.ID] {
			request.IDs = append(request.IDs, file.ID)
			seen[file.ID] = true
		}
		for _, path := range paths {
			if compatIDMatches(path, file.Path, file.SourcePath, filepath.Base(file.Path)) {
				matchedPaths[path] = true
			}
		}
	}
	if len(request.IDs) == 0 {
		for _, id := range ids {
			if _, err := strconv.Atoi(id); err != nil {
				request.IDs = append(request.IDs, id)
			}
		}
	}
	for _, path := range paths {
		if !matchedPaths[path] {
			request.Paths = append(request.Paths, path)
		}
	}
	request.IDs = firstUniqueStrings(request.IDs)
	request.Paths = firstUniqueStrings(request.Paths)
	return request, true
}

func bookFileMatchesQuery(r *http.Request, file library.FileRecord, item *wanted.WantedItem) bool {
	if bookID := strings.TrimSpace(r.URL.Query().Get("bookId")); bookID != "" {
		candidates := []string{file.ID, file.EditionID, file.Title, file.Path, filepath.Base(file.Path)}
		if item != nil {
			candidates = append(candidates, item.ID, item.WorkID, item.EditionID, item.SourceKey, item.Title)
		}
		if !compatIDMatches(bookID, candidates...) {
			return false
		}
	}
	if authorID := strings.TrimSpace(r.URL.Query().Get("authorId")); authorID != "" {
		candidates := []string{file.AuthorName}
		if item != nil {
			candidates = append(candidates, item.AuthorName)
		}
		if !compatIDMatches(authorID, candidates...) {
			return false
		}
	}
	return true
}

func bookFileQueryHasFilter(r *http.Request) bool {
	return strings.TrimSpace(r.URL.Query().Get("bookId")) != "" || strings.TrimSpace(r.URL.Query().Get("authorId")) != ""
}

func bookFileDeleteIDs(payload map[string]any) []string {
	return payloadStringList(payload, "bookFileIds", "bookFileIDs", "ids", "bookFileId", "bookFileID", "id")
}

func bookFileMatchesAnyID(file library.FileRecord, ids []string) bool {
	for _, id := range ids {
		if compatIDMatches(id, file.ID, file.Path, file.Title, filepath.Base(file.Path)) {
			return true
		}
	}
	return false
}

func bookFileMatchesAnyPath(file library.FileRecord, paths []string) bool {
	for _, path := range paths {
		if compatIDMatches(path, file.Path, file.SourcePath, filepath.Base(file.Path)) {
			return true
		}
	}
	return false
}

func bookFileMatchesPayloadSelection(file library.FileRecord, item *wanted.WantedItem, payload map[string]any) bool {
	matched := false
	bookIDs := payloadStringList(payload, "bookIds", "bookIDs", "bookId", "bookID")
	if len(bookIDs) > 0 {
		matched = true
		candidates := []string{file.ID, file.EditionID, file.Title, file.Path, filepath.Base(file.Path)}
		if item != nil {
			candidates = append(candidates, item.ID, item.WorkID, item.EditionID, item.SourceKey, item.Title)
		}
		if !anyCompatIDMatches(bookIDs, candidates...) {
			return false
		}
	}
	authorIDs := payloadStringList(payload, "authorIds", "authorIDs", "authorId", "authorID")
	if len(authorIDs) > 0 {
		matched = true
		candidates := []string{file.AuthorName}
		if item != nil {
			candidates = append(candidates, item.AuthorName)
		}
		if !anyCompatIDMatches(authorIDs, candidates...) {
			return false
		}
	}
	return matched
}

func anyCompatIDMatches(values []string, candidates ...string) bool {
	for _, value := range values {
		if compatIDMatches(value, candidates...) {
			return true
		}
	}
	return false
}

func renameCommandFileIDs(payload map[string]any) []string {
	ids := bookFileDeleteIDs(payload)
	for _, value := range compatPayloadArray(payload, "files") {
		switch typed := value.(type) {
		case map[string]any:
			ids = append(ids, payloadStringList(typed, "id", "bookFileId", "bookFileID", "librarryId")...)
		default:
			if text := stringValue(value); text != "" {
				ids = append(ids, text)
			}
		}
	}
	return firstUniqueStrings(ids)
}

func renameCommandPaths(payload map[string]any) []string {
	paths := payloadStringList(payload, "paths", "path", "existingPath", "sourcePath")
	for _, value := range compatPayloadArray(payload, "files") {
		file, ok := value.(map[string]any)
		if !ok {
			continue
		}
		paths = append(paths, payloadStringList(file, "path", "existingPath", "sourcePath")...)
	}
	return firstUniqueStrings(paths)
}

func payloadStringList(payload map[string]any, keys ...string) []string {
	var values []string
	if payload == nil {
		return values
	}
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok || raw == nil {
			continue
		}
		if list, ok := raw.([]any); ok {
			for _, value := range list {
				if text := stringValue(value); text != "" {
					values = append(values, text)
				}
			}
			continue
		}
		if text := stringValue(raw); text != "" {
			values = append(values, text)
		}
	}
	return firstUniqueStrings(values)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(jsonString(typed)), `"`), `"`))
	}
}

func firstUniqueStrings(values []string) []string {
	unique := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}

func fileMetadataString(file library.FileRecord, key string) string {
	if file.Metadata == nil {
		return ""
	}
	value, ok := file.Metadata[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(jsonString(value)), `"`), `"`))
}

func compatReleaseRecord(release acquisition.Release, item *wanted.WantedItem) map[string]any {
	releaseID := firstNonEmptyString(release.ID, release.InfoHash, release.DownloadURL, release.Title)
	title, authorName := compatReleaseTitleAndAuthor(release, item)
	format := compatReleaseFormat(release, item)
	publishedAt := release.PublishedAt
	if publishedAt.IsZero() {
		publishedAt = time.Now().UTC()
	}
	ageMinutes := int(time.Since(publishedAt).Minutes())
	if ageMinutes < 0 {
		ageMinutes = 0
	}
	rejections := compatReleaseRejections(release)
	book := compatReleaseBookRecord(title, authorName, item)
	author := compatReleaseAuthorRecord(authorName, item)
	return map[string]any{
		"id":              stableInt(releaseID),
		"guid":            releaseID,
		"sourceId":        release.ID,
		"title":           release.Title,
		"releaseTitle":    release.Title,
		"indexer":         defaultString(release.Indexer, "Prowlarr"),
		"indexerId":       stableInt(defaultString(release.Indexer, "Prowlarr")),
		"size":            release.SizeBytes,
		"seeders":         release.Seeders,
		"leechers":        release.Leechers,
		"downloadUrl":     release.DownloadURL,
		"infoUrl":         release.InfoURL,
		"infoHash":        release.InfoHash,
		"protocol":        compatReleaseProtocol(release),
		"publishDate":     publishedAt,
		"age":             ageMinutes / (60 * 24),
		"ageHours":        ageMinutes / 60,
		"ageMinutes":      ageMinutes,
		"approved":        len(rejections) == 0,
		"rejections":      rejections,
		"releaseWeight":   compatReleaseWeight(release, len(rejections) == 0),
		"quality":         compatReleaseQuality(format),
		"languages":       []map[string]any{{"id": 1, "name": "English"}},
		"author":          author,
		"book":            book,
		"bookId":          book["id"],
		"authorId":        author["id"],
		"books":           []map[string]any{book},
		"categories":      release.Categories,
		"librarryRelease": release,
	}
}

func compatGrabbedReleaseRecord(status acquisition.DownloadStatus, wantedID string) map[string]any {
	queueRecords := compatQueueRecords([]acquisition.DownloadStatus{status})
	record := map[string]any{
		"grabbed":  true,
		"download": status,
	}
	if len(queueRecords) > 0 {
		for key, value := range queueRecords[0] {
			record[key] = value
		}
	}
	record["downloadId"] = status.ID
	if wantedID != "" {
		record["librarryWantedId"] = wantedID
		record["bookId"] = stableInt(wantedID)
	}
	return record
}

func compatReleaseURLFromPayload(payload map[string]any) string {
	releaseURL := firstNonEmptyString(
		payloadString(payload, "downloadUrl"),
		payloadString(payload, "downloadURL"),
		payloadString(payload, "releaseUrl"),
		payloadString(payload, "releaseURL"),
		payloadString(payload, "magnetUrl"),
		payloadString(payload, "magnetURL"),
		payloadString(payload, "magnetUri"),
		payloadString(payload, "magnetURI"),
		payloadString(payload, "url"),
		payloadString(payload, "link"),
		nestedString(payload, "librarryRelease", "downloadUrl"),
		nestedString(payload, "librarryRelease", "downloadURL"),
	)
	if releaseURL != "" {
		return releaseURL
	}
	return compatDirectURLCandidate(payloadString(payload, "guid"))
}

func compatDirectURLCandidate(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(normalized, "magnet:") || strings.Contains(normalized, "://") {
		return strings.TrimSpace(value)
	}
	return ""
}

func compatProtocolFromPayload(payload map[string]any, releaseURL string) string {
	protocol := firstNonEmptyString(payloadString(payload, "protocol"), nestedString(payload, "librarryRelease", "protocol"))
	if protocol != "" {
		return protocol
	}
	return compatProtocolForURL(releaseURL)
}

func compatProtocolForURL(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.HasPrefix(normalized, "magnet:"):
		return "torrent"
	case strings.Contains(normalized, ".nzb"), strings.Contains(normalized, "usenet"), strings.Contains(normalized, "newznab"):
		return "usenet"
	default:
		return "torrent"
	}
}

func compatReleaseProtocol(release acquisition.Release) string {
	if strings.TrimSpace(release.Protocol) != "" {
		return release.Protocol
	}
	return compatProtocolForURL(release.DownloadURL)
}

func compatReleaseTitleAndAuthor(release acquisition.Release, item *wanted.WantedItem) (string, string) {
	if item != nil {
		return item.Title, item.AuthorName
	}
	title, author := parseCompatReleaseTitle(release.Title)
	return defaultString(title, release.Title), author
}

func compatReleaseBookRecord(title string, authorName string, item *wanted.WantedItem) map[string]any {
	if item != nil {
		return compatBookRecord(*item)
	}
	return map[string]any{
		"id":           stableInt(title),
		"title":        title,
		"authorTitle":  authorName,
		"titleSlug":    slug(title),
		"monitored":    false,
		"anyEditionOk": true,
	}
}

func compatReleaseAuthorRecord(authorName string, item *wanted.WantedItem) map[string]any {
	if item != nil {
		return map[string]any{
			"id":         stableInt(item.AuthorName),
			"authorName": item.AuthorName,
			"titleSlug":  slug(item.AuthorName),
			"monitored":  true,
		}
	}
	return map[string]any{
		"id":         stableInt(authorName),
		"authorName": authorName,
		"titleSlug":  slug(authorName),
		"monitored":  false,
	}
}

func compatReleaseFormat(release acquisition.Release, item *wanted.WantedItem) string {
	if item != nil && strings.TrimSpace(item.Format) != "" {
		return item.Format
	}
	haystack := strings.ToLower(release.Title + " " + strings.Join(release.Categories, " "))
	for _, token := range []string{"audiobook", "audio book", "m4b", "mp3"} {
		if strings.Contains(haystack, token) {
			return "audiobook"
		}
	}
	return "ebook"
}

func compatReleaseQuality(format string) map[string]any {
	qualityName := "Ebook"
	if strings.EqualFold(format, "audiobook") || strings.EqualFold(format, "audio") {
		qualityName = "Audiobook"
	}
	return map[string]any{
		"quality": map[string]any{
			"id":   stableInt(qualityName),
			"name": qualityName,
		},
		"revision": map[string]any{
			"version":  1,
			"real":     0,
			"isRepack": false,
		},
	}
}

func compatReleaseRejections(release acquisition.Release) []map[string]any {
	var rejections []map[string]any
	if strings.TrimSpace(release.DownloadURL) == "" {
		rejections = append(rejections, map[string]any{"reason": "missing download URL", "type": "error"})
	}
	return rejections
}

func compatReleaseWeight(release acquisition.Release, approved bool) int {
	if !approved {
		return 0
	}
	weight := 100
	if release.Seeders > 0 {
		weight += release.Seeders
	}
	if release.SizeBytes > 0 {
		weight += 5
	}
	return weight
}

func compatQueueRecords(downloads []acquisition.DownloadStatus) []map[string]any {
	records := make([]map[string]any, 0, len(downloads))
	for _, download := range downloads {
		sizeLeft := download.SizeBytes - download.DownloadedBytes
		if sizeLeft < 0 {
			sizeLeft = 0
		}
		records = append(records, map[string]any{
			"id":                      download.ID,
			"downloadId":              download.ID,
			"title":                   defaultString(download.Name, download.ID),
			"status":                  download.State,
			"trackedDownloadStatus":   queueTrackedStatus(download),
			"trackedDownloadState":    queueTrackedState(download),
			"statusMessages":          queueStatusMessages(download),
			"size":                    download.SizeBytes,
			"sizeleft":                sizeLeft,
			"timeleft":                download.ETASeconds,
			"estimatedCompletionTime": estimatedCompletion(download),
			"protocol":                queueProtocol(download),
			"downloadClient":          defaultString(download.Client, "qBittorrent"),
			"outputPath":              download.SavePath,
			"downloadClientId":        stableInt(defaultString(download.Client, "qBittorrent")),
			"indexer":                 "",
			"author":                  map[string]any{"authorName": ""},
			"book":                    map[string]any{"title": defaultString(download.Name, download.ID)},
		})
	}
	return records
}

func compatBlocklistDownloadRecord(download acquisition.DownloadStatus, items []wanted.WantedItem) map[string]any {
	wantedID := wantedIDFromDownload(download)
	item := wantedItemByID(items, wantedID)
	title, authorName := titleAuthorForDownload(download, item)
	reason := firstNonEmptyString(download.FailureReason, download.ImportError, "Download client reported a failed download")
	date := firstNonNilTime(download.FailedAt, download.LastActivityAt, download.CompletedAt, download.AddedAt)
	if date == nil {
		now := time.Now().UTC()
		date = &now
	}
	recordID := stableInt("download:" + download.ID)
	return map[string]any{
		"id":             recordID,
		"authorId":       stableInt(authorName),
		"bookId":         blocklistBookID(title, item),
		"sourceTitle":    defaultString(download.Name, title),
		"quality":        compatReleaseQuality(blocklistFormat(download.Name, download.Category, item)),
		"languages":      []map[string]any{{"id": 1, "name": "English"}},
		"date":           date.UTC(),
		"protocol":       queueProtocol(download),
		"indexer":        "",
		"message":        reason,
		"downloadId":     download.ID,
		"size":           download.SizeBytes,
		"downloadClient": defaultString(download.Client, "qBittorrent"),
		"author":         compatBlocklistAuthorRecord(authorName),
		"book":           compatBlocklistBookRecord(title, authorName, item),
		"librarrySource": "download",
		"librarryReason": reason,
	}
}

func compatBlocklistHistoryRecord(event wanted.HistoryEvent, items []wanted.WantedItem) map[string]any {
	wantedID := firstNonEmptyString(historyDataString(event, "wantedId"), event.EntityID)
	item := wantedItemByID(items, wantedID)
	title := blocklistHistoryTitle(event, item)
	authorName := blocklistHistoryAuthor(event, item)
	reason := firstNonEmptyString(historyDataString(event, "error"), historyDataString(event, "reason"), event.Message, "Release was blocklisted")
	protocol := firstNonEmptyString(historyDataString(event, "protocol"), "torrent")
	recordID := stableInt("history:" + firstNonEmptyString(event.ID, event.EventType+event.CreatedAt.Format(time.RFC3339Nano)))
	return map[string]any{
		"id":             recordID,
		"authorId":       stableInt(authorName),
		"bookId":         blocklistBookID(title, item),
		"sourceTitle":    firstNonEmptyString(historyDataString(event, "releaseTitle"), historyDataString(event, "title"), title),
		"quality":        compatReleaseQuality(blocklistFormat(historyDataString(event, "releaseTitle"), historyDataString(event, "format"), item)),
		"languages":      []map[string]any{{"id": 1, "name": "English"}},
		"date":           event.CreatedAt,
		"protocol":       protocol,
		"indexer":        historyDataString(event, "indexer"),
		"message":        reason,
		"downloadId":     historyDataString(event, "downloadId"),
		"releaseId":      historyDataString(event, "releaseId"),
		"author":         compatBlocklistAuthorRecord(authorName),
		"book":           compatBlocklistBookRecord(title, authorName, item),
		"librarrySource": "history",
		"librarryEvent":  event.EventType,
		"librarryReason": reason,
	}
}

func compatBlocklistAuthorRecord(authorName string) map[string]any {
	return map[string]any{
		"id":         stableInt(authorName),
		"authorName": authorName,
		"titleSlug":  slug(authorName),
	}
}

func compatBlocklistBookRecord(title string, authorName string, item *wanted.WantedItem) map[string]any {
	if item != nil {
		return compatBookRecord(*item)
	}
	return map[string]any{
		"id":          stableInt(title),
		"title":       title,
		"authorTitle": authorName,
		"titleSlug":   slug(title),
	}
}

func compatMissingRecord(item wanted.WantedItem) map[string]any {
	authorID := stableInt(item.AuthorName)
	bookID := stableInt(item.ID)
	return map[string]any{
		"id":             bookID,
		"librarryId":     item.ID,
		"title":          item.Title,
		"monitored":      true,
		"anyEditionOk":   true,
		"releaseDate":    item.CreatedAt,
		"statistics":     map[string]any{"bookFileCount": 0},
		"qualityProfile": item.QualityProfile,
		"author": map[string]any{
			"id":         authorID,
			"authorName": item.AuthorName,
			"titleSlug":  slug(item.AuthorName),
			"monitored":  true,
			"path":       "",
		},
	}
}

func compatAuthorRecord(subscription wanted.AuthorSubscription, books []wanted.WantedItem) map[string]any {
	id := stableInt(firstNonEmptyString(subscription.ID, subscription.ProviderKey, subscription.AuthorName))
	bookRecords := make([]map[string]any, 0, len(books))
	for _, book := range books {
		bookRecords = append(bookRecords, compatAuthorBookRecord(book))
	}
	return map[string]any{
		"id":                  id,
		"authorMetadataId":    id,
		"authorName":          subscription.AuthorName,
		"sortName":            strings.ToLower(subscription.AuthorName),
		"cleanName":           slug(subscription.AuthorName),
		"titleSlug":           slug(subscription.AuthorName),
		"foreignAuthorId":     firstNonEmptyString(subscription.ProviderKey, subscription.ID),
		"monitored":           subscription.Status != "unmonitored",
		"rootFolderPath":      "",
		"path":                "",
		"qualityProfileId":    stableInt(subscription.QualityProfile),
		"metadataProfileId":   stableInt(subscription.Format),
		"tags":                []int{},
		"genres":              []string{},
		"ratings":             map[string]any{"votes": 0, "value": 0},
		"statistics":          map[string]any{"bookCount": len(bookRecords), "bookFileCount": 0},
		"lastInfoSync":        subscription.LastSyncAt,
		"added":               subscription.CreatedAt,
		"books":               bookRecords,
		"addOptions":          map[string]any{"monitor": "all", "searchForMissingBooks": false},
		"librarryProvider":    subscription.Provider,
		"librarryFormat":      subscription.Format,
		"librarryAuthorId":    subscription.ID,
		"librarryQualityName": subscription.QualityProfile,
	}
}

func compatAuthorBookRecord(item wanted.WantedItem) map[string]any {
	return map[string]any{
		"id":             stableInt(item.ID),
		"title":          item.Title,
		"foreignBookId":  firstNonEmptyString(item.WorkID, item.SourceKey, item.ID),
		"authorTitle":    item.AuthorName,
		"authorId":       stableInt(item.AuthorName),
		"monitored":      item.Status != "ignored",
		"anyEditionOk":   true,
		"qualityProfile": item.QualityProfile,
		"releaseDate":    item.CreatedAt,
		"statistics":     map[string]any{"bookFileCount": boolInt(item.Status == "imported")},
	}
}

func compatBookRecord(item wanted.WantedItem) map[string]any {
	return map[string]any{
		"id":               stableInt(item.ID),
		"librarryId":       item.ID,
		"authorId":         stableInt(item.AuthorName),
		"authorTitle":      item.AuthorName,
		"title":            item.Title,
		"titleSlug":        slug(item.Title),
		"foreignBookId":    firstNonEmptyString(item.WorkID, item.SourceKey, item.ID),
		"monitored":        item.Status != "ignored",
		"anyEditionOk":     true,
		"releaseDate":      item.CreatedAt,
		"qualityProfile":   item.QualityProfile,
		"qualityProfileId": stableInt(item.QualityProfile),
		"statistics": map[string]any{
			"bookFileCount": boolInt(item.Status == "imported"),
			"sizeOnDisk":    0,
		},
		"images": compatImages(item.CoverURL),
		"author": map[string]any{
			"id":         stableInt(item.AuthorName),
			"authorName": item.AuthorName,
			"titleSlug":  slug(item.AuthorName),
			"monitored":  true,
		},
		"addOptions":     map[string]any{"searchForNewBook": false},
		"librarryStatus": item.Status,
		"librarryFormat": item.Format,
	}
}

func compatAuthorLookupRecord(result metadata.SearchResult) map[string]any {
	author := firstAuthor(result)
	return map[string]any{
		"id":                stableInt(firstNonEmptyString(author.ID, author.Name, result.Work.ID)),
		"authorName":        author.Name,
		"sortName":          strings.ToLower(author.Name),
		"cleanName":         slug(author.Name),
		"titleSlug":         slug(author.Name),
		"foreignAuthorId":   firstNonEmptyString(author.ID, result.Work.ID, "lookup:"+slug(author.Name)),
		"monitored":         false,
		"images":            compatImages(result.Work.CoverURL),
		"statistics":        map[string]any{"bookCount": 0, "bookFileCount": 0},
		"ratings":           map[string]any{"votes": 0, "value": result.Score},
		"books":             []map[string]any{},
		"librarryProvider":  result.Provider,
		"librarryMatchedOn": result.MatchedOn,
	}
}

func compatBookLookupRecord(result metadata.SearchResult) map[string]any {
	author := firstAuthor(result)
	return map[string]any{
		"id":            stableInt(firstNonEmptyString(result.Work.ID, result.Edition.ID, result.Work.Title)),
		"title":         result.Work.Title,
		"titleSlug":     slug(result.Work.Title),
		"foreignBookId": firstNonEmptyString(result.Work.ID, result.Edition.ID, "lookup:"+slug(result.Work.Title)),
		"authorTitle":   author.Name,
		"authorId":      stableInt(author.Name),
		"monitored":     false,
		"anyEditionOk":  true,
		"releaseDate":   dateFromYear(result.Work.FirstPublishYear),
		"overview":      result.Work.Description,
		"images":        compatImages(result.Work.CoverURL),
		"statistics":    map[string]any{"bookFileCount": 0, "sizeOnDisk": 0},
		"author": map[string]any{
			"id":              stableInt(firstNonEmptyString(author.ID, author.Name)),
			"authorName":      author.Name,
			"foreignAuthorId": author.ID,
			"titleSlug":       slug(author.Name),
			"monitored":       false,
		},
		"editions": []map[string]any{{
			"id":               stableInt(firstNonEmptyString(result.Edition.ID, result.Work.ID, result.Work.Title)),
			"foreignEditionId": result.Edition.ID,
			"title":            firstNonEmptyString(result.Edition.Title, result.Work.Title),
			"format":           result.Edition.Format,
			"isbn13":           firstString(result.Edition.ISBNs),
			"monitored":        false,
			"manualAdd":        false,
		}},
		"librarryProvider":   result.Provider,
		"librarryConfidence": result.Confidence,
		"librarryMatchedOn":  result.MatchedOn,
	}
}

func compatImages(coverURL string) []map[string]any {
	if strings.TrimSpace(coverURL) == "" {
		return []map[string]any{}
	}
	return []map[string]any{{"coverType": "cover", "url": coverURL, "remoteUrl": coverURL}}
}

func compatQualityProfileRecord(id int, profile wanted.QualityProfile) map[string]any {
	return map[string]any{
		"id":                id,
		"name":              profile.Name,
		"upgradeAllowed":    profile.UpgradeAllowed,
		"cutoff":            0,
		"minFormatScore":    profile.MinScore,
		"cutoffFormatScore": profile.CutoffScore,
		"language": map[string]any{
			"id":   1,
			"name": "English",
		},
		"items": []map[string]any{{
			"quality": map[string]any{
				"id":   id,
				"name": profile.MediaFormat,
			},
			"allowed": true,
		}},
		"formatItems": []map[string]any{},
		"librarry":    profile,
	}
}

func compatDelayProfileRecord(payload map[string]any, id int) map[string]any {
	if id <= 0 {
		id = stablePayloadID(payload, "delay-profile")
	}
	return map[string]any{
		"id":                             id,
		"preferredProtocol":              firstNonEmptyString(payloadString(payload, "preferredProtocol"), "torrent"),
		"usenetDelay":                    payloadIntDefault(payload, "usenetDelay", 0),
		"torrentDelay":                   payloadIntDefault(payload, "torrentDelay", 0),
		"bypassIfHighestQuality":         payloadBoolDefault(payload, "bypassIfHighestQuality", true),
		"bypassIfAboveCustomFormatScore": payloadBoolDefault(payload, "bypassIfAboveCustomFormatScore", false),
		"minimumCustomFormatScore":       payloadIntDefault(payload, "minimumCustomFormatScore", 0),
		"order":                          payloadIntDefault(payload, "order", 1),
		"tags":                           compatPayloadIntArray(payload, "tags"),
		"librarryEphemeral":              true,
	}
}

func compatQualityDefinitionRecords() []map[string]any {
	qualities := []struct {
		id            int
		name          string
		weight        int
		maxSize       int
		preferredSize int
	}{
		{1, "Unknown", 0, 0, 0},
		{2, "EPUB", 100, 750, 250},
		{3, "MOBI", 90, 750, 250},
		{4, "AZW3", 95, 750, 250},
		{5, "PDF", 60, 2000, 500},
		{6, "MP3", 80, 8192, 2048},
		{7, "M4B", 100, 8192, 2048},
		{8, "Audiobook", 90, 20000, 4096},
	}
	records := make([]map[string]any, 0, len(qualities))
	for _, quality := range qualities {
		records = append(records, map[string]any{
			"id":            quality.id,
			"quality":       map[string]any{"id": quality.id, "name": quality.name, "source": "librarry"},
			"title":         quality.name,
			"weight":        quality.weight,
			"minSize":       0,
			"maxSize":       quality.maxSize,
			"preferredSize": quality.preferredSize,
		})
	}
	return records
}

func compatQualityDefinitionRecordFromPayload(id int, payload map[string]any) map[string]any {
	if id <= 0 {
		id = stablePayloadID(payload, "quality-definition")
	}
	name := firstNonEmptyString(nestedString(payload, "quality", "name"), payloadString(payload, "title"), payloadString(payload, "name"), "Unknown")
	return map[string]any{
		"id":            id,
		"quality":       map[string]any{"id": id, "name": name, "source": "librarry"},
		"title":         name,
		"weight":        payloadIntDefault(payload, "weight", id*10),
		"minSize":       payloadIntDefault(payload, "minSize", 0),
		"maxSize":       payloadIntDefault(payload, "maxSize", 0),
		"preferredSize": payloadIntDefault(payload, "preferredSize", 0),
	}
}

func compatLanguageProfileRecord(payload map[string]any, id int) map[string]any {
	if id <= 0 {
		id = 1
	}
	name := firstNonEmptyString(payloadString(payload, "name"), "English")
	return map[string]any{
		"id":             id,
		"name":           name,
		"upgradeAllowed": payloadBoolDefault(payload, "upgradeAllowed", true),
		"cutoff":         map[string]any{"id": 1, "name": "English"},
		"cutoffLanguage": map[string]any{"id": 1, "name": "English"},
		"languages": []map[string]any{{
			"language": map[string]any{"id": 1, "name": "English"},
			"allowed":  true,
		}},
		"tags":              []int{},
		"librarryEphemeral": true,
	}
}

func compatMetadataProfileRecord(payload map[string]any, id int) map[string]any {
	if id <= 0 {
		id = 1
	}
	return map[string]any{
		"id":                id,
		"name":              firstNonEmptyString(payloadString(payload, "name"), "Standard"),
		"minPopularity":     payloadIntDefault(payload, "minPopularity", 0),
		"skipMissingDate":   payloadBoolDefault(payload, "skipMissingDate", false),
		"skipMissingIsbn":   payloadBoolDefault(payload, "skipMissingIsbn", false),
		"skipPartsAndSets":  payloadBoolDefault(payload, "skipPartsAndSets", false),
		"tags":              []int{},
		"librarryEphemeral": true,
	}
}

func compatCustomFormatRecord(payload map[string]any, id int) map[string]any {
	if id <= 0 {
		id = stablePayloadID(payload, "custom-format")
	}
	return map[string]any{
		"id":                              id,
		"name":                            firstNonEmptyString(payloadString(payload, "name"), "Librarry Custom Format"),
		"includeCustomFormatWhenRenaming": payloadBoolDefault(payload, "includeCustomFormatWhenRenaming", false),
		"specifications":                  compatPayloadArray(payload, "specifications"),
		"librarryEphemeral":               true,
	}
}

func compatTagRecord(payload map[string]any, id int) map[string]any {
	label := firstNonEmptyString(payloadString(payload, "label"), payloadString(payload, "name"), "librarry")
	if id <= 0 {
		id = stableInt(label)
	}
	return map[string]any{
		"id":    id,
		"label": label,
	}
}

func compatRestrictionRecord(payload map[string]any, id int) map[string]any {
	if id <= 0 {
		id = stablePayloadID(payload, "restriction")
	}
	return map[string]any{
		"id":                id,
		"required":          payloadString(payload, "required"),
		"ignored":           firstNonEmptyString(payloadString(payload, "ignored"), payloadString(payload, "mustNotContain")),
		"tags":              compatPayloadIntArray(payload, "tags"),
		"librarryEphemeral": true,
	}
}

func compatNotificationRecord(payload map[string]any, id int) map[string]any {
	if id <= 0 {
		id = stablePayloadID(payload, "notification")
	}
	implementation := firstNonEmptyString(payloadString(payload, "implementation"), "Webhook")
	return map[string]any{
		"id":                 id,
		"name":               firstNonEmptyString(payloadString(payload, "name"), implementation),
		"implementation":     implementation,
		"implementationName": firstNonEmptyString(payloadString(payload, "implementationName"), implementation),
		"configContract":     firstNonEmptyString(payloadString(payload, "configContract"), implementation+"Settings"),
		"enable":             payloadBoolDefault(payload, "enable", false),
		"onGrab":             payloadBoolDefault(payload, "onGrab", true),
		"onReleaseImport":    payloadBoolDefault(payload, "onReleaseImport", true),
		"onUpgrade":          payloadBoolDefault(payload, "onUpgrade", true),
		"supportsOnGrab":     true,
		"supportsOnDownload": true,
		"fields":             compatPayloadArray(payload, "fields"),
		"tags":               compatPayloadIntArray(payload, "tags"),
		"librarryEphemeral":  true,
	}
}

func compatImportListRecord(payload map[string]any, id int) map[string]any {
	if id <= 0 {
		id = stablePayloadID(payload, "import-list")
	}
	implementation := firstNonEmptyString(payloadString(payload, "implementation"), "ReadarrImportList")
	return map[string]any{
		"id":                 id,
		"name":               firstNonEmptyString(payloadString(payload, "name"), implementation),
		"implementation":     implementation,
		"implementationName": firstNonEmptyString(payloadString(payload, "implementationName"), implementation),
		"configContract":     firstNonEmptyString(payloadString(payload, "configContract"), implementation+"Settings"),
		"enable":             payloadBoolDefault(payload, "enable", false),
		"rootFolderPath":     payloadString(payload, "rootFolderPath"),
		"qualityProfileId":   payloadIntDefault(payload, "qualityProfileId", 1),
		"metadataProfileId":  payloadIntDefault(payload, "metadataProfileId", 1),
		"tags":               compatPayloadIntArray(payload, "tags"),
		"fields":             compatPayloadArray(payload, "fields"),
		"librarryEphemeral":  true,
	}
}

func compatRemotePathMappingRecord(payload map[string]any, id int) map[string]any {
	if id <= 0 {
		id = stablePayloadID(payload, "remote-path-mapping")
	}
	return map[string]any{
		"id":                id,
		"host":              payloadString(payload, "host"),
		"remotePath":        payloadString(payload, "remotePath"),
		"localPath":         payloadString(payload, "localPath"),
		"librarryEphemeral": true,
	}
}

func compatSystemTaskRecord(id int, name string, description string, interval time.Duration, enabled bool, now time.Time) map[string]any {
	intervalMinutes := durationMinutes(interval, 0)
	lastExecution := now.Add(-time.Duration(intervalMinutes) * time.Minute)
	nextExecution := now.Add(time.Duration(intervalMinutes) * time.Minute)
	if intervalMinutes <= 0 {
		lastExecution = time.Time{}
		nextExecution = time.Time{}
	}
	return map[string]any{
		"id":            id,
		"name":          name,
		"taskName":      name,
		"interval":      intervalMinutes,
		"lastExecution": nullableTaskTime(lastExecution),
		"nextExecution": nullableTaskTime(nextExecution),
		"lastStartTime": nullableTaskTime(lastExecution),
		"lastDuration":  "00:00:00",
		"queued":        false,
		"started":       false,
		"enabled":       enabled,
		"description":   description,
	}
}

func compatDownloadClientRecord(id int, name string, protocol string, baseURL string, category string) map[string]any {
	return map[string]any{
		"id":                 id,
		"name":               name,
		"implementation":     name,
		"implementationName": name,
		"protocol":           protocol,
		"enable":             true,
		"priority":           1,
		"fields": []map[string]any{
			{"name": "host", "value": baseURL},
			{"name": "category", "value": category},
		},
	}
}

func queueTrackedStatus(download acquisition.DownloadStatus) string {
	switch stateTone(download.State) {
	case "error":
		return "warning"
	default:
		return "ok"
	}
}

func queueTrackedState(download acquisition.DownloadStatus) string {
	if download.Progress >= 1 {
		return "importPending"
	}
	return "downloading"
}

func queueStatusMessages(download acquisition.DownloadStatus) []map[string]any {
	var messages []map[string]any
	if strings.TrimSpace(download.FailureReason) != "" {
		messages = append(messages, map[string]any{
			"title":    download.FailureReason,
			"messages": []string{download.FailureReason},
		})
	}
	if strings.TrimSpace(download.ImportError) != "" {
		messages = append(messages, map[string]any{
			"title":    download.ImportError,
			"messages": []string{download.ImportError},
		})
	}
	return messages
}

func queueProtocol(download acquisition.DownloadStatus) string {
	if strings.EqualFold(download.Client, "SABnzbd") {
		return "usenet"
	}
	return "torrent"
}

func downloadIsBlocklisted(download acquisition.DownloadStatus) bool {
	return stateTone(download.State) == "error" ||
		strings.TrimSpace(download.FailureReason) != "" ||
		strings.TrimSpace(download.ImportError) != ""
}

func historyEventIsBlocklisted(event wanted.HistoryEvent) bool {
	eventType := strings.ToLower(strings.TrimSpace(event.EventType))
	return strings.Contains(eventType, "failed") || compatHistoryEventType(event.EventType) == "downloadFailed"
}

func wantedIDFromDownload(download acquisition.DownloadStatus) string {
	for _, tag := range download.Tags {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, "wanted:") {
			return strings.TrimPrefix(tag, "wanted:")
		}
	}
	return ""
}

func wantedItemByID(items []wanted.WantedItem, id string) *wanted.WantedItem {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	for _, item := range items {
		if compatIDMatches(id, item.ID, item.WorkID, item.SourceKey, item.Title) {
			matched := item
			return &matched
		}
	}
	return nil
}

func titleAuthorForDownload(download acquisition.DownloadStatus, item *wanted.WantedItem) (string, string) {
	if item != nil {
		return item.Title, item.AuthorName
	}
	title, author := parseCompatReleaseTitle(download.Name)
	return firstNonEmptyString(title, download.Name, download.ID), author
}

func blocklistHistoryTitle(event wanted.HistoryEvent, item *wanted.WantedItem) string {
	if item != nil {
		return item.Title
	}
	return historyTitle(event)
}

func blocklistHistoryAuthor(event wanted.HistoryEvent, item *wanted.WantedItem) string {
	if item != nil {
		return item.AuthorName
	}
	return historyAuthor(event)
}

func blocklistBookID(title string, item *wanted.WantedItem) int {
	if item != nil {
		return stableInt(item.ID)
	}
	return stableInt(title)
}

func blocklistFormat(hints ...any) string {
	for _, hint := range hints {
		switch typed := hint.(type) {
		case *wanted.WantedItem:
			if typed != nil && strings.TrimSpace(typed.Format) != "" {
				return typed.Format
			}
		case string:
			normalized := strings.ToLower(strings.TrimSpace(typed))
			for _, token := range []string{"audiobook", "audio book", "m4b", "mp3", "books-audiobook"} {
				if strings.Contains(normalized, token) {
					return "audiobook"
				}
			}
		}
	}
	return "ebook"
}

func firstNonNilTime(values ...*time.Time) *time.Time {
	for _, value := range values {
		if value != nil && !value.IsZero() {
			normalized := value.UTC()
			return &normalized
		}
	}
	return nil
}

func compatRecordDate(record map[string]any) time.Time {
	switch typed := record["date"].(type) {
	case time.Time:
		return typed
	case *time.Time:
		if typed != nil {
			return *typed
		}
	case string:
		return parseTimeQuery(typed)
	}
	return time.Time{}
}

func estimatedCompletion(download acquisition.DownloadStatus) any {
	if download.ETASeconds <= 0 || download.Progress >= 1 {
		return nil
	}
	return time.Now().UTC().Add(time.Duration(download.ETASeconds) * time.Second)
}

func stateTone(state string) string {
	normalized := strings.ToLower(strings.TrimSpace(state))
	switch {
	case strings.Contains(normalized, "error"), strings.Contains(normalized, "fail"), strings.Contains(normalized, "missing"):
		return "error"
	case strings.Contains(normalized, "warn"), strings.Contains(normalized, "stall"):
		return "warning"
	case normalized == "":
		return "unknown"
	default:
		return "ok"
	}
}

func pageParams(r *http.Request, total int) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize <= 0 {
		pageSize = total
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return page, pageSize
}

func pageRecords(records []map[string]any, page int, pageSize int) []map[string]any {
	start := (page - 1) * pageSize
	if start >= len(records) {
		return []map[string]any{}
	}
	end := start + pageSize
	if end > len(records) {
		end = len(records)
	}
	return records[start:end]
}

func statPath(path string) (int64, int64, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, false
	}
	blockSize := int64(stat.Bsize)
	return int64(stat.Blocks) * blockSize, int64(stat.Bavail) * blockSize, true
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func databaseType(databaseURL string) string {
	if strings.TrimSpace(databaseURL) == "" {
		return "none"
	}
	return "postgres"
}

func maskedValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "********"
}

func compatListenPort(listenAddr string, fallback int) int {
	value := strings.TrimSpace(listenAddr)
	if value == "" {
		return fallback
	}
	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		value = parts[len(parts)-1]
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func durationMinutes(value time.Duration, fallback int) int {
	if value <= 0 {
		return fallback
	}
	minutes := int(value.Minutes())
	if minutes <= 0 {
		return fallback
	}
	return minutes
}

func nullableTaskTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func parseBoolDefault(value string, fallback bool) bool {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func stableInt(value string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.TrimSpace(value)))
	return int(hash.Sum32() & 0x7fffffff)
}

func compatIDMatches(pathValue string, candidates ...string) bool {
	pathValue = strings.TrimSpace(pathValue)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if pathValue == candidate || pathValue == strconv.Itoa(stableInt(candidate)) {
			return true
		}
	}
	return false
}

func booksForAuthor(items []wanted.WantedItem, authorName string) []wanted.WantedItem {
	var matches []wanted.WantedItem
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.AuthorName), strings.TrimSpace(authorName)) {
			matches = append(matches, item)
		}
	}
	return matches
}

func firstAuthor(result metadata.SearchResult) metadata.Author {
	if len(result.Work.Authors) > 0 {
		return result.Work.Authors[0]
	}
	return metadata.Author{Name: ""}
}

func payloadString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(jsonString(typed)), `"`), `"`))
	}
}

func payloadQualityName(payload map[string]any) string {
	qualityName := nestedString(payload, "quality", "name")
	if qualityName != "" {
		return qualityName
	}
	quality, ok := payload["quality"].(map[string]any)
	if !ok {
		return ""
	}
	return firstNonEmptyString(nestedString(quality, "quality", "name"), payloadString(quality, "name"))
}

func nestedString(payload map[string]any, objectKey string, key string) string {
	object, ok := payload[objectKey].(map[string]any)
	if !ok {
		return ""
	}
	return payloadString(object, key)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func payloadBoolDefault(payload map[string]any, key string, fallback bool) bool {
	if payload == nil {
		return fallback
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	case float64:
		return typed != 0
	}
	return fallback
}

func payloadIntDefault(payload map[string]any, key string, fallback int) int {
	if payload == nil {
		return fallback
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func decodeCompatObjectPayload(w http.ResponseWriter, r *http.Request, name string) (map[string]any, bool) {
	defer r.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid " + name + " payload"})
		return nil, false
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, true
}

func pathValueInt(r *http.Request, key string) int {
	value := strings.TrimSpace(r.PathValue(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err == nil {
		return parsed
	}
	return stableInt(value)
}

func stablePayloadID(payload map[string]any, fallback string) int {
	id := payloadIntDefault(payload, "id", 0)
	if id > 0 {
		return id
	}
	return stableInt(firstNonEmptyString(payloadString(payload, "name"), payloadString(payload, "label"), fallback))
}

func compatPayloadArray(payload map[string]any, key string) []any {
	if payload == nil {
		return []any{}
	}
	if values, ok := payload[key].([]any); ok {
		return values
	}
	return []any{}
}

func compatPayloadIntArray(payload map[string]any, key string) []int {
	if payload == nil {
		return []int{}
	}
	values, ok := payload[key].([]any)
	if !ok {
		return []int{}
	}
	ids := make([]int, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case float64:
			ids = append(ids, int(typed))
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				ids = append(ids, parsed)
			}
		}
	}
	return ids
}

func manualImportPayloads(payload any) []map[string]any {
	switch typed := payload.(type) {
	case []any:
		return mapsFromValues(typed)
	case map[string]any:
		for _, key := range []string{"items", "files", "manualImports"} {
			if values, ok := typed[key].([]any); ok {
				return mapsFromValues(values)
			}
		}
		return []map[string]any{typed}
	default:
		return nil
	}
}

func mapsFromValues(values []any) []map[string]any {
	records := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if record, ok := value.(map[string]any); ok {
			records = append(records, record)
		}
	}
	return records
}

func manualImportMove(payload map[string]any) bool {
	mode := strings.ToLower(strings.TrimSpace(firstNonEmptyString(payloadString(payload, "importMode"), payloadString(payload, "mode"))))
	if mode == "move" {
		return true
	}
	if mode == "copy" {
		return false
	}
	return payloadBoolDefault(payload, "move", false)
}

func renderCompatNamingExample(record map[string]any, author string, title string, format string, ext string) string {
	values := map[string]string{
		"Author": author,
		"Title":  title,
		"Format": format,
		"Ext":    ext,
	}
	spaceReplacement := payloadString(record, "replaceSpacesWith")
	parts := []string{
		renderCompatTemplate(payloadString(record, "authorFolderFormat"), values, spaceReplacement),
		renderCompatTemplate(payloadString(record, "bookFolderFormat"), values, spaceReplacement),
		renderCompatTemplate(payloadString(record, "standardBookFormat"), values, spaceReplacement),
	}
	return strings.Join(parts, "/")
}

func renderCompatTemplate(template string, values map[string]string, spaceReplacement string) string {
	template = defaultString(template, "{Title}")
	for key, value := range values {
		template = strings.ReplaceAll(template, "{"+key+"}", value)
		template = strings.ReplaceAll(template, "{"+strings.ToLower(key)+"}", value)
	}
	if spaceReplacement != "" {
		template = strings.ReplaceAll(template, " ", spaceReplacement)
	}
	return template
}

func compatHistoryEventType(eventType string) string {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "wanted_grabbed", "feed_grabbed", "upgrade_grabbed":
		return "grabbed"
	case "wanted_imported", "download_imported", "manual_imported":
		return "downloadFolderImported"
	case "download_failed", "wanted_grab_failed", "feed_grab_failed":
		return "downloadFailed"
	case "wanted_searched", "feed_sync", "upgrade_found":
		return "bookSearch"
	default:
		return "unknown"
	}
}

func historyTitle(event wanted.HistoryEvent) string {
	return firstNonEmptyString(
		historyDataString(event, "title"),
		historyDataString(event, "bookTitle"),
		historyDataString(event, "releaseTitle"),
		event.Message,
		event.EntityID,
		"Unknown Book",
	)
}

func historyAuthor(event wanted.HistoryEvent) string {
	return firstNonEmptyString(
		historyDataString(event, "authorName"),
		historyDataString(event, "author"),
		"Unknown Author",
	)
}

func historyData(event wanted.HistoryEvent) map[string]any {
	data := map[string]any{}
	for key, value := range event.Data {
		data[key] = value
	}
	data["message"] = event.Message
	data["severity"] = event.Severity
	data["entityType"] = event.EntityType
	data["entityId"] = event.EntityID
	return data
}

func historyDataString(event wanted.HistoryEvent, key string) string {
	if event.Data == nil {
		return ""
	}
	value, ok := event.Data[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(jsonString(typed), `"`), `"`))
	}
}

func parseCompatReleaseTitle(value string) (string, string) {
	value = strings.TrimSpace(value)
	for _, separator := range []string{" - ", " -- ", " by "} {
		parts := strings.SplitN(value, separator, 2)
		if len(parts) != 2 {
			continue
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if left == "" || right == "" {
			continue
		}
		if separator == " by " {
			return left, right
		}
		return right, left
	}
	return strings.TrimSuffix(value, filepath.Ext(value)), ""
}

func parseTimeQuery(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func timeInRange(value time.Time, start time.Time, end time.Time) bool {
	if !start.IsZero() && value.Before(start) {
		return false
	}
	if !end.IsZero() && value.After(end) {
		return false
	}
	return true
}

func mapsByIntKey(values map[int]map[string]any) []map[string]any {
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		records = append(records, values[key])
	}
	return records
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func dateFromYear(year int) any {
	if year <= 0 {
		return nil
	}
	return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
}

func jsonString(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, value)
	return strings.Trim(value, "-")
}
