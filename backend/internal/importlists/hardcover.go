package importlists

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/providerhttp"
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
		client = providerhttp.NewClient(15 * time.Second)
	}
	clone := *client
	if clone.Timeout <= 0 || clone.Timeout > 30*time.Second {
		clone.Timeout = 15 * time.Second
	}
	client = &clone
	token = strings.TrimSpace(token)
	if len(token) >= 7 && strings.EqualFold(token[:7], "Bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return &HardcoverClient{client: client, token: token, url: hardcoverGraphQLURL}
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

// FetchList resolves the complete visible list before returning any entries.
// pageSize is a request batch size, never a total catalog limit.
func (c *HardcoverClient) FetchList(ctx context.Context, settings map[string]string, pageSize int) ([]Entry, error) {
	if !c.Configured() {
		return nil, errors.New("hardcover token is not configured (set LIBRARRY_HARDCOVER_TOKEN)")
	}
	listID := strings.TrimSpace(settings["listId"])
	if listID == "" {
		listID = strings.TrimSpace(settings["listID"])
	}
	numericID, err := strconv.ParseInt(listID, 10, 32)
	if err != nil || numericID <= 0 {
		return nil, errors.New("import list settings.listId must be a positive numeric Hardcover list id")
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 200
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	entries := make([]Entry, 0)
	seenBooks := map[string]bool{}
	cursor, rowsSeen, expectedCount := int64(0), 0, -1
	var updatedAt string
	for page := 0; page < 101; page++ {
		var data struct {
			List *struct {
				ID        int64           `json:"id"`
				Count     *int            `json:"books_count"`
				UpdatedAt json.RawMessage `json:"updated_at"`
				Rows      []struct {
					ID     int64          `json:"id"`
					ListID int64          `json:"list_id"`
					BookID int64          `json:"book_id"`
					Book   map[string]any `json:"book"`
				} `json:"list_books"`
			} `json:"lists_by_pk"`
		}
		err := c.query(ctx, `query LibrarryListBooks($listId: Int!, $after: Int!, $limit: Int!) {
			lists_by_pk(id: $listId) {
				id books_count updated_at
				list_books(where: {id: {_gt: $after}}, order_by: {id: asc}, limit: $limit) {
					id list_id book_id
					book { id title release_date cached_image cached_contributors }
				}
			}
		}`, map[string]any{"listId": numericID, "after": cursor, "limit": pageSize}, &data)
		if err != nil {
			return nil, err
		}
		list := data.List
		if list == nil || list.ID != numericID {
			return nil, errors.New("Hardcover list is unavailable; verify its ID and visibility to this token")
		}
		if list.Count == nil || *list.Count < 0 || *list.Count > 10000 || len(list.UpdatedAt) == 0 || list.Rows == nil {
			return nil, errors.New("Hardcover list completeness could not be verified (maximum 10,000 entries)")
		}
		var stamp *string
		if json.Unmarshal(list.UpdatedAt, &stamp) != nil {
			return nil, errors.New("Hardcover list returned an invalid modification timestamp")
		}
		// updated_at is nullable in Hardcover's schema. Compare known values
		// without treating a legitimate null as missing response data.
		stampValue := ""
		if stamp != nil {
			stampValue = *stamp
		}
		if expectedCount < 0 {
			expectedCount, updatedAt = *list.Count, stampValue
		}
		if expectedCount != *list.Count || updatedAt != stampValue {
			return nil, errors.New("Hardcover list changed during traversal; retry the sync")
		}
		if len(list.Rows) == 0 {
			if rowsSeen != expectedCount {
				return nil, errors.New("Hardcover list ended before its declared entry count; retry the sync")
			}
			return entries, nil
		}
		if len(list.Rows) > pageSize || rowsSeen+len(list.Rows) > expectedCount {
			return nil, errors.New("Hardcover list returned inconsistent pagination")
		}
		for _, row := range list.Rows {
			if row.ID <= cursor || row.ID > 2147483647 || row.ListID != numericID || row.BookID <= 0 || row.BookID > 2147483647 {
				return nil, errors.New("Hardcover list returned an invalid or repeated entry identity")
			}
			entry, ok := entryFromHardcoverBook(row.Book)
			if !ok || entry.SourceKey != "hardcover:"+strconv.FormatInt(row.BookID, 10) {
				return nil, errors.New("Hardcover list contains unavailable or invalid book metadata")
			}
			cursor = row.ID
			rowsSeen++
			if !seenBooks[entry.SourceKey] {
				seenBooks[entry.SourceKey] = true
				entries = append(entries, entry)
			}
		}
	}
	return nil, errors.New("Hardcover list traversal exceeded 101 pages; retry with a larger page size")
}

func (c *HardcoverClient) query(ctx context.Context, query string, variables map[string]any, data any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return errors.New("Hardcover list endpoint is invalid")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "librarry/0.1")
	resp, err := c.client.Do(req)
	if err != nil {
		var notSent *providerhttp.NotSentError
		if errors.As(err, &notSent) {
			return notSent
		}
		return errors.New("Hardcover list request failed; check the connection and request budget")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Hardcover list request returned HTTP %d", resp.StatusCode)
	}
	var decoded struct {
		Data   json.RawMessage   `json:"data"`
		Errors []json.RawMessage `json:"errors"`
	}
	// Bound response memory independently of the declared row count.
	body, err = io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil || len(body) > 4<<20 || json.Unmarshal(body, &decoded) != nil {
		return errors.New("Hardcover list response could not be decoded within its size limit")
	}
	if len(decoded.Errors) > 0 {
		return errors.New("Hardcover rejected the list query; check API access and list permissions")
	}
	decoder := json.NewDecoder(bytes.NewReader(decoded.Data))
	decoder.UseNumber()
	if decoder.Decode(data) != nil {
		return errors.New("Hardcover list response is missing valid data")
	}
	return nil
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
