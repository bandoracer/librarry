package acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type QBittorrentClient struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

func NewQBittorrentClient(baseURL string, username string, password string, client *http.Client) *QBittorrentClient {
	if client == nil {
		jar, _ := cookiejar.New(nil)
		client = &http.Client{Timeout: 30 * time.Second, Jar: jar}
	}
	return &QBittorrentClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: strings.TrimSpace(username),
		password: strings.TrimSpace(password),
		client:   client,
	}
}

func (c *QBittorrentClient) Name() string { return "qBittorrent" }

func (c *QBittorrentClient) Configured() bool {
	return c.baseURL != ""
}

func (c *QBittorrentClient) Health(ctx context.Context) IntegrationHealth {
	if !c.Configured() {
		return IntegrationHealth{Name: c.Name(), Configured: false, Status: "missing_credentials", Message: "Set LIBRARRY_QBITTORRENT_URL."}
	}
	if err := c.login(ctx); err != nil {
		return IntegrationHealth{Name: c.Name(), Configured: true, Status: "error", Message: err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/transfer/info", nil)
	if err != nil {
		return IntegrationHealth{Name: c.Name(), Configured: true, Status: "error", Message: err.Error()}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return IntegrationHealth{Name: c.Name(), Configured: true, Status: "error", Message: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return IntegrationHealth{Name: c.Name(), Configured: true, Status: "error", Message: resp.Status}
	}
	var decoded struct {
		ConnectionStatus string `json:"connection_status"`
		DHTNodes         int    `json:"dht_nodes"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&decoded)
	message := "Ready"
	if decoded.ConnectionStatus != "" {
		message = fmt.Sprintf("VPN %s; %d DHT nodes", decoded.ConnectionStatus, decoded.DHTNodes)
	}
	return IntegrationHealth{Name: c.Name(), Configured: true, Status: "ready", Message: message}
}

func (c *QBittorrentClient) EnsureCategory(ctx context.Context, category string, savePath string) error {
	if !c.Configured() {
		return ErrIntegrationNotConfigured
	}
	if strings.TrimSpace(category) == "" {
		return errors.New("category is required")
	}
	if err := c.login(ctx); err != nil {
		return err
	}
	values := url.Values{}
	values.Set("category", category)
	values.Set("savePath", savePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/createCategory", strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusBadRequest {
		return c.editCategory(ctx, category, savePath)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qBittorrent create category returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *QBittorrentClient) Add(ctx context.Context, request DownloadRequest) (DownloadStatus, error) {
	if !c.Configured() {
		return DownloadStatus{}, ErrIntegrationNotConfigured
	}
	if strings.TrimSpace(request.ReleaseURL) == "" && len(request.UploadData) == 0 {
		return DownloadStatus{}, errors.New("releaseUrl or torrent upload is required")
	}
	if err := c.login(ctx); err != nil {
		return DownloadStatus{}, err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if strings.TrimSpace(request.ReleaseURL) != "" {
		_ = writer.WriteField("urls", request.ReleaseURL)
	}
	if len(request.UploadData) > 0 {
		part, err := writer.CreateFormFile("torrents", firstNonEmpty(request.UploadName, "upload.torrent"))
		if err != nil {
			return DownloadStatus{}, err
		}
		if _, err := part.Write(request.UploadData); err != nil {
			return DownloadStatus{}, err
		}
	}
	_ = writer.WriteField("category", request.Category)
	_ = writer.WriteField("savepath", request.SavePath)
	_ = writer.WriteField("paused", boolString(request.Paused))
	_ = writer.WriteField("stopped", boolString(request.Paused))
	if len(request.Tags) > 0 {
		_ = writer.WriteField("tags", strings.Join(request.Tags, ","))
	}
	_ = writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/add", &body)
	if err != nil {
		return DownloadStatus{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.client.Do(req)
	if err != nil {
		return DownloadStatus{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return DownloadStatus{}, fmt.Errorf("qBittorrent add returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	state := "queued"
	id := firstNonEmpty(request.InfoHash, releaseID(request.ReleaseURL))
	if summary, ok := parseAddSummary(respBody); ok {
		if summary.FailureCount > 0 && summary.SuccessCount == 0 && summary.PendingCount == 0 {
			return DownloadStatus{}, fmt.Errorf("qBittorrent add failed: %s", strings.TrimSpace(string(respBody)))
		}
		if summary.PendingCount > 0 {
			state = "pending"
		}
		id = firstNonEmpty(request.InfoHash, firstString(summary.AddedTorrentIDs), releaseID(request.ReleaseURL))
	} else if !strings.Contains(strings.ToLower(string(respBody)), "ok") {
		return DownloadStatus{}, fmt.Errorf("qBittorrent add returned unexpected body: %s", strings.TrimSpace(string(respBody)))
	}
	now := time.Now().UTC()
	return DownloadStatus{
		Client:     c.Name(),
		ID:         firstNonEmpty(id, request.UploadName),
		Name:       firstNonEmpty(request.Title, request.UploadName, request.ReleaseURL),
		State:      state,
		Progress:   0,
		SavePath:   request.SavePath,
		Category:   request.Category,
		Tags:       request.Tags,
		LastSeenAt: &now,
	}, nil
}

type addSummary struct {
	AddedTorrentIDs []string `json:"added_torrent_ids"`
	FailureCount    int      `json:"failure_count"`
	PendingCount    int      `json:"pending_count"`
	SuccessCount    int      `json:"success_count"`
}

func parseAddSummary(body []byte) (addSummary, bool) {
	var summary addSummary
	if err := json.Unmarshal(body, &summary); err != nil {
		return addSummary{}, false
	}
	if summary.FailureCount == 0 && summary.PendingCount == 0 && summary.SuccessCount == 0 && len(summary.AddedTorrentIDs) == 0 {
		return summary, false
	}
	return summary, true
}

func (c *QBittorrentClient) List(ctx context.Context, query DownloadListQuery) ([]DownloadStatus, error) {
	if !c.Configured() {
		return nil, ErrIntegrationNotConfigured
	}
	if err := c.login(ctx); err != nil {
		return nil, err
	}
	values := url.Values{}
	if strings.TrimSpace(query.Tag) != "" {
		values.Set("tag", query.Tag)
	}
	if strings.TrimSpace(query.Category) != "" {
		values.Set("category", query.Category)
	}
	if len(query.IDs) > 0 {
		values.Set("hashes", strings.Join(compactStrings(query.IDs), "|"))
	}
	endpoint := c.baseURL + "/api/v2/torrents/info"
	if encoded := values.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("qBittorrent list returned %s", resp.Status)
	}
	var raw []struct {
		Hash         string  `json:"hash"`
		Name         string  `json:"name"`
		State        string  `json:"state"`
		Progress     float64 `json:"progress"`
		SavePath     string  `json:"save_path"`
		Category     string  `json:"category"`
		Tags         string  `json:"tags"`
		Size         int64   `json:"size"`
		TotalSize    int64   `json:"total_size"`
		Downloaded   int64   `json:"downloaded"`
		Uploaded     int64   `json:"uploaded"`
		DownloadRate int64   `json:"dlspeed"`
		UploadRate   int64   `json:"upspeed"`
		ETA          int64   `json:"eta"`
		Ratio        float64 `json:"ratio"`
		Seeders      int     `json:"num_seeds"`
		Peers        int     `json:"num_leechs"`
		AddedOn      int64   `json:"added_on"`
		CompletionOn int64   `json:"completion_on"`
		LastActivity int64   `json:"last_activity"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	statuses := make([]DownloadStatus, 0, len(raw))
	now := time.Now().UTC()
	for _, item := range raw {
		size := item.Size
		if size <= 0 {
			size = item.TotalSize
		}
		statuses = append(statuses, DownloadStatus{
			Client:          c.Name(),
			ID:              item.Hash,
			Name:            item.Name,
			State:           item.State,
			Progress:        item.Progress,
			SavePath:        item.SavePath,
			Category:        item.Category,
			Tags:            splitTags(item.Tags),
			SizeBytes:       size,
			DownloadedBytes: item.Downloaded,
			UploadedBytes:   item.Uploaded,
			DownloadRate:    item.DownloadRate,
			UploadRate:      item.UploadRate,
			ETASeconds:      item.ETA,
			Ratio:           item.Ratio,
			Seeders:         item.Seeders,
			Peers:           item.Peers,
			AddedAt:         unixTime(item.AddedOn),
			CompletedAt:     unixTime(item.CompletionOn),
			LastActivityAt:  unixTime(item.LastActivity),
			LastSeenAt:      &now,
		})
	}
	return statuses, nil
}

func (c *QBittorrentClient) Details(ctx context.Context, id string) (DownloadDetails, error) {
	if !c.Configured() {
		return DownloadDetails{}, ErrIntegrationNotConfigured
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return DownloadDetails{}, errors.New("download id is required")
	}
	statuses, err := c.List(ctx, DownloadListQuery{IDs: []string{id}})
	if err != nil {
		return DownloadDetails{}, err
	}
	if len(statuses) == 0 {
		return DownloadDetails{}, ErrDownloadNotFound
	}
	properties, err := c.properties(ctx, id)
	if err != nil {
		return DownloadDetails{}, err
	}
	files, err := c.files(ctx, id)
	if err != nil {
		return DownloadDetails{}, err
	}
	trackers, err := c.trackers(ctx, id)
	if err != nil {
		return DownloadDetails{}, err
	}
	peers, err := c.peers(ctx, id)
	if err != nil {
		return DownloadDetails{}, err
	}
	return DownloadDetails{
		Status:     statuses[0],
		Properties: properties,
		Files:      files,
		Trackers:   trackers,
		Peers:      peers,
	}, nil
}

func (c *QBittorrentClient) Action(ctx context.Context, request DownloadActionRequest) (DownloadActionResult, error) {
	if !c.Configured() {
		return DownloadActionResult{}, ErrIntegrationNotConfigured
	}
	ids := compactStrings(request.IDs)
	if len(ids) == 0 {
		return DownloadActionResult{}, errors.New("at least one download id is required")
	}
	if err := c.login(ctx); err != nil {
		return DownloadActionResult{}, err
	}

	action := normalizeAction(request.Action)
	values := url.Values{}
	values.Set("hashes", strings.Join(ids, "|"))

	switch action {
	case DownloadActionStart:
		if err := c.postTorrentAction(ctx, "start", values); err != nil {
			if fallbackErr := c.postTorrentAction(ctx, "resume", values); fallbackErr != nil {
				return DownloadActionResult{}, err
			}
		}
	case DownloadActionStop:
		if err := c.postTorrentAction(ctx, "stop", values); err != nil {
			if fallbackErr := c.postTorrentAction(ctx, "pause", values); fallbackErr != nil {
				return DownloadActionResult{}, err
			}
		}
	case DownloadActionDelete:
		values.Set("deleteFiles", boolString(request.DeleteFiles))
		if err := c.postTorrentAction(ctx, "delete", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionRecheck:
		if err := c.postTorrentAction(ctx, "recheck", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionIncreasePriority:
		if err := c.postTorrentAction(ctx, "increasePrio", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionDecreasePriority:
		if err := c.postTorrentAction(ctx, "decreasePrio", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionTopPriority:
		if err := c.postTorrentAction(ctx, "topPrio", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionBottomPriority:
		if err := c.postTorrentAction(ctx, "bottomPrio", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionSetCategory:
		category := strings.TrimSpace(request.Category)
		if category == "" {
			return DownloadActionResult{}, errors.New("category is required for setCategory")
		}
		values.Set("category", category)
		if err := c.postTorrentAction(ctx, "setCategory", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionSetLocation:
		location := strings.TrimSpace(request.SavePath)
		if location == "" {
			return DownloadActionResult{}, errors.New("savePath is required for setLocation")
		}
		values.Set("location", location)
		if err := c.postTorrentAction(ctx, "setLocation", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionSetDownloadLimit:
		if request.DownloadLimit < 0 {
			return DownloadActionResult{}, errors.New("downloadLimit must be zero or greater")
		}
		values.Set("limit", strconv.FormatInt(request.DownloadLimit, 10))
		if err := c.postTorrentAction(ctx, "setDownloadLimit", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionSetUploadLimit:
		if request.UploadLimit < 0 {
			return DownloadActionResult{}, errors.New("uploadLimit must be zero or greater")
		}
		values.Set("limit", strconv.FormatInt(request.UploadLimit, 10))
		if err := c.postTorrentAction(ctx, "setUploadLimit", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionForceStart:
		values.Set("value", boolString(request.ForceStart))
		if err := c.postTorrentAction(ctx, "setForceStart", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionToggleSequential:
		if err := c.postTorrentAction(ctx, "toggleSequentialDownload", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionToggleFirstLast:
		if err := c.postTorrentAction(ctx, "toggleFirstLastPiecePrio", values); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionRename:
		if len(ids) != 1 {
			return DownloadActionResult{}, errors.New("rename requires exactly one download id")
		}
		name := strings.TrimSpace(request.Name)
		if name == "" {
			return DownloadActionResult{}, errors.New("name is required for rename")
		}
		renameValues := url.Values{}
		renameValues.Set("hash", ids[0])
		renameValues.Set("name", name)
		if err := c.postTorrentAction(ctx, "rename", renameValues); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionAddTags, DownloadActionRemoveTags:
		tags := compactStrings(request.Tags)
		if len(tags) == 0 {
			return DownloadActionResult{}, errors.New("at least one tag is required")
		}
		values.Set("tags", strings.Join(tags, ","))
		endpoint := "addTags"
		if action == DownloadActionRemoveTags {
			endpoint = "removeTags"
		}
		if err := c.postTorrentAction(ctx, endpoint, values); err != nil {
			return DownloadActionResult{}, err
		}
	default:
		return DownloadActionResult{}, fmt.Errorf("unsupported download action %q", request.Action)
	}

	return DownloadActionResult{Action: action, IDs: ids, Applied: true}, nil
}

func (c *QBittorrentClient) FileAction(ctx context.Context, request DownloadFileActionRequest) (DownloadFileActionResult, error) {
	if !c.Configured() {
		return DownloadFileActionResult{}, ErrIntegrationNotConfigured
	}
	id := strings.TrimSpace(request.DownloadID)
	if id == "" {
		return DownloadFileActionResult{}, errors.New("download id is required")
	}
	fileIDs := compactInts(request.IDs)
	if len(fileIDs) == 0 {
		return DownloadFileActionResult{}, errors.New("at least one file id is required")
	}
	action, priority, err := normalizeFilePriorityAction(request.Action, request.Priority)
	if err != nil {
		return DownloadFileActionResult{}, err
	}
	if err := c.login(ctx); err != nil {
		return DownloadFileActionResult{}, err
	}
	values := url.Values{}
	values.Set("hash", id)
	values.Set("id", joinInts(fileIDs, "|"))
	values.Set("priority", strconv.Itoa(priority))
	if err := c.postTorrentAction(ctx, "filePrio", values); err != nil {
		return DownloadFileActionResult{}, err
	}
	return DownloadFileActionResult{
		Action:     action,
		DownloadID: id,
		IDs:        fileIDs,
		Priority:   priority,
		Applied:    true,
	}, nil
}

func (c *QBittorrentClient) TrackerAction(ctx context.Context, id string, request DownloadTrackerActionRequest) (DownloadTrackerActionResult, error) {
	if !c.Configured() {
		return DownloadTrackerActionResult{}, ErrIntegrationNotConfigured
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return DownloadTrackerActionResult{}, errors.New("download id is required")
	}
	action := normalizeTrackerAction(request.Action)
	if err := c.login(ctx); err != nil {
		return DownloadTrackerActionResult{}, err
	}

	values := url.Values{}
	values.Set("hash", id)
	var urls []string
	switch action {
	case DownloadTrackerActionAdd:
		urls = trackerURLs(request)
		if len(urls) == 0 {
			return DownloadTrackerActionResult{}, errors.New("at least one tracker url is required")
		}
		values.Set("urls", strings.Join(urls, "\n"))
		if err := c.postTorrentAction(ctx, "addTrackers", values); err != nil {
			return DownloadTrackerActionResult{}, err
		}
	case DownloadTrackerActionEdit:
		originalURL := strings.TrimSpace(firstNonEmpty(request.OriginalURL, request.URL))
		newURL := strings.TrimSpace(request.NewURL)
		if originalURL == "" || newURL == "" {
			return DownloadTrackerActionResult{}, errors.New("originalUrl and newUrl are required for edit")
		}
		values.Set("origUrl", originalURL)
		values.Set("newUrl", newURL)
		if err := c.postTorrentAction(ctx, "editTracker", values); err != nil {
			return DownloadTrackerActionResult{}, err
		}
		urls = []string{originalURL, newURL}
	case DownloadTrackerActionRemove:
		urls = trackerURLs(request)
		if len(urls) == 0 {
			return DownloadTrackerActionResult{}, errors.New("at least one tracker url is required")
		}
		values.Set("urls", strings.Join(urls, "|"))
		if err := c.postTorrentAction(ctx, "removeTrackers", values); err != nil {
			return DownloadTrackerActionResult{}, err
		}
	default:
		return DownloadTrackerActionResult{}, fmt.Errorf("unsupported tracker action %q", request.Action)
	}
	return DownloadTrackerActionResult{
		Action:     action,
		DownloadID: id,
		URLs:       urls,
		Applied:    true,
	}, nil
}

func (c *QBittorrentClient) properties(ctx context.Context, id string) (DownloadProperties, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/torrents/properties?hash="+url.QueryEscape(id), nil)
	if err != nil {
		return DownloadProperties{}, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return DownloadProperties{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return DownloadProperties{}, fmt.Errorf("qBittorrent properties returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var raw struct {
		SavePath         string  `json:"save_path"`
		CreationDate     int64   `json:"creation_date"`
		AdditionDate     int64   `json:"addition_date"`
		CompletionDate   int64   `json:"completion_date"`
		TotalSize        int64   `json:"total_size"`
		TotalDownloaded  int64   `json:"total_downloaded"`
		TotalUploaded    int64   `json:"total_uploaded"`
		DownloadLimit    int64   `json:"dl_limit"`
		UploadLimit      int64   `json:"up_limit"`
		DownloadSpeed    int64   `json:"dl_speed"`
		UploadSpeed      int64   `json:"up_speed"`
		ETA              int64   `json:"eta"`
		Ratio            float64 `json:"share_ratio"`
		Connections      int     `json:"nb_connections"`
		ConnectionsLimit int     `json:"nb_connections_limit"`
		TimeElapsed      int64   `json:"time_elapsed"`
		SeedingTime      int64   `json:"seeding_time"`
		PieceSize        int64   `json:"piece_size"`
		PiecesHave       int     `json:"pieces_have"`
		PiecesTotal      int     `json:"pieces_num"`
		Reannounce       int64   `json:"reannounce"`
		CreatedBy        string  `json:"created_by"`
		Comment          string  `json:"comment"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return DownloadProperties{}, err
	}
	return DownloadProperties{
		SavePath:           raw.SavePath,
		CreationDate:       unixTime(raw.CreationDate),
		AdditionDate:       unixTime(raw.AdditionDate),
		CompletionDate:     unixTime(raw.CompletionDate),
		TotalSizeBytes:     raw.TotalSize,
		TotalDownloaded:    raw.TotalDownloaded,
		TotalUploaded:      raw.TotalUploaded,
		DownloadLimit:      raw.DownloadLimit,
		UploadLimit:        raw.UploadLimit,
		DownloadSpeed:      raw.DownloadSpeed,
		UploadSpeed:        raw.UploadSpeed,
		ETASeconds:         raw.ETA,
		Ratio:              raw.Ratio,
		Connections:        raw.Connections,
		ConnectionsLimit:   raw.ConnectionsLimit,
		TimeElapsedSeconds: raw.TimeElapsed,
		SeedingTimeSeconds: raw.SeedingTime,
		PieceSizeBytes:     raw.PieceSize,
		PiecesHave:         raw.PiecesHave,
		PiecesTotal:        raw.PiecesTotal,
		ReannounceSeconds:  raw.Reannounce,
		CreatedBy:          strings.TrimSpace(raw.CreatedBy),
		Comment:            strings.TrimSpace(raw.Comment),
	}, nil
}

func (c *QBittorrentClient) files(ctx context.Context, id string) ([]DownloadFile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/torrents/files?hash="+url.QueryEscape(id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qBittorrent files returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var raw []struct {
		Name         string  `json:"name"`
		Size         int64   `json:"size"`
		Progress     float64 `json:"progress"`
		Priority     int     `json:"priority"`
		Availability float64 `json:"availability"`
		IsSeed       bool    `json:"is_seed"`
		PieceRange   []int   `json:"piece_range"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	files := make([]DownloadFile, 0, len(raw))
	for i, item := range raw {
		firstPiece, lastPiece := pieceRange(item.PieceRange)
		files = append(files, DownloadFile{
			ID:           i,
			Name:         item.Name,
			SizeBytes:    item.Size,
			Progress:     item.Progress,
			Priority:     item.Priority,
			Availability: item.Availability,
			IsSeed:       item.IsSeed,
			FirstPiece:   firstPiece,
			LastPiece:    lastPiece,
		})
	}
	return files, nil
}

func (c *QBittorrentClient) trackers(ctx context.Context, id string) ([]DownloadTracker, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/torrents/trackers?hash="+url.QueryEscape(id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qBittorrent trackers returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var raw []struct {
		URL       string `json:"url"`
		Status    int    `json:"status"`
		Tier      int    `json:"tier"`
		Message   string `json:"msg"`
		Peers     int    `json:"num_peers"`
		Seeds     int    `json:"num_seeds"`
		Leeches   int    `json:"num_leeches"`
		Downloads int    `json:"num_downloaded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	trackers := make([]DownloadTracker, 0, len(raw))
	for _, item := range raw {
		trackers = append(trackers, DownloadTracker{
			URL:        item.URL,
			StatusCode: item.Status,
			Status:     trackerStatus(item.Status),
			Tier:       item.Tier,
			Message:    strings.TrimSpace(item.Message),
			Peers:      item.Peers,
			Seeds:      item.Seeds,
			Leeches:    item.Leeches,
			Downloads:  item.Downloads,
		})
	}
	return trackers, nil
}

func (c *QBittorrentClient) peers(ctx context.Context, id string) ([]DownloadPeer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/sync/torrentPeers?hash="+url.QueryEscape(id)+"&rid=0", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qBittorrent peers returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var raw struct {
		Peers map[string]struct {
			Client           string  `json:"client"`
			Connection       string  `json:"connection"`
			Country          string  `json:"country"`
			CountryCode      string  `json:"country_code"`
			DownloadRate     int64   `json:"dl_speed"`
			Downloaded       int64   `json:"downloaded"`
			Files            string  `json:"files"`
			Flags            string  `json:"flags"`
			FlagsDescription string  `json:"flags_desc"`
			IP               string  `json:"ip"`
			Port             int     `json:"port"`
			Progress         float64 `json:"progress"`
			Relevance        float64 `json:"relevance"`
			UploadRate       int64   `json:"up_speed"`
			Uploaded         int64   `json:"uploaded"`
		} `json:"peers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	peers := make([]DownloadPeer, 0, len(raw.Peers))
	for key, item := range raw.Peers {
		peerID := strings.TrimSpace(key)
		if peerID == "" {
			peerID = item.IP
		}
		peers = append(peers, DownloadPeer{
			ID:               peerID,
			IP:               item.IP,
			Port:             item.Port,
			Client:           strings.TrimSpace(item.Client),
			Connection:       strings.TrimSpace(item.Connection),
			Country:          strings.TrimSpace(item.Country),
			CountryCode:      strings.TrimSpace(item.CountryCode),
			Flags:            strings.TrimSpace(item.Flags),
			FlagsDescription: strings.TrimSpace(item.FlagsDescription),
			Progress:         item.Progress,
			Relevance:        item.Relevance,
			DownloadRate:     item.DownloadRate,
			UploadRate:       item.UploadRate,
			DownloadedBytes:  item.Downloaded,
			UploadedBytes:    item.Uploaded,
			Files:            strings.TrimSpace(item.Files),
		})
	}
	return peers, nil
}

func (c *QBittorrentClient) editCategory(ctx context.Context, category string, savePath string) error {
	values := url.Values{}
	values.Set("category", category)
	values.Set("savePath", savePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/editCategory", strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qBittorrent edit category returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *QBittorrentClient) postTorrentAction(ctx context.Context, action string, values url.Values) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/"+action, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qBittorrent %s returned %s: %s", action, resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *QBittorrentClient) login(ctx context.Context) error {
	if c.username == "" && c.password == "" {
		return nil
	}
	values := url.Values{}
	values.Set("username", c.username)
	values.Set("password", c.password)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/auth/login", strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 || !strings.Contains(strings.ToLower(string(body)), "ok") {
		return fmt.Errorf("qBittorrent login failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func normalizeAction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "resume", "start":
		return DownloadActionStart
	case "pause", "stop":
		return DownloadActionStop
	case "delete", "remove":
		return DownloadActionDelete
	case "recheck", "forcecheck", "force_check":
		return DownloadActionRecheck
	case "increaseprio", "increasepriority", "increase_priority":
		return DownloadActionIncreasePriority
	case "decreaseprio", "decreasepriority", "decrease_priority":
		return DownloadActionDecreasePriority
	case "topprio", "toppriority", "top_priority":
		return DownloadActionTopPriority
	case "bottomprio", "bottompriority", "bottom_priority":
		return DownloadActionBottomPriority
	case "setcategory", "set_category":
		return DownloadActionSetCategory
	case "setlocation", "set_location":
		return DownloadActionSetLocation
	case "setdownloadlimit", "set_download_limit", "downloadlimit", "download_limit":
		return DownloadActionSetDownloadLimit
	case "setuploadlimit", "set_upload_limit", "uploadlimit", "upload_limit":
		return DownloadActionSetUploadLimit
	case "forcestart", "force_start", "setforcestart", "set_force_start":
		return DownloadActionForceStart
	case "togglesequential", "toggle_sequential", "togglesequentialdownload", "sequential":
		return DownloadActionToggleSequential
	case "togglefirstlastpiece", "toggle_first_last_piece", "togglefirstlastpieceprio", "firstlastpiece", "first_last_piece":
		return DownloadActionToggleFirstLast
	case "rename", "setname", "set_name":
		return DownloadActionRename
	case "addtags", "add_tags", "tag":
		return DownloadActionAddTags
	case "removetags", "remove_tags", "untag":
		return DownloadActionRemoveTags
	default:
		return strings.TrimSpace(value)
	}
}

func splitTags(value string) []string {
	parts := strings.Split(value, ",")
	var tags []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			tags = append(tags, part)
		}
	}
	return tags
}

func compactInts(values []int) []int {
	var compacted []int
	seen := map[int]bool{}
	for _, value := range values {
		if value < 0 || seen[value] {
			continue
		}
		seen[value] = true
		compacted = append(compacted, value)
	}
	return compacted
}

func joinInts(values []int, sep string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, sep)
}

func normalizeFilePriorityAction(action string, priority int) (string, int, error) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "skip", "ignore", "do_not_download", "donotdownload":
		return DownloadFileActionSkip, 0, nil
	case "normal", "download":
		return DownloadFileActionNormal, 1, nil
	case "high":
		return DownloadFileActionHigh, 6, nil
	case "max", "maximum":
		return DownloadFileActionMax, 7, nil
	case "priority", "setpriority", "set_priority":
		if priority == 0 || priority == 1 || priority == 6 || priority == 7 {
			return actionOrDefault(action, "priority"), priority, nil
		}
		return "", 0, fmt.Errorf("unsupported file priority %d", priority)
	default:
		if priority == 0 || priority == 1 || priority == 6 || priority == 7 {
			return actionOrDefault(action, "priority"), priority, nil
		}
		return "", 0, fmt.Errorf("unsupported download file action %q", action)
	}
}

func normalizeTrackerAction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "add", "addtracker", "add_trackers", "addtrackers":
		return DownloadTrackerActionAdd
	case "edit", "replace", "edittracker", "edit_tracker":
		return DownloadTrackerActionEdit
	case "remove", "delete", "removetracker", "remove_tracker", "removetrackers":
		return DownloadTrackerActionRemove
	default:
		return strings.TrimSpace(value)
	}
}

func trackerURLs(request DownloadTrackerActionRequest) []string {
	var urls []string
	urls = append(urls, request.URLs...)
	if strings.TrimSpace(request.URL) != "" {
		urls = append(urls, request.URL)
	}
	seen := map[string]bool{}
	compacted := make([]string, 0, len(urls))
	for _, value := range urls {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		compacted = append(compacted, value)
	}
	return compacted
}

func actionOrDefault(action string, fallback string) string {
	action = strings.TrimSpace(action)
	if action == "" {
		return fallback
	}
	return action
}

func pieceRange(values []int) (int, int) {
	if len(values) < 2 {
		return 0, 0
	}
	return values[0], values[1]
}

func trackerStatus(value int) string {
	switch value {
	case 0:
		return "disabled"
	case 1:
		return "not_contacted"
	case 2:
		return "working"
	case 3:
		return "updating"
	case 4:
		return "not_working"
	default:
		return "unknown"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func compactStrings(values []string) []string {
	var compacted []string
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		compacted = append(compacted, value)
	}
	return compacted
}

func unixTime(seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	value := time.Unix(seconds, 0).UTC()
	return &value
}
