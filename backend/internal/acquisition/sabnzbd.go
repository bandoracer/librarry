package acquisition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SABnzbdClient struct {
	baseURL  string
	apiKey   string
	username string
	password string
	client   *http.Client
}

func NewSABnzbdClient(baseURL string, apiKey string, username string, password string, client *http.Client) *SABnzbdClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &SABnzbdClient{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:   strings.TrimSpace(apiKey),
		username: strings.TrimSpace(username),
		password: strings.TrimSpace(password),
		client:   client,
	}
}

func (c *SABnzbdClient) Name() string { return "SABnzbd" }

func (c *SABnzbdClient) Configured() bool {
	return c.baseURL != "" && c.apiKey != ""
}

func (c *SABnzbdClient) Health(ctx context.Context) IntegrationHealth {
	if c.baseURL == "" {
		return IntegrationHealth{Name: c.Name(), Configured: false, Status: "missing_credentials", Message: "Set LIBRARRY_SABNZBD_URL."}
	}
	if c.apiKey == "" {
		return IntegrationHealth{Name: c.Name(), Configured: false, Status: "missing_credentials", Message: "Set LIBRARRY_SABNZBD_API_KEY."}
	}
	var payload sabVersionResponse
	if err := c.api(ctx, url.Values{"mode": {"version"}}, &payload); err != nil {
		return IntegrationHealth{Name: c.Name(), Configured: true, Status: "error", Message: err.Error()}
	}
	message := "Ready"
	if payload.Version != "" {
		message = "Ready; version " + payload.Version
	}
	return IntegrationHealth{Name: c.Name(), Configured: true, Status: "ready", Message: message}
}

func (c *SABnzbdClient) Add(ctx context.Context, request DownloadRequest) (DownloadStatus, error) {
	if !c.Configured() {
		return DownloadStatus{}, ErrIntegrationNotConfigured
	}
	if strings.TrimSpace(request.ReleaseURL) == "" {
		return DownloadStatus{}, errors.New("releaseUrl is required")
	}
	values := url.Values{
		"mode":    {"addurl"},
		"name":    {request.ReleaseURL},
		"cat":     {request.Category},
		"nzbname": {firstNonEmpty(request.Title, request.ReleaseURL)},
	}
	var payload sabAddResponse
	if err := c.api(ctx, values, &payload); err != nil {
		return DownloadStatus{}, err
	}
	if !payload.OK() {
		return DownloadStatus{}, fmt.Errorf("SABnzbd add failed: %s", payload.Error())
	}
	id := firstString(payload.NZOIDs)
	if id == "" {
		id = releaseID(request.ReleaseURL)
	}
	if request.Paused && id != "" {
		_, _ = c.Action(ctx, DownloadActionRequest{Action: DownloadActionStop, IDs: []string{id}})
	}
	now := time.Now().UTC()
	return DownloadStatus{
		Client:     c.Name(),
		ID:         id,
		Name:       firstNonEmpty(request.Title, request.ReleaseURL),
		State:      "queued",
		Progress:   0,
		Category:   request.Category,
		Tags:       request.Tags,
		LastSeenAt: &now,
	}, nil
}

func (c *SABnzbdClient) List(ctx context.Context, query DownloadListQuery) ([]DownloadStatus, error) {
	if !c.Configured() {
		return nil, ErrIntegrationNotConfigured
	}
	var queue sabQueueResponse
	if err := c.api(ctx, url.Values{"mode": {"queue"}}, &queue); err != nil {
		return nil, err
	}
	var history sabHistoryResponse
	if err := c.api(ctx, url.Values{"mode": {"history"}, "limit": {"100"}}, &history); err != nil {
		return nil, err
	}
	ids := make(map[string]bool)
	for _, id := range compactStrings(query.IDs) {
		ids[id] = true
	}
	statuses := make([]DownloadStatus, 0, len(queue.Queue.Slots)+len(history.History.Slots))
	now := time.Now().UTC()
	for _, slot := range queue.Queue.Slots {
		status := slot.DownloadStatus(now)
		if includeSABStatus(status, query, ids) {
			statuses = append(statuses, status)
		}
	}
	for _, slot := range history.History.Slots {
		status := slot.DownloadStatus(now)
		if includeSABStatus(status, query, ids) {
			statuses = append(statuses, status)
		}
	}
	return statuses, nil
}

