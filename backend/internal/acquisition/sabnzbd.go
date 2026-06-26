package acquisition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
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
	ids := make(map[string]bool)
	compactIDs := compactStrings(query.IDs)
	for _, id := range compactIDs {
		ids[id] = true
	}
	queueValues := url.Values{"mode": {"queue"}}
	historyValues := url.Values{"mode": {"history"}, "limit": {"100"}}
	if len(compactIDs) > 0 {
		idList := strings.Join(compactIDs, ",")
		queueValues.Set("nzo_ids", idList)
		historyValues.Set("nzo_ids", idList)
	}
	var queue sabQueueResponse
	if err := c.api(ctx, queueValues, &queue); err != nil {
		return nil, err
	}
	var history sabHistoryResponse
	if err := c.api(ctx, historyValues, &history); err != nil {
		return nil, err
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

func (c *SABnzbdClient) Details(ctx context.Context, id string) (DownloadDetails, error) {
	if !c.Configured() {
		return DownloadDetails{}, ErrIntegrationNotConfigured
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return DownloadDetails{}, errors.New("download id is required")
	}
	if details, ok, err := c.queueDetails(ctx, id); err != nil || ok {
		return details, err
	}
	if details, ok, err := c.historyDetails(ctx, id); err != nil || ok {
		return details, err
	}
	return DownloadDetails{}, ErrDownloadNotFound
}

func (c *SABnzbdClient) Resources(ctx context.Context) (DownloadResources, error) {
	if !c.Configured() {
		return DownloadResources{}, ErrIntegrationNotConfigured
	}
	categories, err := c.categories(ctx)
	if err != nil {
		return DownloadResources{}, err
	}
	return DownloadResources{
		Client:     c.Name(),
		Categories: categories,
		Tags:       []string{},
	}, nil
}

func (c *SABnzbdClient) CategoryAction(ctx context.Context, request DownloadCategoryActionRequest) (DownloadResourceActionResult, error) {
	if !c.Configured() {
		return DownloadResourceActionResult{}, ErrIntegrationNotConfigured
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return DownloadResourceActionResult{}, errors.New("category name is required")
	}
	action := normalizeResourceAction(request.Action)
	switch action {
	case "create", "upsert":
		if err := c.upsertCategory(ctx, name, strings.TrimSpace(request.SavePath)); err != nil {
			return DownloadResourceActionResult{}, err
		}
	case "edit":
		newName := strings.TrimSpace(request.NewName)
		if newName != "" && !strings.EqualFold(newName, name) {
			if name == "*" {
				return DownloadResourceActionResult{}, errors.New("SABnzbd default category cannot be renamed")
			}
			if err := c.upsertCategory(ctx, newName, strings.TrimSpace(request.SavePath)); err != nil {
				return DownloadResourceActionResult{}, err
			}
			if err := c.deleteCategory(ctx, name); err != nil {
				return DownloadResourceActionResult{}, err
			}
		} else if err := c.upsertCategory(ctx, name, strings.TrimSpace(request.SavePath)); err != nil {
			return DownloadResourceActionResult{}, err
		}
	case "delete":
		if name == "*" {
			return DownloadResourceActionResult{}, errors.New("SABnzbd default category cannot be deleted")
		}
		if err := c.deleteCategory(ctx, name); err != nil {
			return DownloadResourceActionResult{}, err
		}
	default:
		return DownloadResourceActionResult{}, fmt.Errorf("unsupported SABnzbd category action %q", request.Action)
	}
	result := DownloadResourceActionResult{
		Action:  action,
		Client:  c.Name(),
		Applied: true,
	}
	if resources, err := c.Resources(ctx); err == nil {
		result.Resources = &resources
	}
	return result, nil
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
		case DownloadActionSetCategory:
			category := strings.TrimSpace(request.Category)
			if category == "" {
				return DownloadActionResult{}, errors.New("category is required for setCategory")
			}
			if err := c.changeJobValue(ctx, "change_cat", id, category); err != nil {
				return DownloadActionResult{}, err
			}
		case DownloadActionRename:
			name := strings.TrimSpace(request.Name)
			if name == "" {
				return DownloadActionResult{}, errors.New("name is required for rename")
			}
			extra := url.Values{"value2": {name}}
			if err := c.queueCommand(ctx, "rename", id, extra); err != nil {
				return DownloadActionResult{}, err
			}
		case DownloadActionIncreasePriority,
			DownloadActionDecreasePriority,
			DownloadActionTopPriority,
			DownloadActionBottomPriority:
			extra := url.Values{"value2": {sabPriorityForAction(action)}}
			if err := c.queueCommand(ctx, "priority", id, extra); err != nil {
				return DownloadActionResult{}, err
			}
		default:
			return DownloadActionResult{}, fmt.Errorf("unsupported SABnzbd download action %q", request.Action)
		}
	}
	return DownloadActionResult{Action: action, IDs: ids, Applied: true}, nil
}

