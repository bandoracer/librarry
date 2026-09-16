package metadata

import (
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
	observation *providerObservation
	client      *http.Client
	token       string
}

func NewHardcoverProvider(client *http.Client, token string) *HardcoverProvider {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return &HardcoverProvider{client: boundedProviderClient(client), token: token, observation: newProviderObservation("Hardcover", true)}
}

func (p *HardcoverProvider) Name() string { return "Hardcover" }

func (p *HardcoverProvider) Health(ctx Context) ProviderHealth {
	if p.token == "" {
		return p.observation.health(false, "Set LIBRARRY_HARDCOVER_TOKEN to enable rich metadata.")
	}
	return p.observation.health(true, "Token configured; connection has not been checked.")
}

func (p *HardcoverProvider) Diagnostics(ctx Context) Diagnostic {
	return Diagnostic{
		Name:       p.Name(),
		Configured: p.token != "",
		Capabilities: []string{
			"book search",
			"author identity search",
			"paginated author bibliography",
			"work publication metadata",
		},
		Notes: []string{"Primary rich metadata provider. Token stays server-side."},
	}
}

func (p *HardcoverProvider) Search(ctx Context, query Query) (results []SearchResult, err error) {
	if p.token == "" {
		return nil, nil
	}
	if query.Type == SearchTypeAuthor {
		return p.searchAuthors(ctx, query)
	}
	if query.Type == SearchTypeAuthorWorks {
		if hardcoverAuthorID(query.ProviderKey) == 0 {
			return nil, nil
		}
		return p.Bibliography(ctx, query)
	}
	if query.Type == SearchTypeSeries {
		return nil, nil
	}
	var decoded struct {
		Search struct {
			Results json.RawMessage `json:"results"`
		} `json:"search"`
	}
	var rawResults []map[string]any
	if err := p.graphQL(ctx, `query SearchBooks($query: String!, $limit: Int!) {
		search(query: $query, query_type: "Book", per_page: $limit, page: 1) { ids results }
	}`, map[string]any{"query": query.Query, "limit": clampLimit(query.Limit)}, &decoded, func() error {
		var parseErr error
		rawResults, parseErr = hardcoverSearchDocuments(decoded.Search.Results)
		if parseErr != nil {
			return parseErr
		}
		for _, doc := range rawResults {
			if hardcoverDocumentID(doc["id"]) == 0 || strings.TrimSpace(stringValue(doc["title"])) == "" {
				return providerValidationError("Hardcover book result lacks a stable identity or title")
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	results = make([]SearchResult, 0, len(rawResults))
	for _, raw := range rawResults {
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
		if names, ok := raw["author_names"].([]any); authorName == "" && ok && len(names) > 0 {
			authorName = stringValue(names[0])
		}

		id := fmt.Sprintf("hardcover:%d", hardcoverDocumentID(raw["id"]))
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
	observation *providerObservation
	client      *http.Client
}

func NewOpenLibraryProvider(client *http.Client) *OpenLibraryProvider {
	return &OpenLibraryProvider{client: boundedProviderClient(client), observation: newProviderObservation("Open Library", false)}
}

func (p *OpenLibraryProvider) Name() string { return "Open Library" }

func (p *OpenLibraryProvider) Health(ctx Context) ProviderHealth {
	return p.observation.health(true, "Open API configured; connection has not been checked.")
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

func (p *OpenLibraryProvider) Search(ctx Context, query Query) (results []SearchResult, err error) {
	if strings.TrimSpace(query.Query) == "" {
		return nil, errors.New("query is required")
	}
	finish, err := p.observation.begin(ctx, false)
	if err != nil {
		return nil, err
	}
	defer func() { err = finish(err) }()
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

	resp, err := providerRequest(p.client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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

	if decoded.Docs == nil {
		return nil, errors.New("missing provider result list")
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

	resp, err := providerRequest(p.client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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

	if decoded.Entries == nil {
		return nil, errors.New("missing provider result list")
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

	resp, err := providerRequest(p.client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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

	if decoded.Docs == nil {
		return nil, errors.New("missing provider result list")
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
	observation *providerObservation
	client      *http.Client
	apiKey      string
}

func NewGoogleBooksProvider(client *http.Client, apiKey string) *GoogleBooksProvider {
	return &GoogleBooksProvider{client: boundedProviderClient(client), apiKey: strings.TrimSpace(apiKey), observation: newProviderObservation("Google Books", true)}
}

func (p *GoogleBooksProvider) Name() string { return "Google Books" }

func (p *GoogleBooksProvider) Health(ctx Context) ProviderHealth {
	if p.apiKey == "" {
		return p.observation.health(false, "Set LIBRARRY_GOOGLE_BOOKS_API_KEY for exact fallback lookup.")
	}
	return p.observation.health(true, "API key configured; connection has not been checked.")
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
	if _, eligible := exactBookLookup(query); !eligible {
		return nil, nil
	}
	finish, err := p.observation.begin(ctx, false)
	if err != nil {
		return nil, err
	}
	results, err := p.searchBooks(ctx, query)
	return results, finish(err)
}
func (p *GoogleBooksProvider) searchBooks(ctx Context, query Query) ([]SearchResult, error) {
	values := url.Values{}
	lookup, eligible := exactBookLookup(query)
	if !eligible {
		return []SearchResult{}, nil
	}
	if lookup.isbn != "" {
		values.Set("q", "isbn:"+lookup.isbn)
	} else {
		values.Set("q", "intitle:"+strconv.Quote(strings.TrimSpace(query.Query)))
	}
	values.Set("maxResults", strconv.Itoa(clampLimit(query.Limit)))
	values.Set("projection", "full")
	values.Set("printType", "books")
	values.Set("fields", "totalItems,items(id,volumeInfo(title,subtitle,authors,publishedDate,publisher,pageCount,industryIdentifiers,imageLinks/thumbnail,language),saleInfo/isEbook)")
	values.Set("key", p.apiKey)

	req, err := http.NewRequestWithContext(asContext(ctx), http.MethodGet, "https://www.googleapis.com/books/v1/volumes?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "librarry/0.1")

	resp, err := providerRequest(p.client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var decoded struct {
		TotalItems *int `json:"totalItems"`
		Items      []struct {
			ID       string `json:"id"`
			SaleInfo struct {
				IsEbook bool `json:"isEbook"`
			} `json:"saleInfo"`
			VolumeInfo struct {
				Title               string   `json:"title"`
				Subtitle            string   `json:"subtitle"`
				Language            string   `json:"language"`
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

	if decoded.TotalItems == nil || *decoded.TotalItems < 0 || (*decoded.TotalItems > 0 && decoded.Items == nil) {
		return nil, errors.New("missing provider result count or list")
	}

	results := make([]SearchResult, 0, len(decoded.Items))
	for _, item := range decoded.Items {
		var isbns []string
		for _, identifier := range item.VolumeInfo.IndustryIdentifiers {
			if strings.HasPrefix(identifier.Type, "ISBN") {
				isbns = append(isbns, identifier.Identifier)
			}
		}
		title := strings.TrimSpace(item.VolumeInfo.Title)
		if subtitle := strings.TrimSpace(item.VolumeInfo.Subtitle); subtitle != "" {
			title += ": " + subtitle
		}
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.VolumeInfo.Title) == "" {
			continue
		}
		format := FormatAny
		if item.SaleInfo.IsEbook {
			format = FormatEbook
		}
		authors := []Author{}
		for _, name := range compactStrings(item.VolumeInfo.Authors) {
			authors = append(authors, Author{ID: stableID("googlebooks-author", name), Name: name})
		}
		author := first(item.VolumeInfo.Authors)
		score := scoreResult(query, title, author, isbns)
		workID := "googlebooks:" + item.ID
		result := SearchResult{
			Provider: p.Name(),
			Kind:     SearchTypeBook,
			Work: Work{
				ID:          workID,
				Title:       title,
				CoverURL:    item.VolumeInfo.ImageLinks.Thumbnail,
				Authors:     authors,
				ProviderIDs: []string{workID},
			},
			Edition: Edition{
				ID:            workID + ":edition",
				WorkID:        workID,
				Title:         title,
				Format:        format,
				Language:      item.VolumeInfo.Language,
				ISBNs:         compactStrings(isbns),
				Publisher:     item.VolumeInfo.Publisher,
				PublishedDate: item.VolumeInfo.PublishedDate,
				Pages:         item.VolumeInfo.PageCount,
				ProviderIDs:   []string{workID + ":edition"},
			},
			Score:        score,
			Confidence:   confidence(score),
			MatchedOn:    matchedOn(query, title, author, isbns),
			RawSourceKey: item.ID,
		}
		if !lookup.matches(result) || !resultFitsQuery(query, result) {
			continue
		}
		if lookup.isbn != "" {
			result.Score = 0.99
			result.Confidence = confidence(result.Score)
			result.MatchedOn = []string{"exact ISBN fallback"}
		} else {
			result.MatchedOn = append(result.MatchedOn, "exact title fallback")
		}
		results = append(results, result)
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

// Hardcover returns Typesense search JSON; retain array support for older
// recorded responses while rejecting malformed/missing data as provider errors.
func hardcoverSearchDocuments(raw json.RawMessage) ([]map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("hardcover search response is missing results")
	}
	var list []map[string]any
	if raw[0] == '[' {
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		return list, nil
	}
	var envelope struct {
		Hits *[]struct {
			Document map[string]any `json:"document"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("invalid hardcover results: %w", err)
	}
	if envelope.Hits == nil {
		return nil, fmt.Errorf("hardcover search response is missing hits")
	}
	for _, hit := range *envelope.Hits {
		if hit.Document == nil {
			return nil, fmt.Errorf("hardcover search hit is missing document")
		}
		list = append(list, hit.Document)
	}
	return list, nil
}
