package acquisition

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
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
	limit := query.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	queries := prowlarrSearchQueries(query)
	releases := make([]Release, 0, limit)
	seen := map[string]bool{}
	var firstErr error
	for _, queryText := range queries {
		found, err := c.searchOnce(ctx, queryText, query.Format, limit)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, release := range found {
			key := prowlarrReleaseKey(release)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			releases = append(releases, release)
			if len(releases) >= limit {
				return releases, nil
			}
		}
	}
	if len(releases) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return releases, nil
}

func (c *ProwlarrClient) searchOnce(ctx context.Context, queryText string, format string, limit int) ([]Release, error) {
	values := url.Values{}
	values.Set("query", strings.TrimSpace(queryText))
	values.Set("type", "search")
	values.Set("categories", categoriesForFormat(format))

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
	return decodeProwlarrReleases(resp.Body, limit)
}

func prowlarrSearchQueries(query ReleaseSearchQuery) []string {
	queries := make([]string, 0, 4)
	for _, isbn := range splitSearchISBNs(query.ISBN) {
		queries = appendUniqueSearchQuery(queries, isbn)
		if compact := compactISBN(isbn); compact != "" && !strings.EqualFold(compact, isbn) {
			queries = appendUniqueSearchQuery(queries, compact)
		}
	}
	queries = appendUniqueSearchQuery(queries, query.Query)
	if len(queries) == 0 && strings.TrimSpace(query.Author) != "" {
		queries = appendUniqueSearchQuery(queries, query.Author)
	}
	if len(queries) == 0 {
		queries = append(queries, "")
	}
	return queries
}

func splitSearchISBNs(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = appendUniqueSearchQuery(out, part)
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func appendUniqueSearchQuery(queries []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return queries
	}
	for _, existing := range queries {
		if strings.EqualFold(existing, value) {
			return queries
		}
	}
	return append(queries, value)
}

func compactISBN(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' || r == 'X' || r == 'x' {
			builder.WriteRune(r)
		}
	}
	return strings.ToUpper(builder.String())
}

func prowlarrReleaseKey(release Release) string {
	for _, value := range []string{release.InfoHash, release.DownloadURL, release.ID, release.Title} {
		value = strings.TrimSpace(value)
		if value != "" {
			return strings.ToLower(value)
		}
	}
	return ""
}

func (c *ProwlarrClient) Feed(ctx context.Context, query ReleaseFeedQuery) ([]Release, error) {
	if !c.Configured() {
		return nil, ErrIntegrationNotConfigured
	}
	limit := query.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	indexers, err := c.listRSSIndexers(ctx)
	if err != nil {
		return nil, err
	}
	if len(indexers) == 0 {
		return nil, fmt.Errorf("no rss-enabled Prowlarr indexers are configured")
	}

	var releases []Release
	var feedErrors []string
	for _, indexer := range indexers {
		found, err := c.fetchIndexerFeed(ctx, indexer, query, limit)
		if err != nil {
			feedErrors = append(feedErrors, fmt.Sprintf("%s: %v", indexer.Name, err))
			continue
		}
		releases = append(releases, found...)
	}
	if len(releases) == 0 && len(feedErrors) > 0 {
		return nil, fmt.Errorf("prowlarr feed failed: %s", strings.Join(feedErrors, "; "))
	}
	sortReleasesByPublishedAt(releases)
	if len(releases) > limit {
		releases = releases[:limit]
	}
	return releases, nil
}

type rawProwlarrRelease struct {
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

type rawProwlarrIndexer struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	EnableRSS *bool  `json:"enableRss"`
}

type torznabRSS struct {
	Channel torznabChannel `xml:"channel"`
}

type torznabChannel struct {
	Items []torznabItem `xml:"item"`
}

type torznabItem struct {
	GUID      string         `xml:"guid"`
	Title     string         `xml:"title"`
	Link      string         `xml:"link"`
	Comments  string         `xml:"comments"`
	PubDate   string         `xml:"pubDate"`
	Category  []string       `xml:"category"`
	Enclosure torznabClosure `xml:"enclosure"`
	Attrs     []torznabAttr  `xml:"attr"`
}

type torznabClosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type torznabAttr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func (c *ProwlarrClient) listRSSIndexers(ctx context.Context) ([]rawProwlarrIndexer, error) {
	req, err := c.request(ctx, http.MethodGet, "/api/v1/indexer", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("prowlarr indexer list returned %s", resp.Status)
	}
	var raw []rawProwlarrIndexer
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	indexers := make([]rawProwlarrIndexer, 0, len(raw))
	for _, indexer := range raw {
		if indexer.ID == 0 {
			continue
		}
		if indexer.EnableRSS != nil && !*indexer.EnableRSS {
			continue
		}
		if strings.TrimSpace(indexer.Name) == "" {
			indexer.Name = strconv.Itoa(indexer.ID)
		}
		indexers = append(indexers, indexer)
	}
	return indexers, nil
}