func (c *SABnzbdClient) queueDetails(ctx context.Context, id string) (DownloadDetails, bool, error) {
	var queue sabQueueResponse
	values := url.Values{"mode": {"queue"}, "nzo_ids": {id}, "limit": {"1"}}
	if err := c.api(ctx, values, &queue); err != nil {
		return DownloadDetails{}, false, err
	}
	if len(queue.Queue.Slots) == 0 {
		return DownloadDetails{}, false, nil
	}
	slot := queue.Queue.Slots[0]
	now := time.Now().UTC()
	files, err := c.files(ctx, id)
	if err != nil {
		files = nil
	}
	return DownloadDetails{
		Status:     slot.DownloadStatus(now),
		Properties: slot.DownloadProperties(),
		Files:      files,
	}, true, nil
}

func (c *SABnzbdClient) historyDetails(ctx context.Context, id string) (DownloadDetails, bool, error) {
	var history sabHistoryResponse
	values := url.Values{"mode": {"history"}, "nzo_ids": {id}, "limit": {"1"}}
	if err := c.api(ctx, values, &history); err != nil {
		return DownloadDetails{}, false, err
	}
	if len(history.History.Slots) == 0 {
		return DownloadDetails{}, false, nil
	}
	slot := history.History.Slots[0]
	return DownloadDetails{
		Status:     slot.DownloadStatus(time.Now().UTC()),
		Properties: slot.DownloadProperties(),
	}, true, nil
}

func (c *SABnzbdClient) files(ctx context.Context, id string) ([]DownloadFile, error) {
	var payload sabFilesResponse
	if err := c.api(ctx, url.Values{"mode": {"get_files"}, "value": {id}}, &payload); err != nil {
		return nil, err
	}
	files := make([]DownloadFile, 0, len(payload.Files))
	for i, file := range payload.Files {
		files = append(files, file.DownloadFile(i))
	}
	return files, nil
}

func (c *SABnzbdClient) categories(ctx context.Context) ([]DownloadCategory, error) {
	var payload sabCategoriesResponse
	if err := c.api(ctx, url.Values{"mode": {"get_cats"}}, &payload); err != nil {
		return nil, err
	}
	savePaths := map[string]string{}
	if paths, err := c.categorySavePaths(ctx); err == nil {
		savePaths = paths
	}
	seen := map[string]bool{}
	categories := make([]DownloadCategory, 0, len(payload.Categories))
	for _, rawName := range payload.Categories {
		name := strings.TrimSpace(rawName)
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		categories = append(categories, DownloadCategory{
			Name:     name,
			SavePath: strings.TrimSpace(savePaths[name]),
		})
	}
	sort.Slice(categories, func(i, j int) bool {
		return strings.ToLower(categories[i].Name) < strings.ToLower(categories[j].Name)
	})
	return categories, nil
}

func (c *SABnzbdClient) categorySavePaths(ctx context.Context) (map[string]string, error) {
	var payload any
	if err := c.api(ctx, url.Values{"mode": {"get_config"}, "section": {"categories"}}, &payload); err != nil {
		return nil, err
	}
	return sabCategorySavePaths(payload), nil
}

func (c *SABnzbdClient) upsertCategory(ctx context.Context, name string, savePath string) error {
	values := url.Values{
		"mode":    {"set_config"},
		"section": {"categories"},
		"name":    {name},
	}
	if savePath != "" {
		values.Set("dir", savePath)
	}
	var payload sabStatusResponse
	if err := c.api(ctx, values, &payload); err != nil {
		return err
	}
	if !payload.OK() {
		return fmt.Errorf("SABnzbd category save failed: %s", payload.Error())
	}
	return nil
}

