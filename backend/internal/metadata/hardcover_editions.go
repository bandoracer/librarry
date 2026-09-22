package metadata

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Work and edition fields are deliberately queried independently: book-level
// ISBN collections and format summaries do not identify a published edition.
const hardcoverWorkFields = `id title book_category_id genres: cached_tags(path:"Genre") description release_year release_date image { url }
 contributions(where:{contributable_type:{_eq:"Book"}}) { contribution author { id name } }`
const hardcoverEditionFields = `id book_id title subtitle edition_information reading_format_id isbn_10 isbn_13 asin pages audio_seconds release_date release_year
 language { language } publisher { name } image { url }
 contributions(where:{contributable_type:{_eq:"Edition"}}) { contribution author { id name } }`
const hardcoverDefaultEditions = `default_ebook_edition { ` + hardcoverEditionFields + ` }
 default_audio_edition { ` + hardcoverEditionFields + ` }`

type hardcoverContribution struct {
	Contribution string `json:"contribution"`
	Author       struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"author"`
}
type hardcoverGraphEdition struct {
	EditionInformation string `json:"edition_information"`
	ID                 int64  `json:"id"`
	BookID             int64  `json:"book_id"`
	Title              string `json:"title"`
	Subtitle           string `json:"subtitle"`
	ReadingFormatID    int    `json:"reading_format_id"`
	ISBN10             string `json:"isbn_10"`
	ISBN13             string `json:"isbn_13"`
	ASIN               string `json:"asin"`
	Pages              int    `json:"pages"`
	AudioSeconds       int    `json:"audio_seconds"`
	ReleaseDate        string `json:"release_date"`
	ReleaseYear        int    `json:"release_year"`
	Language           struct {
		Language string `json:"language"`
	} `json:"language"`
	Publisher struct {
		Name string `json:"name"`
	} `json:"publisher"`
	Image struct {
		URL string `json:"url"`
	} `json:"image"`
	Contributions []hardcoverContribution `json:"contributions"`
	Book          *hardcoverGraphBook     `json:"book"`
}

func hardcoverAuthors(credits []hardcoverContribution) ([]Author, error) {
	authors := make([]Author, 0, len(credits))
	for _, credit := range credits {
		if credit.Author.ID <= 0 || credit.Author.ID > 2147483647 || strings.TrimSpace(credit.Author.Name) == "" {
			return nil, providerValidationError("Hardcover returned an invalid contributor identity")
		}
		key := fmt.Sprintf("hardcover-author:%d", credit.Author.ID)
		authors = append(authors, Author{ID: key, Name: strings.TrimSpace(credit.Author.Name), Role: firstNonEmpty(strings.TrimSpace(credit.Contribution), "unknown"), ProviderIDs: []string{key}})
	}
	// A known writer takes display precedence over narrators and other credits.
	ordered := make([]Author, 0, len(authors))
	for _, a := range authors {
		if strings.EqualFold(a.Role, "author") || strings.EqualFold(a.Role, "writer") {
			ordered = append(ordered, a)
		}
	}
	for _, a := range authors {
		if !strings.EqualFold(a.Role, "author") && !strings.EqualFold(a.Role, "writer") {
			ordered = append(ordered, a)
		}
	}
	return ordered, nil
}
func hardcoverWork(book hardcoverGraphBook) (Work, error) {
	if book.ID <= 0 || book.ID > 2147483647 || strings.TrimSpace(book.Title) == "" {
		return Work{}, providerValidationError("Hardcover returned an invalid work identity or title")
	}
	authors, err := hardcoverAuthors(book.Contributions)
	if err != nil {
		return Work{}, err
	}
	key := fmt.Sprintf("hardcover:%d", book.ID)
	// Category 4 is Graphic Novel in Hardcover's book_categories catalog.
	contentType := ""
	if book.BookCategoryID == 4 {
		contentType = "graphic_novel"
	}
	subjects := []string{}
	for _, genre := range book.Genres {
		subjects = appendUniqueStrings(subjects, genre.Tag)
	}
	return Work{Subjects: subjects, ContentType: contentType, ID: key, Title: strings.TrimSpace(book.Title), Description: book.Description, Authors: authors,
		FirstPublishYear: book.ReleaseYear, FirstPublishDate: book.ReleaseDate, CoverURL: book.Image.URL, ProviderIDs: []string{key}}, nil
}
func hardcoverEdition(raw hardcoverGraphEdition, work Work) (Edition, error) {
	if raw.ID <= 0 || raw.ID > 2147483647 || work.ID != fmt.Sprintf("hardcover:%d", raw.BookID) {
		return Edition{}, providerValidationError("Hardcover edition does not belong to its returned work")
	}
	contributors, err := hardcoverAuthors(raw.Contributions)
	if err != nil {
		return Edition{}, err
	}
	format := FormatAny
	switch raw.ReadingFormatID {
	case 2:
		format = FormatAudiobook
	case 4:
		format = FormatEbook
	}
	isbns := []string{}
	for _, value := range []string{raw.ISBN10, raw.ISBN13} {
		if canonicalISBN(value) != "" {
			isbns = appendUniqueStrings(isbns, isbnCharacters(value))
		}
	}
	if len(isbns) == 2 && canonicalISBN(isbns[0]) != canonicalISBN(isbns[1]) {
		return Edition{}, providerValidationError("Hardcover edition has conflicting ISBN identities")
	}
	key := fmt.Sprintf("hardcover-edition:%d", raw.ID)
	title := firstNonEmpty(strings.TrimSpace(raw.Title), work.Title)
	if subtitle := strings.TrimSpace(raw.Subtitle); subtitle != "" {
		title += ": " + subtitle
	}
	published := raw.ReleaseDate
	if published == "" && raw.ReleaseYear > 0 {
		published = strconv.Itoa(raw.ReleaseYear)
	}
	return Edition{EditionInformation: raw.EditionInformation, ID: key, WorkID: work.ID, Title: title, Format: format, Language: raw.Language.Language,
		ISBNs: isbns, ASIN: raw.ASIN, Publisher: raw.Publisher.Name, PublishedDate: published, Pages: max(0, raw.Pages),
		AudioSeconds: max(0, raw.AudioSeconds), CoverURL: raw.Image.URL, Contributors: contributors, ProviderIDs: []string{key}}, nil
}
func hardcoverResults(book hardcoverGraphBook, query Query) ([]SearchResult, error) {
	work, err := hardcoverWork(book)
	if err != nil {
		return nil, err
	}
	rawEditions := []*hardcoverGraphEdition{}
	if query.Format != FormatAudiobook {
		rawEditions = append(rawEditions, book.DefaultEbook)
	}
	if query.Format != FormatEbook {
		rawEditions = append(rawEditions, book.DefaultAudio)
	}
	editions := []Edition{}
	seen := map[string]bool{}
	for _, raw := range rawEditions {
		if raw == nil {
			continue
		}
		edition, err := hardcoverEdition(*raw, work)
		if err != nil {
			return nil, err
		}
		// A default relationship alone cannot make an edition an ebook/audiobook.
		// Physical and ambiguous formats remain work-level evidence.
		if edition.Format == FormatAny {
			continue
		}
		if raw == book.DefaultEbook && edition.Format != FormatEbook || raw == book.DefaultAudio && edition.Format != FormatAudiobook {
			return nil, providerValidationError("Hardcover default edition format conflicts with its relationship")
		}
		if seen[edition.ID] {
			return nil, providerValidationError("Hardcover default editions repeat an identity with conflicting formats")
		}
		editions = append(editions, edition)
		seen[edition.ID] = true
	}
	if len(editions) == 0 {
		editions = append(editions, Edition{WorkID: work.ID, Title: work.Title, Format: FormatAny})
	}
	results := make([]SearchResult, 0, len(editions))
	for _, edition := range editions {
		results = append(results, hardcoverResult(query, work, edition))
	}
	return results, nil
}
func hardcoverResult(query Query, work Work, edition Edition) SearchResult {
	author := ""
	if len(work.Authors) > 0 {
		author = work.Authors[0].Name
	}
	score := scoreResult(query, work.Title, author, edition.ISBNs)
	if lookup, ok := exactBookLookup(query); ok && lookup.isbn != "" {
		for _, isbn := range edition.ISBNs {
			if canonicalISBN(isbn) == lookup.isbn {
				score = 0.99
			}
		}
	}
	return SearchResult{Provider: "Hardcover", Kind: SearchTypeBook, Work: work, Edition: edition, Score: score, Confidence: confidence(score), MatchedOn: []string{"hardcover work and edition records"}, RawSourceKey: work.ID}
}

// Interactive discovery may keep a validated work and independent healthy
// editions when a default relationship is malformed. Bibliography and exact
// edition lookups keep their strict validation contracts.
func hardcoverDiscoveryResults(book hardcoverGraphBook, query Query) ([]SearchResult, error) {
	rows, err := hardcoverResults(book, query)
	if err == nil {
		return rows, nil
	}
	work, workErr := hardcoverWork(book)
	if workErr != nil {
		return nil, workErr
	}
	healthy := []SearchResult{}
	duplicate := map[string]bool{}
	seen := map[string]bool{}
	for _, format := range []MediaFormat{FormatEbook, FormatAudiobook} {
		if concreteFormat(query.Format) != FormatAny && query.Format != format {
			continue
		}
		subset, request := book, query
		request.Format = format
		if format == FormatEbook {
			subset.DefaultAudio = nil
		} else {
			subset.DefaultEbook = nil
		}
		candidates, editionErr := hardcoverResults(subset, request)
		if editionErr != nil {
			continue
		}
		for _, candidate := range candidates {
			id := candidate.Edition.ID
			if id == "" {
				continue
			}
			if seen[id] {
				duplicate[id] = true
			}
			seen[id] = true
			healthy = append(healthy, candidate)
		}
	}
	valid := healthy[:0]
	for _, result := range healthy {
		if !duplicate[result.Edition.ID] {
			valid = append(valid, result)
		}
	}
	if len(valid) == 0 {
		valid = append(valid, hardcoverResult(query, work, Edition{WorkID: work.ID, Title: work.Title, Format: FormatAny}))
	}
	return valid, err
}

func (p *HardcoverProvider) enrichBookSearch(ctx Context, query Query, ids []int64) ([]SearchResult, error) {
	if len(ids) == 0 {
		return []SearchResult{}, nil
	}
	var data struct {
		Books []hardcoverGraphBook `json:"books"`
	}
	results := []SearchResult{}
	var invalid []error
	err := p.graphQL(ctx, `query SearchBookDetails($ids:[Int!]!) { books(where:{id:{_in:$ids}}) { `+hardcoverWorkFields+hardcoverDefaultEditions+` } }`, map[string]any{"ids": ids}, &data, func() error {
		byID := map[int64]hardcoverGraphBook{}
		for _, book := range data.Books {
			if _, exists := byID[book.ID]; exists {
				return providerValidationError("Hardcover returned duplicate work details")
			}
			byID[book.ID] = book
		}
		if len(byID) != len(ids) {
			return providerValidationError("Hardcover search details are incomplete")
		}
		for _, id := range ids {
			if _, ok := byID[id]; !ok {
				return providerValidationError("Hardcover returned details for an unexpected work")
			}
		}
		for _, id := range ids {
			rows, err := hardcoverDiscoveryResults(byID[id], query)
			if err != nil {
				invalid = append(invalid, err)
			}
			results = append(results, rows...)
		}
		if len(invalid) > 0 {
			return errors.Join(invalid...)
		}
		return nil
	})
	if err != nil {
		// Observation classifies the underlying provider failure, so attach the
		// partial-result marker afterwards to preserve it at the service/cache.
		if len(results) > 0 && len(invalid) > 0 {
			return results, &partialSearchError{fmt.Errorf("Ignored invalid edition or work data in %d Hardcover records: %w", len(invalid), err)}
		}
		return nil, err
	}
	return results, nil
}

// isbn10ForCanonical is used only after canonicalISBN validated an ISBN-13.
func isbn10ForCanonical(isbn string) string {
	if len(isbn) != 13 || !strings.HasPrefix(isbn, "978") {
		return ""
	}
	base := isbn[3:12]
	sum := 0
	for i, r := range base {
		sum += (10 - i) * int(r-'0')
	}
	check := (11 - sum%11) % 11
	if check == 10 {
		return base + "X"
	}
	return base + strconv.Itoa(check)
}
func (p *HardcoverProvider) searchISBN(ctx Context, query Query, isbn string) ([]SearchResult, error) {
	var data struct {
		Editions []hardcoverGraphEdition `json:"editions"`
	}
	results := []SearchResult{}
	isbn10s := []string{}
	if id := isbn10ForCanonical(isbn); id != "" {
		isbn10s = append(isbn10s, id)
	}
	err := p.graphQL(ctx, `query ExactBookEdition($isbn13:String!, $isbn10s:[String!]!) {
 editions(where:{_or:[{isbn_13:{_eq:$isbn13}},{isbn_10:{_in:$isbn10s}}]},limit:51) { `+hardcoverEditionFields+` book { `+hardcoverWorkFields+` } } }`, map[string]any{"isbn13": isbn, "isbn10s": isbn10s}, &data, func() error {
		if data.Editions == nil || len(data.Editions) > 50 {
			return providerValidationError("Hardcover exact edition lookup is incomplete or exceeds its result bound")
		}
		seen := map[string]bool{}
		for _, raw := range data.Editions {
			if raw.Book == nil {
				return providerValidationError("Hardcover exact edition is missing its work")
			}
			work, err := hardcoverWork(*raw.Book)
			if err != nil {
				return err
			}
			edition, err := hardcoverEdition(raw, work)
			if err != nil {
				return err
			}
			if seen[edition.ID] {
				return providerValidationError("Hardcover exact edition lookup repeated an identity")
			}
			seen[edition.ID] = true
			matches := false
			for _, id := range edition.ISBNs {
				matches = matches || canonicalISBN(id) == isbn
			}
			if !matches {
				return providerValidationError("Hardcover returned an edition with a different ISBN")
			}
			if raw.ReadingFormatID == 1 {
				continue
			}
			result := hardcoverResult(query, work, edition)
			if resultFitsQuery(query, result) {
				results = append(results, result)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}