func (c *ProwlarrClient) fetchIndexerFeed(ctx context.Context, indexer rawProwlarrIndexer, query ReleaseFeedQuery, limit int) ([]Release, error) {
	values := url.Values{}
	values.Set("t", "search")
	if categories := categoriesForFormat(query.Format); categories != "" {
		values.Set("cat", categories)
	}
	endpoint := fmt.Sprintf("/%d/api?%s", indexer.ID, values.Encode())
	req, err := c.request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, */*")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("indexer feed returned %s", resp.Status)
	}
	return decodeTorznabFeed(resp.Body, indexer.Name, indexer.Protocol, limit)
}

func decodeProwlarrReleases(body io.Reader, limit int) ([]Release, error) {
	var raw []rawProwlarrRelease
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 200 {
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

func decodeTorznabFeed(body io.Reader, indexerName string, protocol string, limit int) ([]Release, error) {
	var feed torznabRSS
	if err := xml.NewDecoder(body).Decode(&feed); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	releases := make([]Release, 0, min(len(feed.Channel.Items), limit))
	for _, item := range feed.Channel.Items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		attrs := torznabAttrs(item.Attrs)
		infoHash := firstAttr(attrs, "infohash")
		downloadURL := firstNonEmpty(item.Enclosure.URL, firstAttr(attrs, "magneturl"), item.Link)
		infoURL := firstNonEmpty(item.Comments, item.GUID)
		size := item.Enclosure.Length
		if size == 0 {
			size = attrInt64(attrs, "size")
		}
		seeders := attrInt(attrs, "seeders")
		leechers := attrInt(attrs, "leechers")
		if leechers == 0 {
			peers := attrInt(attrs, "peers")
			if peers > seeders {
				leechers = peers - seeders
			}
		}
		categories := append([]string{}, item.Category...)
		categories = append(categories, attrs["category"]...)
		categories = append(categories, attrs["categoryid"]...)
		published := parseRSSDate(item.PubDate)
		releases = append(releases, Release{
			ID:          releaseID(infoHash, item.GUID, downloadURL, title),
			InfoHash:    infoHash,
			Indexer:     strings.TrimSpace(indexerName),
			Title:       title,
			SizeBytes:   size,
			Seeders:     seeders,
			Leechers:    leechers,
			DownloadURL: downloadURL,
			InfoURL:     infoURL,
			Protocol:    normalizeFeedProtocol(protocol, item.Enclosure.Type, downloadURL, infoHash),
			Categories:  compactStrings(categories),
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

func torznabAttrs(items []torznabAttr) map[string][]string {
	attrs := map[string][]string{}
	for _, item := range items {
		name := strings.ToLower(strings.TrimSpace(item.Name))
		value := strings.TrimSpace(item.Value)
		if name == "" || value == "" {
			continue
		}
		attrs[name] = append(attrs[name], value)
	}
	return attrs
}

func firstAttr(attrs map[string][]string, name string) string {
	values := attrs[strings.ToLower(strings.TrimSpace(name))]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func attrInt(attrs map[string][]string, name string) int {
	value, _ := strconv.Atoi(firstAttr(attrs, name))
	return value
}

func attrInt64(attrs map[string][]string, name string) int64 {
	value, _ := strconv.ParseInt(firstAttr(attrs, name), 10, 64)
	return value
}

func parseRSSDate(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC3339Nano} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func normalizeFeedProtocol(protocol string, contentType string, downloadURL string, infoHash string) string {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	switch protocol {
	case "torrent", "usenet":
		return protocol
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	downloadURL = strings.ToLower(strings.TrimSpace(downloadURL))
	if strings.Contains(contentType, "bittorrent") || strings.HasPrefix(downloadURL, "magnet:") || strings.TrimSpace(infoHash) != "" {
		return "torrent"
	}
	if strings.Contains(contentType, "nzb") {
		return "usenet"
	}
	if protocol != "" {
		return protocol
	}
	return "unknown"
}

func sortReleasesByPublishedAt(releases []Release) {
	for i := 1; i < len(releases); i++ {
		for j := i; j > 0; j-- {
			left := releases[j-1].PublishedAt
			right := releases[j].PublishedAt
			if right.IsZero() || (!left.IsZero() && !right.After(left)) {
				break
			}
			releases[j-1], releases[j] = releases[j], releases[j-1]
		}
	}
}

func categoriesForFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "audiobook", "audio":
		return "7000,3030"
	case "ebook", "book":
		return "7000"
	case "", "any":
		return "7000,3030"
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
