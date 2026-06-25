package api

import (
	"encoding/json"
	"hash/fnv"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/library"
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
		{"method": "GET", "path": "/api/v1/rootfolder"},
		{"method": "GET", "path": "/api/v1/queue"},
		{"method": "GET", "path": "/api/v1/queue/status"},
		{"method": "GET", "path": "/api/v1/wanted/missing"},
		{"method": "GET", "path": "/api/v1/qualityprofile"},
		{"method": "GET", "path": "/api/v1/downloadclient"},
		{"method": "GET", "path": "/api/v1/indexer"},
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

func (h *handler) compatRootFolderRecords() []map[string]any {
	return []map[string]any{
		compatRootFolderRecord(1, "Ebooks", defaultString(h.deps.Config.EbookLibraryRoot, "/data/media/books/ebooks")),
		compatRootFolderRecord(2, "Audiobooks", defaultString(h.deps.Config.AudiobookLibraryRoot, "/data/media/books/audiobooks")),
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
