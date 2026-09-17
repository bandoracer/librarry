package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (p *OpenLibraryProvider) readJSON(ctx Context, path string, query url.Values, target any, validate func() error) (err error) {
	finish, err := p.observation.begin(ctx, false)
	if err != nil {
		return err
	}
	defer func() { err = finish(err) }()
	req, err := http.NewRequestWithContext(asContext(ctx), http.MethodGet, "https://openlibrary.org"+path+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "librarry/0.1")
	resp, err := providerRequest(p.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}
	return validate()
}

func (p *OpenLibraryProvider) Bibliography(ctx Context, query Query) ([]SearchResult, error) {
	bounded, cancel := context.WithTimeout(asContext(ctx), 2*time.Minute)
	defer cancel()
	ctx = bounded
	id := openLibraryAuthorKey(query.ProviderKey)
	if id == "" {
		return nil, errors.New("Open Library bibliography requires a stable Open Library author ID")
	}
	var author struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	}
	if err := p.readJSON(ctx, "/authors/"+id+".json", nil, &author, func() error {
		if author.Key != "/authors/"+id || strings.TrimSpace(author.Name) == "" {
			return providerValidationError("Open Library author identity was not verified")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	results := []SearchResult{}
	seen := map[string]bool{}
	total := -1
	for offset := 0; ; {
		var page struct {
			Size    *int `json:"size"`
			Entries []struct {
				Key              string `json:"key"`
				Title            string `json:"title"`
				FirstPublishDate string `json:"first_publish_date"`
				Covers           []int  `json:"covers"`
			} `json:"entries"`
		}
		err := p.readJSON(ctx, "/authors/"+id+"/works.json", url.Values{"limit": {strconv.Itoa(bibliographyPageSize)}, "offset": {strconv.Itoa(offset)}}, &page, func() error {
			if page.Size == nil || *page.Size < 0 || page.Entries == nil {
				return providerValidationError("Open Library bibliography is missing its count or entries")
			}
			if *page.Size > bibliographyMaxRecords {
				return providerValidationError("Open Library bibliography exceeds the traversal safety limit; no partial bibliography was applied")
			}
			if total >= 0 && total != *page.Size {
				return providerValidationError("Open Library bibliography changed during pagination; retry the full traversal")
			}
			if len(page.Entries) == 0 && offset < *page.Size {
				return providerValidationError("Open Library bibliography ended before its declared count")
			}
			if offset+len(page.Entries) > *page.Size {
				return providerValidationError("Open Library bibliography exceeds its declared count")
			}
			for _, entry := range page.Entries {
				if !validOpenLibraryWorkKey(entry.Key) || strings.TrimSpace(entry.Title) == "" || seen[entry.Key] {
					return providerValidationError("Open Library bibliography returned invalid or duplicate work identities")
				}
				seen[entry.Key] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		total = *page.Size
		for _, entry := range page.Entries {
			key := "openlibrary:" + strings.TrimPrefix(entry.Key, "/works/")
			authorKey := "openlibrary:" + id
			results = append(results, SearchResult{Provider: p.Name(), Kind: SearchTypeBook,
				Work:    Work{ID: key, Title: entry.Title, FirstPublishYear: yearFromOpenLibraryDate(entry.FirstPublishDate), FirstPublishDate: entry.FirstPublishDate, CoverURL: openLibraryCoverURL(firstInt(entry.Covers)), Authors: []Author{{ID: authorKey, Name: author.Name, ProviderIDs: []string{authorKey, author.Key}}}, ProviderIDs: []string{key, entry.Key}},
				Edition: Edition{WorkID: key, Title: entry.Title, Format: FormatAny},
				Score:   0.95, Confidence: "high", MatchedOn: []string{"open_library_author_works"}, RawSourceKey: entry.Key})
		}
		offset += len(page.Entries)
		if offset == total {
			return results, nil
		}
	}
}
func validOpenLibraryWorkKey(key string) bool {
	value := strings.TrimPrefix(key, "/works/OL")
	if value == key || len(value) < 2 || !strings.HasSuffix(value, "W") {
		return false
	}
	for _, r := range value[:len(value)-1] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