func (c *SABnzbdClient) Action(ctx context.Context, request DownloadActionRequest) (DownloadActionResult, error) {
	if !c.Configured() {
		return DownloadActionResult{}, ErrIntegrationNotConfigured
	}
	ids := compactStrings(request.IDs)
	if len(ids) == 0 {
		return DownloadActionResult{}, errors.New("at least one download id is required")
	}
	action := normalizeAction(request.Action)
	for _, id := range ids {
		switch action {
		case DownloadActionStart:
			if err := c.queueCommand(ctx, "resume", id, nil); err != nil {
				return DownloadActionResult{}, err
			}
		case DownloadActionStop:
			if err := c.queueCommand(ctx, "pause", id, nil); err != nil {
				return DownloadActionResult{}, err
			}
		case DownloadActionDelete:
			extra := url.Values{}
			extra.Set("del_files", boolString(request.DeleteFiles))
			if err := c.queueCommand(ctx, "delete", id, extra); err != nil {
				return DownloadActionResult{}, err
			}
			_ = c.historyCommand(ctx, "delete", id, nil)
		default:
			return DownloadActionResult{}, fmt.Errorf("unsupported SABnzbd download action %q", request.Action)
		}
	}
	return DownloadActionResult{Action: action, IDs: ids, Applied: true}, nil
}

func (c *SABnzbdClient) queueCommand(ctx context.Context, name string, value string, extra url.Values) error {
	values := url.Values{"mode": {"queue"}, "name": {name}, "value": {value}}
	for key, list := range extra {
		for _, value := range list {
			values.Add(key, value)
		}
	}
	var payload sabStatusResponse
	if err := c.api(ctx, values, &payload); err != nil {
		return err
	}
	if !payload.OK() {
		return fmt.Errorf("SABnzbd queue command %s failed: %s", name, payload.Error())
	}
	return nil
}

func (c *SABnzbdClient) historyCommand(ctx context.Context, name string, value string, extra url.Values) error {
	values := url.Values{"mode": {"history"}, "name": {name}, "value": {value}}
	for key, list := range extra {
		for _, value := range list {
			values.Add(key, value)
		}
	}
	var payload sabStatusResponse
	if err := c.api(ctx, values, &payload); err != nil {
		return err
	}
	if !payload.OK() {
		return fmt.Errorf("SABnzbd history command %s failed: %s", name, payload.Error())
	}
	return nil
}

func (c *SABnzbdClient) api(ctx context.Context, values url.Values, target any) error {
	if !c.Configured() {
		return ErrIntegrationNotConfigured
	}
	values.Set("apikey", c.apiKey)
	values.Set("output", "json")
	endpoint := c.baseURL + "/api?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("SABnzbd API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if target == nil || len(body) == 0 {
		return nil
	}
	var apiError struct {
		ErrorText string `json:"error"`
	}
	if err := json.Unmarshal(body, &apiError); err == nil && strings.TrimSpace(apiError.ErrorText) != "" {
		return fmt.Errorf("SABnzbd API error: %s", strings.TrimSpace(apiError.ErrorText))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("SABnzbd API decode failed: %w", err)
	}
	return nil
}

type sabVersionResponse struct {
	Version string `json:"version"`
}

func (r *sabVersionResponse) UnmarshalJSON(data []byte) error {
	var object struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &object); err == nil && object.Version != "" {
		r.Version = object.Version
		return nil
	}
	var version string
	if err := json.Unmarshal(data, &version); err == nil {
		r.Version = version
		return nil
	}
	return nil
}

type sabAddResponse struct {
	Status    any      `json:"status"`
	NZOIDs    []string `json:"nzo_ids"`
	ErrorText string   `json:"error"`
}

func (r sabAddResponse) OK() bool {
	switch value := r.Status.(type) {
	case bool:
		return value
	case float64:
		return value >= 0
	case string:
		normalized := strings.ToLower(strings.TrimSpace(value))
		return normalized == "true" || normalized == "ok" || normalized == "0"
	default:
		return len(r.NZOIDs) > 0 && r.ErrorText == ""
	}
}

func (r sabAddResponse) Error() string {
	if strings.TrimSpace(r.ErrorText) != "" {
		return strings.TrimSpace(r.ErrorText)
	}
	return "unexpected response"
}

type sabStatusResponse struct {
	Status    any    `json:"status"`
	ErrorText string `json:"error"`
}

func (r sabStatusResponse) OK() bool {
	if r.Status == nil && r.ErrorText == "" {
		return true
	}
	switch value := r.Status.(type) {
	case bool:
		return value
	case float64:
		return value >= 0
	case string:
		normalized := strings.ToLower(strings.TrimSpace(value))
		return normalized == "true" || normalized == "ok" || normalized == "0"
	default:
		return r.ErrorText == ""
	}
}

func (r sabStatusResponse) Error() string {
	if strings.TrimSpace(r.ErrorText) != "" {
		return strings.TrimSpace(r.ErrorText)
	}
	return "unexpected response"
}

type sabQueueResponse struct {
	Queue struct {
		Slots []sabQueueSlot `json:"slots"`
	} `json:"queue"`
}

