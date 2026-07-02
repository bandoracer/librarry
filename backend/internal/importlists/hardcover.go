package importlists

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const hardcoverGraphQLURL = "https://api.hardcover.app/v1/graphql"

// HardcoverClient fetches a user list/shelf from the Hardcover GraphQL API
// using the same token/endpoint plumbing as the metadata provider.
type HardcoverClient struct {
	client *http.Client
	token  string
	url    string
}

func NewHardcoverClient(client *http.Client, token string) *HardcoverClient {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &HardcoverClient{client: client, token: strings.TrimSpace(token), url: hardcoverGraphQLURL}
}

// WithURL overrides the GraphQL endpoint (tests).
func (c *HardcoverClient) WithURL(url string) *HardcoverClient {
	if strings.TrimSpace(url) != "" {
		c.url = strings.TrimSpace(url)
	}
	return c
}

func (c *HardcoverClient) Configured() bool {
	return c != nil && c.token != ""
}

// FetchList resolves settings.listId into list entries.
func (c *HardcoverClient) FetchList(ctx context.Context, settings map[string]string, limit int) ([]Entry, error) {
	if !c.Configured() {
		return nil, errors.New("hardcover token is not configured (set LIBRARRY_HARDCOVER_TOKEN)")
	}
	listID := strings.TrimSpace(settings["listId"])
	if listID == "" {
		listID = strings.TrimSpace(settings["listID"])
	}
	if listID == "" {
		return nil, errors.New("import list settings.listId is required")
	}
	numericID, err := strconv.Atoi(listID)
	if err != nil {
		return nil, fmt.Errorf("import list settings.listId must be a numeric Hardcover list id: %q", listID)
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	payload := map[string]any{
		"query": `query LibrarryListBooks($listId: Int!, $limit: Int!) {
			list_books(where: {list_id: {_eq: $listId}}, limit: $limit) {
				book {
					id
					title
					release_date
					cached_image
					cached_contributors
				}
			}
		}`,
		"variables": map[string]any{
			"listId": numericID,
			"limit":  limit,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "librarry/0.1")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("hardcover list fetch returned %s", resp.Status)
	}

	var decoded struct {
		Data struct {
			ListBooks []struct {
				Book map[string]any `json:"book"`
			} `json:"list_books"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if len(decoded.Errors) > 0 {
		return nil, fmt.Errorf("hardcover list fetch failed: %s", decoded.Errors[0].Message)
	}

	entries := make([]Entry, 0, len(decoded.Data.ListBooks))
	for _, row := range decoded.Data.ListBooks {
		entry, ok := entryFromHardcoverBook(row.Book)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// entryFromHardcoverBook maps one raw Hardcover book record onto an Entry,
// tolerating schema drift in the cached json columns.
func entryFromHardcoverBook(book map[string]any) (Entry, bool) {
	if book == nil {
		return Entry{}, false
	}
	title := stringValue(book["title"])
	if title == "" {
		return Entry{}, false
	}
	id := stringValue(book["id"])
	if id == "" {
		return Entry{}, false
	}
	return Entry{
		SourceKey:   "hardcover:" + id,
		Title:       title,
		AuthorName:  hardcoverAuthorName(book["cached_contributors"]),
		CoverURL:    hardcoverImageURL(book["cached_image"]),
		ReleaseDate: stringValue(book["release_date"]),
	}, true
}

func hardcoverAuthorName(raw any) string {
	switch value := raw.(type) {
	case []any:
		for _, item := range value {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if author, ok := entry["author"].(map[string]any); ok {
				if name := stringValue(author["name"]); name != "" {
					return name
				}
			}
			if name := stringValue(entry["name"]); name != "" {
				return name
			}
			if name := stringValue(entry["author_name"]); name != "" {
				return name
			}
		}
	case map[string]any:
		if author, ok := value["author"].(map[string]any); ok {
			return stringValue(author["name"])
		}
		return stringValue(value["name"])
	case string:
		return strings.TrimSpace(value)
	}
	return ""
}

func hardcoverImageURL(raw any) string {
	switch value := raw.(type) {
	case map[string]any:
		return stringValue(value["url"])
	case string:
		return strings.TrimSpace(value)
	}
	return ""
}

func stringValue(raw any) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		if value == float64(int64(value)) {
			return strconv.FormatInt(int64(value), 10)
		}
		return strconv.FormatFloat(value, 'f', -1, 64)
	case json.Number:
		return value.String()
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	}
	return ""
}
