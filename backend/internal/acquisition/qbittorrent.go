package acquisition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
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
	if strings.TrimSpace(request.ReleaseURL) == "" {
		return DownloadStatus{}, errors.New("releaseUrl is required")
	}
	if err := c.login(ctx); err != nil {
		return DownloadStatus{}, err
	}

	var body strings.Builder
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("urls", request.ReleaseURL)
	_ = writer.WriteField("category", request.Category)
	_ = writer.WriteField("savepath", request.SavePath)
	_ = writer.WriteField("paused", boolString(request.Paused))
	_ = writer.WriteField("stopped", boolString(request.Paused))
	if len(request.Tags) > 0 {
		_ = writer.WriteField("tags", strings.Join(request.Tags, ","))
	}
	_ = writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/add", strings.NewReader(body.String()))
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
		ID:         id,
		Name:       firstNonEmpty(request.Title, request.ReleaseURL),
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
			LastSeenAt:      &now,
		})
	}
	return statuses, nil
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
	default:
		return DownloadActionResult{}, fmt.Errorf("unsupported download action %q", request.Action)
	}

	return DownloadActionResult{Action: action, IDs: ids, Applied: true}, nil
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
