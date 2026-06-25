package acquisition

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ProwlarrClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewProwlarrClient(baseURL string, apiKey string, client *http.Client) *ProwlarrClient {
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	return &ProwlarrClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  strings.TrimSpace(apiKey),
		client:  client,
	}
}

func (c *ProwlarrClient) Name() string { return "Prowlarr" }

func (c *ProwlarrClient) Configured() bool {
	return c.baseURL != "" && c.apiKey != ""
}

func (c *ProwlarrClient) Health(ctx context.Context) IntegrationHealth {
	if !c.Configured() {
		return IntegrationHealth{Name: c.Name(), Configured: false, Status: "missing_credentials", Message: "Set LIBRARRY_PROWLARR_URL and LIBRARRY_PROWLARR_API_KEY."}
	}

	req, err := c.request(ctx, http.MethodGet, "/api/v1/system/status", nil)
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
		Version string `json:"version"`
		AppName string `json:"appName"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&decoded)
	message := "Ready"
	if decoded.Version != "" {
		message = fmt.Sprintf("%s %s ready", decoded.AppName, decoded.Version)
	}
	return IntegrationHealth{Name: c.Name(), Configured: true, Status: "ready", Message: message}
}

func (c *ProwlarrClient) Search(ctx context.Context, query ReleaseSearchQuery) ([]Release, error) {
	if !c.Configured() {
		return nil, ErrIntegrationNotConfigured
	}
	values := url.Values{}
	values.Set("query", strings.TrimSpace(query.Query))
	values.Set("type", "search")
	values.Set("categories", categoriesForFormat(query.Format))

	endpoint := "/api/v1/search?" + values.Encode()
	req, err := c.request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("prowlarr search returned %s", resp.Status)
	}

	var raw []struct {
		GUID        string `json:"guid"`
		InfoHash    string `json:"infoHash"`
		Indexer     string `json:"indexer"`
		Title       string `json:"title"`
		Size        int64  `json:"size"`
		Seeders     int    `json:"seeders"`
		Leechers    int    `json:"leechers"`
		MagnetURL   string `json:"magnetUrl"`
		DownloadURL string `json:"downloadUrl"`
		InfoURL     string `json:"infoUrl"`
		Protocol    string `json:"protocol"`
		PublishDate string `json:"publishDate"`
		Categories  []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"categories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	limit := query.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	releases := make([]Release, 0, min(len(raw), limit))
	for _, item := range raw {
		downloadURL := item.DownloadURL
		if downloadURL == "" {
			downloadURL = item.MagnetURL
		}
		if downloadURL == "" {
			downloadURL = item.GUID
		}
		var published time.Time
		if item.PublishDate != "" {
			published, _ = time.Parse(time.RFC3339, item.PublishDate)
		}
		categories := make([]string, 0, len(item.Categories))
		for _, category := range item.Categories {
			if category.Name != "" {
				categories = append(categories, category.Name)
			} else if category.ID != 0 {
				categories = append(categories, strconv.Itoa(category.ID))
			}
		}
		releases = append(releases, Release{
			ID:          releaseID(item.InfoHash, item.GUID, item.Title),
			InfoHash:    item.InfoHash,
			Indexer:     item.Indexer,
			Title:       item.Title,
			SizeBytes:   item.Size,
			Seeders:     item.Seeders,
			Leechers:    item.Leechers,
			DownloadURL: downloadURL,
			InfoURL:     item.InfoURL,
			Protocol:    item.Protocol,
			Categories:  categories,
			PublishedAt: published,
		})
		if len(releases) >= limit {
			break
		}
	}
	return releases, nil
}

func (c *ProwlarrClient) request(ctx context.Context, method string, endpoint string, body any) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "librarry/0.1")
	return req, nil
}

func categoriesForFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "audiobook", "audio":
		return "7000,3030"
	case "ebook", "book", "":
		return "7000"
	default:
		return "7000"
	}
}

func releaseID(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			sum := sha1.Sum([]byte(value))
			return hex.EncodeToString(sum[:8])
		}
	}
	return ""
}
