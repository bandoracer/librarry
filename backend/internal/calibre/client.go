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

type Importer interface {
	AddBook(ctx context.Context, request AddBookRequest) (AddBookResult, error)
}

type Manager interface {
	Importer
	DeleteBooks(ctx context.Context, request DeleteBooksRequest) error
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
