package calibre

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Settings struct {
	Host          string
	Port          int
	URLBase       string
	Username      string
	Password      string
	Library       string
	OutputFormat  string
	OutputProfile string
	UseSSL        bool
}

type AddBookRequest struct {
	Settings Settings
	Path     string
}

type AddBookResult struct {
	ID int `json:"id"`
}

type DeleteBooksRequest struct {
	Settings Settings
	IDs      []int
}

type SetFieldsRequest struct {
	Settings Settings
	ID       int
	Metadata Metadata
}

type ConvertRequest struct {
	Settings    Settings
	ID          int
	InputFormat string
}

type ConvertResult struct {
	Jobs    []ConvertJob `json:"jobs,omitempty"`
	Skipped []string     `json:"skipped,omitempty"`
}

type ConvertJob struct {
	OutputFormat string `json:"outputFormat"`
	JobID        int64  `json:"jobId"`
}

type PollConversionsRequest struct {
	Settings    Settings
	Jobs        []ConvertJob
	MaxAttempts int
	Interval    time.Duration
}

type ConversionStatus struct {
	OutputFormat string `json:"outputFormat,omitempty"`
	JobID        int64  `json:"jobId"`
	Running      bool   `json:"running"`
	OK           bool   `json:"ok"`
	WasAborted   bool   `json:"wasAborted"`
	Traceback    string `json:"traceback,omitempty"`
	Log          string `json:"log,omitempty"`
}

type Metadata struct {
	Title       string
	Authors     []string
	Publisher   string
	Languages   string
	Tags        []string
	Comments    string
	Identifiers map[string]string
	Series      string
	SeriesIndex *float64
}

type Importer interface {
	AddBook(ctx context.Context, request AddBookRequest) (AddBookResult, error)
}

type Manager interface {
	Importer
	DeleteBooks(ctx context.Context, request DeleteBooksRequest) error
	SetFields(ctx context.Context, request SetFieldsRequest) error
	Convert(ctx context.Context, request ConvertRequest) (ConvertResult, error)
	PollConversions(ctx context.Context, request PollConversionsRequest) ([]ConversionStatus, error)
}

type Client struct {
	httpClient *http.Client
	jobID      func() int
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{
		httpClient: httpClient,
		jobID: func() int {
			return int(time.Now().UTC().UnixNano() % 1_000_000_000)
		},
	}
}

