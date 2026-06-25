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
	if summary, ok := parseAddSummary(respBody); ok {
		if summary.FailureCount > 0 && summary.SuccessCount == 0 && summary.PendingCount == 0 {
			return DownloadStatus{}, fmt.Errorf("qBittorrent add failed: %s", strings.TrimSpace(string(respBody)))
		}
		if summary.PendingCount > 0 {
			state = "pending"
		}
	} else if !strings.Contains(strings.ToLower(string(respBody)), "ok") {
		return DownloadStatus{}, fmt.Errorf("qBittorrent add returned unexpected body: %s", strings.TrimSpace(string(respBody)))
	}
	return DownloadStatus{
		ID:       firstNonEmpty(request.InfoHash, releaseID(request.ReleaseURL)),
		Name:     firstNonEmpty(request.Title, request.ReleaseURL),
		State:    state,
		Progress: 0,
		SavePath: request.SavePath,
		Category: request.Category,
		Tags:     request.Tags,
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

func (c *QBittorrentClient) List(ctx context.Context, tag string) ([]DownloadStatus, error) {
	if !c.Configured() {
		return nil, ErrIntegrationNotConfigured
	}
	if err := c.login(ctx); err != nil {
		return nil, err
	}
	values := url.Values{}
	if strings.TrimSpace(tag) != "" {
		values.Set("tag", tag)
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
		Hash     string  `json:"hash"`
		Name     string  `json:"name"`
		State    string  `json:"state"`
		Progress float64 `json:"progress"`
		SavePath string  `json:"save_path"`
		Category string  `json:"category"`
		Tags     string  `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	statuses := make([]DownloadStatus, 0, len(raw))
	for _, item := range raw {
		statuses = append(statuses, DownloadStatus{
			ID:       item.Hash,
			Name:     item.Name,
			State:    item.State,
			Progress: item.Progress,
			SavePath: item.SavePath,
			Category: item.Category,
			Tags:     splitTags(item.Tags),
		})
	}
	return statuses, nil
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
