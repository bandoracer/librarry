package metadata

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Search returns works and a nested, query-matching edition. The work-level
// language, ISBN and edition-key arrays must never be zipped into an edition.
type openLibrarySearchDocument struct {
	Key              string   `json:"key"`
	Title            string   `json:"title"`
	Subtitle         string   `json:"subtitle"`
	AuthorName       []string `json:"author_name"`
	AuthorKey        []string `json:"author_key"`
	FirstPublishYear int      `json:"first_publish_year"`
	Language         []string `json:"language"`
	CoverID          int      `json:"cover_i"`
	Editions         struct {
		Docs []openLibrarySearchEdition `json:"docs"`
	} `json:"editions"`
}

type openLibrarySearchEdition struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Language    []string `json:"language"`
	ISBN        []string `json:"isbn"`
	CoverID     int      `json:"cover_i"`
	Publisher   []string `json:"publisher"`
	PublishDate []string `json:"publish_date"`
}

const openLibrarySearchFields = "key,title,subtitle,author_name,author_key,first_publish_year,language,cover_i,editions,editions.key,editions.title,editions.language,editions.isbn,editions.cover_i,editions.publisher,editions.publish_date"

func (p *OpenLibraryProvider) searchBooks(ctx Context, query Query) ([]SearchResult, error) {
	results, err := p.searchBookBranch(ctx, query, false)
	if err != nil {
		return results, err
	}
	if _, _, explicit := explicitTitleAuthor(query.Query); !explicit || query.Type != SearchTypeBook {
		return results, nil
	}
	for _, result := range results {
		if discoveryAuthorAgreement(query, result) && resultFitsQuery(query, result) {
			return results, nil
		}
	}
	rescued, err := p.searchBookBranch(ctx, query, true)
	// Preserve useful primary results on rescue failure; the service exposes the
	// failure and avoids caching the partial response as a complete success.
	if err != nil {
		return results, &partialSearchError{err}
	}
	seen := map[string]bool{}
	for _, result := range results {
		seen[result.Work.ID+"|"+result.Edition.ID] = true
	}
	for _, result := range rescued {
		key := result.Work.ID + "|" + result.Edition.ID
		if !seen[key] {
			results = append(results, result)
			seen[key] = true
		}
	}
	return results, nil
}

