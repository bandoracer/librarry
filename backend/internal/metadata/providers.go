package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ProviderConfig struct {
	HardcoverToken string
	GoogleAPIKey   string
	HTTPTimeout    time.Duration
}

func DefaultProviders(cfg ProviderConfig) []Provider {
	timeout := cfg.HTTPTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	return []Provider{
		NewHardcoverProvider(client, cfg.HardcoverToken),
		NewOpenLibraryProvider(client),
		NewGoogleBooksProvider(client, cfg.GoogleAPIKey),
		NewLocalOPFProvider(),
	}
}

type HardcoverProvider struct {
	client *http.Client
	token  string
}

func NewHardcoverProvider(client *http.Client, token string) *HardcoverProvider {
	return &HardcoverProvider{client: client, token: strings.TrimSpace(token)}
}

func (p *HardcoverProvider) Name() string { return "Hardcover" }

func (p *HardcoverProvider) Health(ctx Context) ProviderHealth {
	if p.token == "" {
		return health(p.Name(), "missing_credentials", false, "Set LIBRARRY_HARDCOVER_TOKEN to enable rich metadata.")
	}
	return health(p.Name(), "ready", true, "Token configured; requests are rate-limited by the backend.")
}

func (p *HardcoverProvider) Diagnostics(ctx Context) Diagnostic {
	return Diagnostic{
		Name:       p.Name(),
		Configured: p.token != "",
		Capabilities: []string{
			"book search",
			"author search",
			"series and edition enrichment",
			"ebook/audiobook metadata",
		},
		Notes: []string{"Primary rich metadata provider. Token stays server-side."},
	}
}

