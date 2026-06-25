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
		{"method": "GET", "path": "/api/v1/wanted/missing"},
		{"method": "GET", "path": "/api/v1/qualityprofile"},
		{"method": "GET", "path": "/api/v1/downloadclient"},
		{"method": "GET", "path": "/api/v1/indexer"},
		{"method": "GET", "path": "/api/v1/release"},
		{"method": "POST", "path": "/api/v1/release"},
		{"method": "GET", "path": "/api/v1/manualimport"},
		{"method": "POST", "path": "/api/v1/manualimport"},
		{"method": "GET", "path": "/api/v1/command"},
		{"method": "POST", "path": "/api/v1/command"},
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
	var payload struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	name := defaultString(payload.Name, "Unknown")
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