type sabQueueSlot struct {
	NZOID      string `json:"nzo_id"`
	Filename   string `json:"filename"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Category   string `json:"cat"`
	Percentage string `json:"percentage"`
	Size       string `json:"size"`
	MB         string `json:"mb"`
	MBLeft     string `json:"mbleft"`
	TimeLeft   string `json:"timeleft"`
}

func (s sabQueueSlot) DownloadStatus(now time.Time) DownloadStatus {
	sizeBytes := parseSABSize(s.Size)
	if sizeBytes == 0 {
		sizeBytes = parseSABMegabytes(s.MB)
	}
	leftBytes := parseSABMegabytes(s.MBLeft)
	progress := parseSABPercent(s.Percentage)
	downloaded := int64(0)
	if sizeBytes > 0 && leftBytes > 0 && leftBytes <= sizeBytes {
		downloaded = sizeBytes - leftBytes
	}
	return DownloadStatus{
		Client:          "SABnzbd",
		ID:              s.NZOID,
		Name:            firstNonEmpty(s.Filename, s.Name, s.NZOID),
		State:           normalizeSABState(s.Status),
		Progress:        progress,
		Category:        s.Category,
		Tags:            []string{"librarry"},
		SizeBytes:       sizeBytes,
		DownloadedBytes: downloaded,
		ETASeconds:      parseSABDuration(s.TimeLeft),
		LastSeenAt:      &now,
	}
}

type sabHistoryResponse struct {
	History struct {
		Slots []sabHistorySlot `json:"slots"`
	} `json:"history"`
}

type sabHistorySlot struct {
	NZOID       string `json:"nzo_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Category    string `json:"category"`
	Size        string `json:"size"`
	Completed   int64  `json:"completed"`
	FailMessage string `json:"fail_message"`
}

func (s sabHistorySlot) DownloadStatus(now time.Time) DownloadStatus {
	progress := 1.0
	if strings.Contains(strings.ToLower(s.Status), "fail") {
		progress = 0
	}
	status := DownloadStatus{
		Client:        "SABnzbd",
		ID:            s.NZOID,
		Name:          firstNonEmpty(s.Name, s.NZOID),
		State:         normalizeSABState(s.Status),
		Progress:      progress,
		Category:      s.Category,
		Tags:          []string{"librarry"},
		SizeBytes:     parseSABSize(s.Size),
		LastSeenAt:    &now,
		CompletedAt:   unixTime(s.Completed),
		FailureReason: strings.TrimSpace(s.FailMessage),
	}
	if status.Progress >= 1 {
		status.DownloadedBytes = status.SizeBytes
	}
	if status.FailureReason != "" && status.FailedAt == nil {
		failedAt := now
		status.FailedAt = &failedAt
	}
	return status
}

func includeSABStatus(status DownloadStatus, query DownloadListQuery, ids map[string]bool) bool {
	if status.ID == "" {
		return false
	}
	if len(ids) > 0 && !ids[status.ID] {
		return false
	}
	if client := strings.TrimSpace(query.Client); client != "" && !strings.EqualFold(client, status.Client) {
		return false
	}
	if category := strings.TrimSpace(query.Category); category != "" && !strings.EqualFold(category, status.Category) {
		return false
	}
	return true
}

func normalizeSABState(state string) string {
	normalized := strings.ToLower(strings.TrimSpace(state))
	switch {
	case normalized == "":
		return "queued"
	case strings.Contains(normalized, "complete"):
		return "completed"
	case strings.Contains(normalized, "fail"):
		return "error"
	case strings.Contains(normalized, "pause"):
		return "paused"
	case strings.Contains(normalized, "download"):
		return "downloading"
	default:
		return strings.TrimSpace(state)
	}
}

func parseSABPercent(value string) float64 {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	if parsed > 1 {
		parsed = parsed / 100
	}
	if parsed < 0 {
		return 0
	}
	if parsed > 1 {
		return 1
	}
	return parsed
}

func parseSABSize(value string) int64 {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return 0
	}
	normalized = strings.ReplaceAll(normalized, ",", "")
	fields := strings.Fields(normalized)
	if len(fields) == 0 {
		return 0
	}
	unit := ""
	numberText := strings.TrimSuffix(fields[0], "b")
	if len(fields) > 1 {
		unit = strings.TrimSuffix(fields[1], "b")
	} else {
		for _, suffix := range []string{"t", "g", "m", "k"} {
			if strings.HasSuffix(numberText, suffix) {
				unit = suffix
				numberText = strings.TrimSuffix(numberText, suffix)
				break
			}
		}
	}
	valueFloat, err := strconv.ParseFloat(numberText, 64)
	if err != nil {
		return 0
	}
	switch unit {
	case "tb", "t":
		valueFloat *= 1024 * 1024 * 1024 * 1024
	case "gb", "g":
		valueFloat *= 1024 * 1024 * 1024
	case "mb", "m":
		valueFloat *= 1024 * 1024
	case "kb", "k":
		valueFloat *= 1024
	}
	return int64(valueFloat)
}

func parseSABMegabytes(value string) int64 {
	parsed, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", ""), 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return int64(parsed * 1024 * 1024)
}

func parseSABDuration(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parts := strings.Split(value, ":")
	var seconds int64
	for _, part := range parts {
		parsed, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			return 0
		}
		seconds = seconds*60 + parsed
	}
	return seconds
}