func (p *HardcoverProvider) Search(ctx Context, query Query) ([]SearchResult, error) {
	if p.token == "" {
		return nil, nil
	}
	if query.Type == SearchTypeAuthor {
		return nil, nil
	}

	payload := map[string]any{
		"query": `query SearchBooks($query: String!, $limit: Int!) {
			search(query: $query, query_type: "Book", per_page: $limit, page: 1) {
				ids
				results
			}
		}`,
		"variables": map[string]any{
			"query": query.Query,
			"limit": clampLimit(query.Limit),
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(asContext(ctx), http.MethodPost, "https://api.hardcover.app/v1/graphql", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "librarry/0.1")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("hardcover search returned %s", resp.Status)
	}

	var decoded struct {
		Data struct {
			Search struct {
				Results []map[string]any `json:"results"`
			} `json:"search"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(decoded.Data.Search.Results))
	for _, raw := range decoded.Data.Search.Results {
		title := stringValue(raw["title"])
		if title == "" {
			continue
		}
		authorName := ""
		if contributors, ok := raw["contributions"].([]any); ok && len(contributors) > 0 {
			if first, ok := contributors[0].(map[string]any); ok {
				authorName = stringValue(first["author_name"])
			}
		}
		id := fmt.Sprintf("hardcover:%v", raw["id"])
		result := SearchResult{
			Provider: p.Name(),
			Kind:     SearchTypeBook,
			Work: Work{
				ID:    id,
				Title: title,
				Authors: []Author{{
					ID:   stableID("hardcover-author", authorName),
					Name: authorName,
				}},
				CoverURL: stringValue(raw["image_url"]),
				ProviderIDs: []string{
					id,
				},
			},
			Score:        scoreResult(query, title, authorName, nil),
			Confidence:   confidence(scoreResult(query, title, authorName, nil)),
			MatchedOn:    []string{"hardcover search"},
			RawSourceKey: id,
		}
		results = append(results, result)
	}
	return results, nil
}

type OpenLibraryProvider struct {
	client *http.Client
}

func NewOpenLibraryProvider(client *http.Client) *OpenLibraryProvider {
	return &OpenLibraryProvider{client: client}
}

func (p *OpenLibraryProvider) Name() string { return "Open Library" }

func (p *OpenLibraryProvider) Health(ctx Context) ProviderHealth {
	return health(p.Name(), "ready", true, "Open API configured as the open-data backbone.")
}

func (p *OpenLibraryProvider) Diagnostics(ctx Context) Diagnostic {
	return Diagnostic{
		Name:       p.Name(),
		Configured: true,
		Capabilities: []string{
			"book search",
			"author search",
			"work and edition identifiers",
			"cover images",
		},
		Notes: []string{"Used as the open-data backbone and fallback identity source."},
	}
}

func (p *OpenLibraryProvider) Search(ctx Context, query Query) ([]SearchResult, error) {
	if strings.TrimSpace(query.Query) == "" {
		return nil, errors.New("query is required")
	}
	switch query.Type {
	case SearchTypeAuthor:
		return p.searchAuthors(ctx, query)
	case SearchTypeAuthorWorks:
		if authorKey := openLibraryAuthorKey(query.ProviderKey); authorKey != "" {
			return p.searchAuthorWorks(ctx, query, authorKey)
		}
	}
	return p.searchBooks(ctx, query)
}

func (p *OpenLibraryProvider) searchAuthors(ctx Context, query Query) ([]SearchResult, error) {
	values := url.Values{}
	values.Set("q", query.Query)
	values.Set("limit", strconv.Itoa(clampLimit(query.Limit)))
	req, err := http.NewRequestWithContext(asContext(ctx), http.MethodGet, "https://openlibrary.org/search/authors.json?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "librarry/0.1")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("open library author search returned %s", resp.Status)
	}

	var decoded struct {
		Docs []struct {
			Key       string `json:"key"`
			Name      string `json:"name"`
			TopWork   string `json:"top_work"`
			WorkCount int    `json:"work_count"`
		} `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(decoded.Docs))
	for _, doc := range decoded.Docs {
		authorKey := strings.TrimPrefix(strings.TrimSpace(doc.Key), "/authors/")
		if authorKey == "" || strings.TrimSpace(doc.Name) == "" {
			continue
		}
		authorID := "openlibrary:" + authorKey
		score := authorIdentityScore(query, doc.Name)
		results = append(results, SearchResult{
			Provider: p.Name(),
			Kind:     SearchTypeAuthor,
			Work: Work{
				ID:          authorID,
				Title:       doc.Name,
				Description: firstNonEmpty(doc.TopWork, fmt.Sprintf("%d Open Library works", doc.WorkCount)),
				CoverURL:    openLibraryAuthorCoverURL(authorKey),
				Authors: []Author{{
					ID:          authorID,
					Name:        doc.Name,
					ProviderIDs: []string{"/authors/" + authorKey},
				}},
				ProviderIDs: []string{authorID, "/authors/" + authorKey},
			},
			Score:        score,
			Confidence:   confidence(score),
			MatchedOn:    authorMatchedOn(query, doc.Name),
			RawSourceKey: "/authors/" + authorKey,
		})
	}
	return results, nil
}

func (p *OpenLibraryProvider) searchAuthorWorks(ctx Context, query Query, authorKey string) ([]SearchResult, error) {
	values := url.Values{}
	values.Set("limit", strconv.Itoa(clampLimit(query.Limit)))
	req, err := http.NewRequestWithContext(asContext(ctx), http.MethodGet, "https://openlibrary.org/authors/"+url.PathEscape(authorKey)+"/works.json?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "librarry/0.1")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("open library author works returned %s", resp.Status)
	}

	var decoded struct {
		Entries []struct {
			Key              string `json:"key"`
			Title            string `json:"title"`
			FirstPublishDate string `json:"first_publish_date"`
			Covers           []int  `json:"covers"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}

	authorID := "openlibrary:" + authorKey
	results := make([]SearchResult, 0, len(decoded.Entries))
	for _, entry := range decoded.Entries {
		title := strings.TrimSpace(entry.Title)
		if title == "" {
			continue
		}
		workID := "openlibrary:" + strings.TrimPrefix(strings.TrimSpace(entry.Key), "/works/")
		publishedYear := yearFromOpenLibraryDate(entry.FirstPublishDate)
		score := authorWorkScore(query)
		results = append(results, SearchResult{
			Provider: p.Name(),
			Kind:     SearchTypeBook,
			Work: Work{
				ID:               workID,
				Title:            title,
				FirstPublishYear: publishedYear,
				Authors: []Author{{
					ID:          authorID,
					Name:        query.Query,
					ProviderIDs: []string{"/authors/" + authorKey},
				}},
				CoverURL:    openLibraryCoverURL(firstInt(entry.Covers)),
				ProviderIDs: []string{workID},
			},
			Edition: Edition{
				ID:            workID + ":edition",
				WorkID:        workID,
				Title:         title,
				Format:        inferFormat(query.Format, nil),
				PublishedDate: entry.FirstPublishDate,
			},
			Score:        score,
			Confidence:   confidence(score),
			MatchedOn:    []string{"open_library_author_works"},
			RawSourceKey: entry.Key,
		})
	}
	return results, nil
}

func (p *OpenLibraryProvider) searchBooks(ctx Context, query Query) ([]SearchResult, error) {
	endpoint := "https://openlibrary.org/search.json"
	values := url.Values{}
	if isbn := normalizeISBN(query.Query); isbn != "" {
		values.Set("isbn", isbn)
	} else if query.Type == SearchTypeAuthorWorks {
		values.Set("author", query.Query)
	} else {
		values.Set("title", query.Query)
	}
	values.Set("limit", strconv.Itoa(clampLimit(query.Limit)))
	values.Set("fields", "key,title,author_name,author_key,first_publish_year,isbn,edition_key,language,cover_i")
	req, err := http.NewRequestWithContext(asContext(ctx), http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "librarry/0.1")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("open library search returned %s", resp.Status)
	}

	var decoded struct {
		Docs []struct {
			Key              string   `json:"key"`
			Title            string   `json:"title"`
			AuthorName       []string `json:"author_name"`
			AuthorKey        []string `json:"author_key"`
			FirstPublishYear int      `json:"first_publish_year"`
			ISBN             []string `json:"isbn"`
			EditionKey       []string `json:"edition_key"`
			Language         []string `json:"language"`
			CoverID          int      `json:"cover_i"`
		} `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(decoded.Docs))
	for _, doc := range decoded.Docs {
		author := first(doc.AuthorName)
		authorID := first(doc.AuthorKey)
		isbns := compactStrings(doc.ISBN)
		workID := "openlibrary:" + strings.TrimPrefix(doc.Key, "/works/")
		editionID := ""
		if len(doc.EditionKey) > 0 {
			editionID = "openlibrary:" + doc.EditionKey[0]
		}
		format := inferFormat(query.Format, isbns)
		score := scoreResult(query, doc.Title, author, isbns)
		if query.Type == SearchTypeAuthorWorks && normalize(author) == normalize(query.Query) {
			score = authorWorkScore(query)
		}
		result := SearchResult{
			Provider: p.Name(),
			Kind:     SearchTypeBook,
			Work: Work{
				ID:               workID,
				Title:            doc.Title,
				FirstPublishYear: doc.FirstPublishYear,
				Authors: []Author{{
					ID:   "openlibrary:" + authorID,
					Name: author,
				}},
				CoverURL:    openLibraryCoverURL(doc.CoverID),
				ProviderIDs: []string{workID},
			},
			Edition: Edition{
				ID:          editionID,
				WorkID:      workID,
				Title:       doc.Title,
				Format:      format,
				Language:    first(doc.Language),
				ISBNs:       isbns,
				ProviderIDs: compactStrings([]string{editionID}),
			},
			Score:        score,
			Confidence:   confidence(score),
			MatchedOn:    matchedOn(query, doc.Title, author, isbns),
			RawSourceKey: doc.Key,
		}
		results = append(results, result)
	}
	return results, nil
}

type GoogleBooksProvider struct {
	client *http.Client
	apiKey string
}

func NewGoogleBooksProvider(client *http.Client, apiKey string) *GoogleBooksProvider {
	return &GoogleBooksProvider{client: client, apiKey: strings.TrimSpace(apiKey)}
}

func (p *GoogleBooksProvider) Name() string { return "Google Books" }

func (p *GoogleBooksProvider) Health(ctx Context) ProviderHealth {
	if p.apiKey == "" {
		return health(p.Name(), "missing_credentials", false, "Set LIBRARRY_GOOGLE_BOOKS_API_KEY for exact fallback lookup.")
	}
	return health(p.Name(), "ready", true, "Configured as exact ISBN/title fallback only.")
}

func (p *GoogleBooksProvider) Diagnostics(ctx Context) Diagnostic {
	return Diagnostic{
		Name:       p.Name(),
		Configured: p.apiKey != "",
		Capabilities: []string{
			"exact ISBN fallback",
			"title fallback",
			"cover and publisher hints",
		},
		Notes: []string{"Not used for author bibliography crawling."},
	}
}

func (p *GoogleBooksProvider) Search(ctx Context, query Query) ([]SearchResult, error) {
	if p.apiKey == "" {
		return nil, nil
	}
	if query.Type == SearchTypeAuthor {
		return nil, nil
	}
	values := url.Values{}
	if query.Type == SearchTypeAuthorWorks {
		values.Set("q", "inauthor:"+query.Query)
	} else {
		values.Set("q", query.Query)
	}
	values.Set("maxResults", strconv.Itoa(clampLimit(query.Limit)))
	values.Set("projection", "lite")
	values.Set("key", p.apiKey)

	req, err := http.NewRequestWithContext(asContext(ctx), http.MethodGet, "https://www.googleapis.com/books/v1/volumes?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "librarry/0.1")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("google books search returned %s", resp.Status)
	}

	var decoded struct {
		Items []struct {
			ID         string `json:"id"`
			VolumeInfo struct {
				Title               string   `json:"title"`
				Authors             []string `json:"authors"`
				PublishedDate       string   `json:"publishedDate"`
				Publisher           string   `json:"publisher"`
				PageCount           int      `json:"pageCount"`
				IndustryIdentifiers []struct {
					Type       string `json:"type"`
					Identifier string `json:"identifier"`
				} `json:"industryIdentifiers"`
				ImageLinks struct {
					Thumbnail string `json:"thumbnail"`
				} `json:"imageLinks"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(decoded.Items))
	for _, item := range decoded.Items {
		var isbns []string
		for _, identifier := range item.VolumeInfo.IndustryIdentifiers {
			if strings.HasPrefix(identifier.Type, "ISBN") {
				isbns = append(isbns, identifier.Identifier)
			}
		}
		author := first(item.VolumeInfo.Authors)
		score := scoreResult(query, item.VolumeInfo.Title, author, isbns)
		workID := "googlebooks:" + item.ID
		results = append(results, SearchResult{
			Provider: p.Name(),
			Kind:     SearchTypeBook,
			Work: Work{
				ID:       workID,
				Title:    item.VolumeInfo.Title,
				CoverURL: item.VolumeInfo.ImageLinks.Thumbnail,
				Authors: []Author{{
					ID:   stableID("googlebooks-author", author),
					Name: author,
				}},
				ProviderIDs: []string{workID},
			},
			Edition: Edition{
				ID:            workID + ":edition",
				WorkID:        workID,
				Title:         item.VolumeInfo.Title,
				Format:        inferFormat(query.Format, isbns),
				ISBNs:         compactStrings(isbns),
				Publisher:     item.VolumeInfo.Publisher,
				PublishedDate: item.VolumeInfo.PublishedDate,
				Pages:         item.VolumeInfo.PageCount,
				ProviderIDs:   []string{workID + ":edition"},
			},
			Score:        score,
			Confidence:   confidence(score),
			MatchedOn:    matchedOn(query, item.VolumeInfo.Title, author, isbns),
			RawSourceKey: item.ID,
		})
	}
	return results, nil
}

type LocalOPFProvider struct{}

func NewLocalOPFProvider() *LocalOPFProvider { return &LocalOPFProvider{} }

func (p *LocalOPFProvider) Name() string { return "Local OPF" }

func (p *LocalOPFProvider) Health(ctx Context) ProviderHealth {
	return health(p.Name(), "ready", true, "Local embedded and sidecar metadata will be used during imports.")
}

func (p *LocalOPFProvider) Diagnostics(ctx Context) Diagnostic {
	return Diagnostic{
		Name:       p.Name(),
		Configured: true,
		Capabilities: []string{
			"metadata.opf parsing",
			"embedded EPUB metadata",
			"audio tag import evidence",
		},
		Notes: []string{"Search returns no remote results; this provider participates in import matching."},
	}
}

func (p *LocalOPFProvider) Search(ctx Context, query Query) ([]SearchResult, error) {
	return nil, nil
}

func health(name string, status string, configured bool, message string) ProviderHealth {
	return ProviderHealth{
		Name:       name,
		Status:     status,
		Configured: configured,
		Message:    message,
		CheckedAt:  time.Now().UTC(),
	}
}