func (p *OpenLibraryProvider) searchBookBranch(ctx Context, query Query, structured bool) ([]SearchResult, error) {
	values := url.Values{}
	lookup, _ := exactBookLookup(query)
	if structured {
		title, author, _ := explicitTitleAuthor(query.Query)
		// q includes translated edition titles; title is limited to work titles.
		values.Set("q", literalOpenLibraryQuery(title))
		values.Set("author", literalOpenLibraryQuery(author))
	} else if lookup.isbn != "" {
		values.Set("isbn", lookup.isbn)
	} else if query.Type == SearchTypeAuthorWorks {
		values.Set("author", query.Query)
	} else {
		values.Set("q", literalOpenLibraryQuery(query.Query))
	}
	// Fetch beyond the displayed page so language filtering does not starve it.
	// Each branch is bounded; the caller permits at most one structured rescue.
	limit := 25
	if lookup.isbn != "" {
		limit = clampLimit(query.Limit)
	}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("fields", openLibrarySearchFields)
	if language := openLibrarySearchLanguage(query.PreferredLanguage); language != "" {
		values.Set("lang", language)
	}
	req, err := http.NewRequestWithContext(asContext(ctx), http.MethodGet, "https://openlibrary.org/search.json?"+values.Encode(), nil)
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
		Docs []openLibrarySearchDocument `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if decoded.Docs == nil {
		return nil, errors.New("missing provider result list")
	}
	if len(decoded.Docs) > limit {
		return nil, errors.New("Open Library search exceeded its requested result bound")
	}
	results := make([]SearchResult, 0, len(decoded.Docs))
	for _, doc := range decoded.Docs {
		workKey := openLibraryEntityKey(doc.Key, 'W')
		if workKey == "" || strings.TrimSpace(doc.Title) == "" {
			continue
		}
		workID := "openlibrary:" + workKey
		result := SearchResult{Provider: p.Name(), Kind: SearchTypeBook, RawSourceKey: doc.Key,
			Work: Work{ID: workID, Title: doc.Title, Subtitle: doc.Subtitle,
				FirstPublishYear: doc.FirstPublishYear, Languages: compactStrings(doc.Language),
				CoverURL: openLibraryCoverURL(doc.CoverID), ProviderIDs: []string{workID}},
			Edition: Edition{Format: FormatAny},
		}
		for index, name := range doc.AuthorName {
			if strings.TrimSpace(name) == "" {
				continue
			}
			author := Author{Name: name}
			if index < len(doc.AuthorKey) {
				if key := openLibraryAuthorKey(doc.AuthorKey[index]); key != "" {
					author.ID = "openlibrary:" + key
					author.ProviderIDs = []string{author.ID}
				}
			}
			result.Work.Authors = append(result.Work.Authors, author)
		}
		for _, edition := range doc.Editions.Docs {
			key := openLibraryEntityKey(edition.Key, 'M')
			if key == "" {
				continue
			}
			candidate := Edition{ID: "openlibrary:" + key, WorkID: workID, Title: edition.Title,
				Format: FormatAny, Language: selectedEditionLanguage(edition.Language, query.PreferredLanguage),
				ISBNs: compactStrings(edition.ISBN), CoverURL: openLibraryCoverURL(edition.CoverID),
				Publisher: strings.Join(compactStrings(edition.Publisher), "; "), PublishedDate: first(edition.PublishDate),
				ProviderIDs: []string{"openlibrary:" + key}}
			if lookup.isbn != "" && !lookup.matches(SearchResult{Kind: SearchTypeBook, Edition: candidate}) {
				continue
			}
			if result.Edition.ID == "" {
				result.Edition = candidate
			}
			if languageMatchesPreference(candidate.Language, query.PreferredLanguage) {
				result.Edition = candidate
				break
			}
		}
		// An ISBN on a work is insufficient to claim an exact edition match.
		if lookup.isbn != "" && !lookup.matches(result) {
			continue
		}
		result.Score = scoreResult(query, doc.Title, firstAuthorName(result), result.Edition.ISBNs)
		result.Confidence = confidence(result.Score)
		result.MatchedOn = matchedOn(query, doc.Title, firstAuthorName(result), result.Edition.ISBNs)
		results = append(results, result)
	}
	return results, nil
}

func openLibraryEntityKey(value string, kind byte) string {
	key := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(value, "openlibrary:"), "/works/"), "/books/")
	if len(key) < 4 || !strings.HasPrefix(key, "OL") || key[len(key)-1] != kind {
		return ""
	}
	if _, err := strconv.ParseUint(key[2:len(key)-1], 10, 64); err != nil {
		return ""
	}
	return key
}

func selectedEditionLanguage(languages []string, preferred string) string {
	for _, language := range languages {
		if strings.TrimSpace(language) != "" && languageMatchesPreference(language, preferred) {
			return language
		}
	}
	return first(languages)
}

func openLibrarySearchLanguage(preferred string) string {
	code := map[string]string{"english": "en", "french": "fr", "german": "de", "spanish": "es", "italian": "it", "dutch": "nl", "japanese": "ja", "chinese": "zh", "korean": "ko", "portuguese": "pt", "russian": "ru", "polish": "pl", "swedish": "sv", "danish": "da", "norwegian": "no"}
	return code[normalizeLanguageName(preferred)]
}

func literalOpenLibraryQuery(value string) string {
	// URL encoding alone does not escape the upstream query DSL. Preserve words
	// and Unicode, but do not let title punctuation inject field/operator syntax.
	return strings.NewReplacer("\\", " ", ":", " ", "\"", " ", "+", " ", "-", " ", "(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ", "^", " ", "~", " ", "*", " ", "?", " ", "!", " ", "|", " ", "&", " ").Replace(strings.ToLower(value))
}

// Only explicitly marked secondary-search failures may retain partial results.
// A provider's generic error can carry invalid or unvalidated records.
type partialSearchError struct{ err error }

func (e *partialSearchError) Error() string { return e.err.Error() }
func (e *partialSearchError) Unwrap() error { return e.err }