func (c *Client) AddBook(ctx context.Context, request AddBookRequest) (AddBookResult, error) {
	if c == nil {
		return AddBookResult{}, errors.New("calibre client is unavailable")
	}
	settings := normalizeSettings(request.Settings)
	if settings.Host == "" {
		return AddBookResult{}, errors.New("calibre host is required")
	}
	filePath := filepath.Clean(strings.TrimSpace(request.Path))
	if filePath == "" || filePath == "." {
		return AddBookResult{}, errors.New("calibre import path is required")
	}
	body, err := os.ReadFile(filePath)
	if err != nil {
		return AddBookResult{}, err
	}
	endpoint, err := addBookURL(settings, c.jobID(), filepath.Ext(filePath))
	if err != nil {
		return AddBookResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return AddBookResult{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/octet-stream")
	if settings.Username != "" {
		req.SetBasicAuth(settings.Username, settings.Password)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return AddBookResult{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AddBookResult{}, fmt.Errorf("calibre add-book returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	var result AddBookResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return AddBookResult{}, err
	}
	if result.ID <= 0 {
		return AddBookResult{}, errors.New("calibre rejected duplicate or untracked book")
	}
	return result, nil
}

func (c *Client) DeleteBooks(ctx context.Context, request DeleteBooksRequest) error {
	if c == nil {
		return errors.New("calibre client is unavailable")
	}
	settings := normalizeSettings(request.Settings)
	if settings.Host == "" {
		return errors.New("calibre host is required")
	}
	ids := compactPositiveIDs(request.IDs)
	if len(ids) == 0 {
		return nil
	}
	endpoint, err := deleteBooksURL(settings, ids)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if settings.Username != "" {
		req.SetBasicAuth(settings.Username, settings.Password)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("calibre delete-books returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (c *Client) SetFields(ctx context.Context, request SetFieldsRequest) error {
	if c == nil {
		return errors.New("calibre client is unavailable")
	}
	settings := normalizeSettings(request.Settings)
	if settings.Host == "" {
		return errors.New("calibre host is required")
	}
	if request.ID <= 0 {
		return errors.New("calibre book id is required")
	}
	payload := setFieldsPayloadFromRequest(request)
	if !payload.Changes.hasChanges() {
		return nil
	}
	endpoint, err := setFieldsURL(settings, request.ID)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if settings.Username != "" {
		req.SetBasicAuth(settings.Username, settings.Password)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("calibre set-fields returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (c *Client) Convert(ctx context.Context, request ConvertRequest) (ConvertResult, error) {
	if c == nil {
		return ConvertResult{}, errors.New("calibre client is unavailable")
	}
	settings := normalizeSettings(request.Settings)
	if settings.Host == "" {
		return ConvertResult{}, errors.New("calibre host is required")
	}
	if request.ID <= 0 {
		return ConvertResult{}, errors.New("calibre book id is required")
	}
	targets := outputFormats(settings.OutputFormat)
	if len(targets) == 0 {
		return ConvertResult{}, nil
	}
	bookData, err := c.conversionBookData(ctx, settings, request.ID)
	if err != nil {
		return ConvertResult{}, err
	}
	inputFormat := normalizeFormat(request.InputFormat)
	if inputFormat == "" {
		inputFormat = normalizeFormat(bookData.ConversionOptions.InputFormat)
	}
	result := ConvertResult{}
	for _, target := range targets {
		if formatMatches(target, inputFormat) || formatInList(target, bookData.InputFormats) {
			result.Skipped = append(result.Skipped, target)
			continue
		}
		options := bookData.ConversionOptions
		options.InputFormat = inputFormat
		options.OutputFormat = target
		profile := normalizeOutputProfile(settings.OutputProfile)
		if profile != "" {
			options.Options.OutputProfile = profile
		}
		jobID, err := c.startConversion(ctx, settings, request.ID, options)
		if err != nil {
			return result, err
		}
		result.Jobs = append(result.Jobs, ConvertJob{OutputFormat: target, JobID: jobID})
	}
	return result, nil
}

func (c *Client) PollConversions(ctx context.Context, request PollConversionsRequest) ([]ConversionStatus, error) {
	if c == nil {
		return nil, errors.New("calibre client is unavailable")
	}
	settings := normalizeSettings(request.Settings)
	if settings.Host == "" {
		return nil, errors.New("calibre host is required")
	}
	jobs := compactConversionJobs(request.Jobs)
	if len(jobs) == 0 {
		return nil, nil
	}
	maxAttempts := request.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	statuses := make([]ConversionStatus, 0, len(jobs))
	for _, job := range jobs {
		var status ConversionStatus
		for attempt := 0; attempt < maxAttempts; attempt++ {
			latest, err := c.conversionStatus(ctx, settings, job)
			if err != nil {
				return statuses, err
			}
			status = latest
			if !status.Running {
				break
			}
			if request.Interval > 0 && attempt < maxAttempts-1 {
				timer := time.NewTimer(request.Interval)
				select {
				case <-ctx.Done():
					timer.Stop()
					return statuses, ctx.Err()
				case <-timer.C:
				}
			}
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func addBookURL(settings Settings, jobID int, ext string) (string, error) {
	base, err := baseURL(settings)
	if err != nil {
		return "", err
	}
	if jobID <= 0 {
		jobID = 1
	}
	filename := "$dummy" + strings.ToLower(ext)
	if filename == "$dummy" {
		filename = "$dummy.epub"
	}
	segments := []string{
		strings.Trim(base.Path, "/"),
		"cdb", "add-book", strconv.Itoa(jobID), "1", filename,
	}
	if settings.Library != "" {
		segments = append(segments, settings.Library)
	}
	base.Path = joinURLPath(segments...)
	base.RawQuery = ""
	return base.String(), nil
}

func deleteBooksURL(settings Settings, ids []int) (string, error) {
	base, err := baseURL(settings)
	if err != nil {
		return "", err
	}
	var idStrings []string
	for _, id := range compactPositiveIDs(ids) {
		idStrings = append(idStrings, strconv.Itoa(id))
	}
	segments := []string{
		strings.Trim(base.Path, "/"),
		"cdb", "delete-books", strings.Join(idStrings, ","),
	}
	if settings.Library != "" {
		segments = append(segments, settings.Library)
	}
	base.Path = joinURLPath(segments...)
	base.RawQuery = ""
	return base.String(), nil
}

func setFieldsURL(settings Settings, id int) (string, error) {
	base, err := baseURL(settings)
	if err != nil {
		return "", err
	}
	if id <= 0 {
		id = 1
	}
	segments := []string{
		strings.Trim(base.Path, "/"),
		"cdb", "set-fields", strconv.Itoa(id),
	}
	if settings.Library != "" {
		segments = append(segments, settings.Library)
	}
	base.Path = joinURLPath(segments...)
	base.RawQuery = ""
	return base.String(), nil
}

func conversionBookDataURL(settings Settings, id int) (string, error) {
	base, err := baseURL(settings)
	if err != nil {
		return "", err
	}
	if id <= 0 {
		id = 1
	}
	base.Path = joinURLPath(strings.Trim(base.Path, "/"), "conversion", "book-data", strconv.Itoa(id))
	values := base.Query()
	if settings.Library != "" {
		values.Set("library_id", settings.Library)
	}
	base.RawQuery = values.Encode()
	return base.String(), nil
}

func conversionStartURL(settings Settings, id int) (string, error) {
	base, err := baseURL(settings)
	if err != nil {
		return "", err
	}
	if id <= 0 {
		id = 1
	}
	base.Path = joinURLPath(strings.Trim(base.Path, "/"), "conversion", "start", strconv.Itoa(id))
	values := base.Query()
	if settings.Library != "" {
		values.Set("library_id", settings.Library)
	}
	base.RawQuery = values.Encode()
	return base.String(), nil
}

func conversionStatusURL(settings Settings, jobID int64) (string, error) {
	base, err := baseURL(settings)
	if err != nil {
		return "", err
	}
	if jobID <= 0 {
		jobID = 1
	}
	base.Path = joinURLPath(strings.Trim(base.Path, "/"), "conversion", "status", strconv.FormatInt(jobID, 10))
	values := base.Query()
	if settings.Library != "" {
		values.Set("library_id", settings.Library)
	}
	base.RawQuery = values.Encode()
	return base.String(), nil
}

func baseURL(settings Settings) (*url.URL, error) {
	host := strings.TrimSpace(settings.Host)
	if host == "" {
		return nil, errors.New("calibre host is required")
	}
	if !strings.Contains(host, "://") {
		scheme := "http"
		if settings.UseSSL {
			scheme = "https"
		}
		host = scheme + "://" + host
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "http"
		if settings.UseSSL {
			parsed.Scheme = "https"
		}
	}
	if parsed.Host == "" {
		return nil, errors.New("calibre host is required")
	}
	if settings.Port > 0 && parsed.Port() == "" {
		parsed.Host = net.JoinHostPort(parsed.Hostname(), strconv.Itoa(settings.Port))
	}
	parsed.Path = joinURLPath(parsed.Path, settings.URLBase)
	return parsed, nil
}

func normalizeSettings(settings Settings) Settings {
	settings.Host = strings.TrimSpace(settings.Host)
	settings.URLBase = strings.TrimSpace(settings.URLBase)
	settings.Username = strings.TrimSpace(settings.Username)
	settings.Library = strings.Trim(strings.TrimSpace(settings.Library), "/")
	settings.OutputFormat = strings.TrimSpace(settings.OutputFormat)
	settings.OutputProfile = strings.TrimSpace(settings.OutputProfile)
	if settings.Port <= 0 {
		settings.Port = 8080
	}
	return settings
}

func (c *Client) conversionBookData(ctx context.Context, settings Settings, id int) (conversionBookData, error) {
	endpoint, err := conversionBookDataURL(settings, id)
	if err != nil {
		return conversionBookData{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return conversionBookData{}, err
	}
	req.Header.Set("Accept", "application/json")
	if settings.Username != "" {
		req.SetBasicAuth(settings.Username, settings.Password)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return conversionBookData{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return conversionBookData{}, fmt.Errorf("calibre conversion book-data returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	var data conversionBookData
	if err := json.Unmarshal(respBody, &data); err != nil {
		return conversionBookData{}, err
	}
	return data, nil
}

func (c *Client) startConversion(ctx context.Context, settings Settings, id int, options conversionOptions) (int64, error) {
	endpoint, err := conversionStartURL(settings, id)
	if err != nil {
		return 0, err
	}
	body, err := json.Marshal(options)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if settings.Username != "" {
		req.SetBasicAuth(settings.Username, settings.Password)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("calibre conversion start returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	var jobID int64
	if err := json.Unmarshal(respBody, &jobID); err != nil {
		return 0, err
	}
	return jobID, nil
}

func (c *Client) conversionStatus(ctx context.Context, settings Settings, job ConvertJob) (ConversionStatus, error) {
	endpoint, err := conversionStatusURL(settings, job.JobID)
	if err != nil {
		return ConversionStatus{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ConversionStatus{}, err
	}
	req.Header.Set("Accept", "application/json")
	if settings.Username != "" {
		req.SetBasicAuth(settings.Username, settings.Password)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ConversionStatus{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ConversionStatus{}, fmt.Errorf("calibre conversion status returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	var status conversionStatusResponse
	if err := json.Unmarshal(respBody, &status); err != nil {
		return ConversionStatus{}, err
	}
	return ConversionStatus{
		OutputFormat: job.OutputFormat,
		JobID:        job.JobID,
		Running:      status.Running,
		OK:           status.OK,
		WasAborted:   status.WasAborted,
		Traceback:    strings.TrimSpace(status.Traceback),
		Log:          strings.TrimSpace(status.Log),
	}, nil
}

func compactPositiveIDs(ids []int) []int {
	seen := map[int]bool{}
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func compactConversionJobs(jobs []ConvertJob) []ConvertJob {
	result := make([]ConvertJob, 0, len(jobs))
	seen := map[int64]bool{}
	for _, job := range jobs {
		if job.JobID <= 0 || seen[job.JobID] {
			continue
		}
		seen[job.JobID] = true
		job.OutputFormat = strings.TrimSpace(job.OutputFormat)
		result = append(result, job)
	}
	return result
}

func outputFormats(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';'
	})
	seen := map[string]bool{}
	var result []string
	for _, part := range parts {
		format := normalizeFormat(part)
		if format == "" || seen[format] {
			continue
		}
		seen[format] = true
		result = append(result, format)
	}
	return result
}

func normalizeFormat(value string) string {
	return strings.ToUpper(strings.Trim(strings.TrimSpace(value), "."))
}

func normalizeOutputProfile(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" || strings.EqualFold(value, "default") {
		return ""
	}
	return value
}

func formatMatches(left string, right string) bool {
	return normalizeFormat(left) != "" && normalizeFormat(left) == normalizeFormat(right)
}

func formatInList(format string, values []string) bool {
	for _, value := range values {
		if formatMatches(format, value) {
			return true
		}
	}
	return false
}

type setFieldsPayload struct {
	Changes       setFieldsChanges `json:"changes"`
	LoadedBookIDs []int            `json:"loaded_book_ids"`
}

type setFieldsChanges struct {
	Title       string            `json:"title,omitempty"`
	Authors     []string          `json:"authors,omitempty"`
	Publisher   string            `json:"publisher,omitempty"`
	Languages   string            `json:"languages,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Comments    string            `json:"comments,omitempty"`
	Identifiers map[string]string `json:"identifiers,omitempty"`
	Series      string            `json:"series,omitempty"`
	SeriesIndex *float64          `json:"series_index,omitempty"`
}

func setFieldsPayloadFromRequest(request SetFieldsRequest) setFieldsPayload {
	metadata := request.Metadata
	return setFieldsPayload{
		LoadedBookIDs: []int{request.ID},
		Changes: setFieldsChanges{
			Title:       strings.TrimSpace(metadata.Title),
			Authors:     compactNonEmptyStrings(metadata.Authors),
			Publisher:   strings.TrimSpace(metadata.Publisher),
			Languages:   strings.TrimSpace(metadata.Languages),
			Tags:        compactNonEmptyStrings(metadata.Tags),
			Comments:    strings.TrimSpace(metadata.Comments),
			Identifiers: compactStringMap(metadata.Identifiers),
			Series:      strings.TrimSpace(metadata.Series),
			SeriesIndex: metadata.SeriesIndex,
		},
	}
}

func (c setFieldsChanges) hasChanges() bool {
	return c.Title != "" || len(c.Authors) > 0 || c.Publisher != "" || c.Languages != "" ||
		len(c.Tags) > 0 || c.Comments != "" || len(c.Identifiers) > 0 || c.Series != "" || c.SeriesIndex != nil
}

func compactNonEmptyStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func compactStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := map[string]string{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

type conversionBookData struct {
	ConversionOptions conversionOptions `json:"conversion_options"`
	BookID            int               `json:"book_id"`
	InputFormats      []string          `json:"input_formats"`
	OutputFormats     []string          `json:"output_formats"`
}

type conversionOptions struct {
	Options      conversionOptionSettings `json:"options"`
	InputFormat  string                   `json:"input_fmt,omitempty"`
	OutputFormat string                   `json:"output_fmt,omitempty"`
}

type conversionOptionSettings struct {
	OutputProfile string `json:"output_profile,omitempty"`
}

type conversionStatusResponse struct {
	Running    bool   `json:"running"`
	OK         bool   `json:"ok"`
	WasAborted bool   `json:"was_aborted"`
	Traceback  string `json:"traceback"`
	Log        string `json:"log"`
}

func joinURLPath(parts ...string) string {
	var clean []string
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part != "" {
			clean = append(clean, part)
		}
	}
	if len(clean) == 0 {
		return ""
	}
	return "/" + path.Join(clean...)
}