func (c *SABnzbdClient) deleteCategory(ctx context.Context, name string) error {
	values := url.Values{
		"mode":    {"del_config"},
		"section": {"categories"},
		"keyword": {name},
	}
	var payload sabStatusResponse
	if err := c.api(ctx, values, &payload); err != nil {
		return err
	}
	if !payload.OK() {
		return fmt.Errorf("SABnzbd category delete failed: %s", payload.Error())
	}
	return nil
}

func (c *SABnzbdClient) changeJobValue(ctx context.Context, mode string, id string, value string) error {
	values := url.Values{"mode": {mode}, "value": {id}, "value2": {value}}
	var payload sabStatusResponse
	if err := c.api(ctx, values, &payload); err != nil {
		return err
	}
	if !payload.OK() {
		return fmt.Errorf("SABnzbd %s failed: %s", mode, payload.Error())
	}
	return nil
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

func (r *sabStatusResponse) UnmarshalJSON(data []byte) error {
	var object struct {
		Status    any    `json:"status"`
		ErrorText string `json:"error"`
	}
	trimmed := strings.TrimSpace(string(data))
	if err := json.Unmarshal(data, &object); err == nil && (strings.HasPrefix(trimmed, "{") || object.Status != nil || object.ErrorText != "") {
		r.Status = object.Status
		r.ErrorText = object.ErrorText
		return nil
	}
	var boolValue bool
	if err := json.Unmarshal(data, &boolValue); err == nil {
		r.Status = boolValue
		return nil
	}
	var stringValue string
	if err := json.Unmarshal(data, &stringValue); err == nil {
		r.Status = stringValue
		return nil
	}
	var numberValue float64
	if err := json.Unmarshal(data, &numberValue); err == nil {
		r.Status = numberValue
		return nil
	}
	return nil
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

type sabCategoriesResponse struct {
	Categories []string `json:"categories"`
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
	SizeLeft   string `json:"sizeleft"`
	MB         string `json:"mb"`
	MBLeft     string `json:"mbleft"`
	MBMissing  string `json:"mbmissing"`
	TimeLeft   string `json:"timeleft"`
	TimeAdded  int64  `json:"time_added"`
	Priority   string `json:"priority"`
	Script     string `json:"script"`
	UnpackOpts string `json:"unpackopts"`
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

func (s sabQueueSlot) DownloadProperties() DownloadProperties {
	status := s.DownloadStatus(time.Now().UTC())
	return DownloadProperties{
		AdditionDate:    unixTime(s.TimeAdded),
		TotalSizeBytes:  status.SizeBytes,
		TotalDownloaded: status.DownloadedBytes,
		ETASeconds:      status.ETASeconds,
		Comment:         strings.TrimSpace(firstNonEmpty(s.Priority, s.Script, s.UnpackOpts)),
	}
}

type sabHistoryResponse struct {
	History struct {
		Slots []sabHistorySlot `json:"slots"`
	} `json:"history"`
}

type sabHistorySlot struct {
	NZOID        string `json:"nzo_id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Category     string `json:"category"`
	Size         string `json:"size"`
	Completed    int64  `json:"completed"`
	TimeAdded    int64  `json:"time_added"`
	FailMessage  string `json:"fail_message"`
	Storage      string `json:"storage"`
	Path         string `json:"path"`
	Downloaded   int64  `json:"downloaded"`
	Bytes        int64  `json:"bytes"`
	DownloadTime int64  `json:"download_time"`
	PostProcTime int64  `json:"postproc_time"`
	Script       string `json:"script"`
	URL          string `json:"url"`
}

func (s sabHistorySlot) DownloadStatus(now time.Time) DownloadStatus {
	progress := 1.0
	if strings.Contains(strings.ToLower(s.Status), "fail") {
		progress = 0
	}
	sizeBytes := firstPositiveInt64(s.Bytes, parseSABSize(s.Size))
	downloaded := firstPositiveInt64(s.Downloaded, sizeBytes)
	status := DownloadStatus{
		Client:          "SABnzbd",
		ID:              s.NZOID,
		Name:            firstNonEmpty(s.Name, s.NZOID),
		State:           normalizeSABState(s.Status),
		Progress:        progress,
		SavePath:        firstNonEmpty(s.Storage, s.Path),
		Category:        s.Category,
		Tags:            []string{"librarry"},
		SizeBytes:       sizeBytes,
		DownloadedBytes: downloaded,
		LastSeenAt:      &now,
		AddedAt:         unixTime(s.TimeAdded),
		CompletedAt:     unixTime(s.Completed),
		FailureReason:   strings.TrimSpace(s.FailMessage),
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

func (s sabHistorySlot) DownloadProperties() DownloadProperties {
	sizeBytes := firstPositiveInt64(s.Bytes, parseSABSize(s.Size))
	downloaded := firstPositiveInt64(s.Downloaded, sizeBytes)
	return DownloadProperties{
		SavePath:           firstNonEmpty(s.Storage, s.Path),
		AdditionDate:       unixTime(s.TimeAdded),
		CompletionDate:     unixTime(s.Completed),
		TotalSizeBytes:     sizeBytes,
		TotalDownloaded:    downloaded,
		TimeElapsedSeconds: s.DownloadTime,
		SeedingTimeSeconds: s.PostProcTime,
		Comment:            strings.TrimSpace(firstNonEmpty(s.FailMessage, s.Script, s.URL)),
	}
}

type sabFilesResponse struct {
	Files []sabFile `json:"files"`
}

type sabFile struct {
	NZFID    string `json:"nzf_id"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
	MB       string `json:"mb"`
	MBLeft   string `json:"mbleft"`
	Bytes    string `json:"bytes"`
	Set      string `json:"set"`
	Age      string `json:"age"`
}

func (f sabFile) DownloadFile(index int) DownloadFile {
	sizeBytes := parseSABFileBytes(f.Bytes)
	if sizeBytes == 0 {
		sizeBytes = parseSABMegabytes(f.MB)
	}
	leftBytes := parseSABMegabytes(f.MBLeft)
	progress := sabFileProgress(f.Status, sizeBytes, leftBytes)
	return DownloadFile{
		ID:         index,
		ExternalID: strings.TrimSpace(f.NZFID),
		Name:       strings.TrimSpace(firstNonEmpty(f.Filename, f.NZFID)),
		SizeBytes:  sizeBytes,
		Progress:   progress,
		Priority:   sabFilePriority(f.Status),
	}
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

func sabCategorySavePaths(payload any) map[string]string {
	paths := map[string]string{}
	collectSABCategorySavePaths(payload, "", paths)
	return paths
}

func collectSABCategorySavePaths(value any, hintedName string, paths map[string]string) {
	switch node := value.(type) {
	case []any:
		for _, item := range node {
			collectSABCategorySavePaths(item, "", paths)
		}
	case map[string]any:
		explicitName := strings.TrimSpace(sabConfigString(node["name"]))
		name := firstNonEmpty(explicitName, strings.TrimSpace(hintedName))
		if _, ok := node["dir"]; ok && name != "" {
			paths[name] = strings.TrimSpace(sabConfigString(node["dir"]))
		}
		for key, child := range node {
			switch key {
			case "categories", "config":
				collectSABCategorySavePaths(child, "", paths)
			case "name", "dir":
				continue
			default:
				collectSABCategorySavePaths(child, key, paths)
			}
		}
	}
}

func sabConfigString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
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

func parseSABFileBytes(value string) int64 {
	normalized := strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if normalized == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return int64(parsed)
}

func sabFileProgress(status string, sizeBytes int64, leftBytes int64) float64 {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "finished" {
		return 1
	}
	if sizeBytes <= 0 {
		return 0
	}
	if leftBytes <= 0 {
		if normalized == "active" {
			return 1
		}
		return 0
	}
	if leftBytes > sizeBytes {
		leftBytes = sizeBytes
	}
	return float64(sizeBytes-leftBytes) / float64(sizeBytes)
}

func sabFilePriority(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "finished", "active":
		return 1
	case "queued":
		return -1
	default:
		return 1
	}
}

func sabPriorityForAction(action string) string {
	switch action {
	case DownloadActionTopPriority:
		return "2"
	case DownloadActionIncreasePriority:
		return "1"
	case DownloadActionDecreasePriority, DownloadActionBottomPriority:
		return "-1"
	default:
		return "0"
	}
}
