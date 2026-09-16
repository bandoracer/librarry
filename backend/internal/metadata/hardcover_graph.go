package metadata

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

func hardcoverAuthorID(key string) int64 {
	if !strings.HasPrefix(key, "hardcover-author:") {
		return 0
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(key, "hardcover-author:"), 10, 32)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

func (p *HardcoverProvider) graphQL(ctx Context, query string, variables map[string]any, target any, validate ...func() error) (err error) {
	finish, err := p.observation.begin(ctx, false)
	if err != nil {
		return err
	}
	defer func() { err = finish(err) }()
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(asContext(ctx), http.MethodPost, "https://api.hardcover.app/v1/graphql", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "librarry/0.1")
	resp, err := providerRequest(p.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope struct {
		Data   json.RawMessage         `json:"data"`
		Errors []hardcoverGraphQLError `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if err := hardcoverErrors(envelope.Errors); err != nil {
		return err
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return errors.New("Hardcover response has no data")
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return err
	}
	for _, check := range validate {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

func (p *HardcoverProvider) searchAuthors(ctx Context, query Query) ([]SearchResult, error) {
	var decoded struct {
		Search struct {
			Results json.RawMessage `json:"results"`
		} `json:"search"`
	}
	results := []SearchResult{}
	err := p.graphQL(ctx, `query SearchAuthors($query: String!, $limit: Int!) {
		search(query:$query, query_type:"Author", per_page:$limit, page:1) { results }
	}`, map[string]any{"query": query.Query, "limit": clampLimit(query.Limit)}, &decoded, func() error {
		docs, err := hardcoverSearchDocuments(decoded.Search.Results)
		if err != nil {
			return err
		}
		for _, doc := range docs {
			id := hardcoverDocumentID(doc["id"])
			name := strings.TrimSpace(stringValue(doc["name"]))
			if id <= 0 || name == "" {
				return providerValidationError("Hardcover author result lacks a stable identity")
			}
			key := fmt.Sprintf("hardcover-author:%d", id)
			cover := ""
			if image, ok := doc["image"].(map[string]any); ok {
				cover = stringValue(image["url"])
			}
			score := authorIdentityScore(query, name)
			results = append(results, SearchResult{Provider: p.Name(), Kind: SearchTypeAuthor,
				Work:  Work{ID: key, Title: name, CoverURL: cover, Authors: []Author{{ID: key, Name: name, ProviderIDs: []string{key}}}, ProviderIDs: []string{key}},
				Score: score, Confidence: confidence(score), MatchedOn: authorMatchedOn(query, name), RawSourceKey: strconv.FormatInt(id, 10)})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

type hardcoverGraphBook struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ReleaseYear int    `json:"release_year"`
	ReleaseDate string `json:"release_date"`
	Image       struct {
		URL string `json:"url"`
	} `json:"image"`
	Contributions []struct {
		Contribution string `json:"contribution"`
		Author       struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"author"`
	} `json:"contributions"`
}

func (p *HardcoverProvider) Bibliography(ctx Context, query Query) ([]SearchResult, error) {
	bounded, cancel := context.WithTimeout(asContext(ctx), 2*time.Minute)
	defer cancel()
	ctx = bounded
	id := hardcoverAuthorID(query.ProviderKey)
	if id == 0 {
		return nil, errors.New("Hardcover bibliography requires a stable Hardcover author ID")
	}
	if p.token == "" {
		return nil, errors.New("Hardcover bibliography requires a configured token")
	}
	var identity struct {
		Authors []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"authors"`
	}
	if err := p.graphQL(ctx, `query AuthorIdentity($id:Int!) { authors(where:{id:{_eq:$id}},limit:1) { id name } }`, map[string]any{"id": id}, &identity, func() error {
		if len(identity.Authors) != 1 || identity.Authors[0].ID != id || strings.TrimSpace(identity.Authors[0].Name) == "" {
			return providerValidationError("Hardcover author identity was not verified")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	results := []SearchResult{}
	var after int64
	for {
		var data struct {
			Books []hardcoverGraphBook `json:"books"`
		}
		err := p.graphQL(ctx, `query AuthorBooks($author:Int!, $after:Int!, $limit:Int!) {
			books(where:{id:{_gt:$after},contributions:{author_id:{_eq:$author},contributable_type:{_eq:"Book"}}},order_by:{id:asc},limit:$limit) {
				id title description release_year release_date image { url }
				contributions(where:{contributable_type:{_eq:"Book"}}) { contribution author { id name } }
			}
		}`, map[string]any{"author": id, "after": after, "limit": bibliographyPageSize}, &data, func() error {
			if data.Books == nil {
				return providerValidationError("Hardcover bibliography is missing its book list")
			}
			for _, book := range data.Books {
				if book.ID <= after || strings.TrimSpace(book.Title) == "" {
					return providerValidationError("Hardcover bibliography returned invalid or nonadvancing book identities")
				}
				after = book.ID
				authors := []Author{}
				matched := false
				for _, credit := range book.Contributions {
					if credit.Author.ID <= 0 || strings.TrimSpace(credit.Author.Name) == "" {
						return providerValidationError("Hardcover bibliography returned an invalid contributor")
					}
					key := fmt.Sprintf("hardcover-author:%d", credit.Author.ID)
					authors = append(authors, Author{ID: key, Name: credit.Author.Name, ProviderIDs: []string{key}, Role: firstNonEmpty(credit.Contribution, "unknown")})
					matched = matched || credit.Author.ID == id
				}
				if !matched {
					return providerValidationError("Hardcover bibliography returned a book without the selected author")
				}
				key := fmt.Sprintf("hardcover:%d", book.ID)
				results = append(results, SearchResult{Provider: p.Name(), Kind: SearchTypeBook,
					Work:    Work{ID: key, Title: book.Title, Description: book.Description, FirstPublishYear: book.ReleaseYear, FirstPublishDate: book.ReleaseDate, CoverURL: book.Image.URL, Authors: authors, ProviderIDs: []string{key}},
					Edition: Edition{Title: book.Title, WorkID: key, Format: FormatAny},
					Score:   0.95, Confidence: "high", MatchedOn: []string{"hardcover author identity"}, RawSourceKey: strconv.FormatInt(book.ID, 10)})
				if len(results) > bibliographyMaxRecords {
					return providerValidationError("Hardcover bibliography exceeds the traversal safety limit; no partial bibliography was applied")
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if len(data.Books) == 0 {
			return results, nil
		}

	}
}

func hardcoverDocumentID(value any) int64 {
	if number, ok := value.(float64); ok {
		if number <= 0 || number > 2147483647 || number != float64(int64(number)) {
			return 0
		}
		return int64(number)
	}
	id, err := strconv.ParseInt(fmt.Sprint(value), 10, 32)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}
