package metadata

import "time"

type SearchType string
type MediaFormat string

const (
	SearchTypeBook        SearchType = "book"
	SearchTypeAuthor      SearchType = "author"
	SearchTypeAuthorWorks SearchType = "author_works"
	SearchTypeSeries      SearchType = "series"

	FormatAny       MediaFormat = "any"
	FormatEbook     MediaFormat = "ebook"
	FormatAudiobook MediaFormat = "audiobook"
)

type Query struct {
	Query             string      `json:"query"`
	Type              SearchType  `json:"type"`
	Format            MediaFormat `json:"format"`
	PreferredLanguage string      `json:"preferredLanguage,omitempty"`
	Limit             int         `json:"limit"`
	ProviderKey       string      `json:"providerKey,omitempty"`
}

type Author struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	ProviderIDs []string `json:"providerIds,omitempty"`
}

type Work struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Authors          []Author `json:"authors"`
	FirstPublishYear int      `json:"firstPublishYear,omitempty"`
	Description      string   `json:"description,omitempty"`
	Series           string   `json:"series,omitempty"`
	SeriesPosition   string   `json:"seriesPosition,omitempty"`
	CoverURL         string   `json:"coverUrl,omitempty"`
	ProviderIDs      []string `json:"providerIds,omitempty"`
}

type Edition struct {
	ID            string      `json:"id"`
	WorkID        string      `json:"workId,omitempty"`
	Title         string      `json:"title"`
	Format        MediaFormat `json:"format"`
	Language      string      `json:"language,omitempty"`
	ISBNs         []string    `json:"isbns,omitempty"`
	ASIN          string      `json:"asin,omitempty"`
	Publisher     string      `json:"publisher,omitempty"`
	PublishedDate string      `json:"publishedDate,omitempty"`
	ProviderIDs   []string    `json:"providerIds,omitempty"`
}

type SearchResult struct {
	Provider     string     `json:"provider"`
	Kind         SearchType `json:"kind"`
	Work         Work       `json:"work"`
	Edition      Edition    `json:"edition,omitempty"`
	Score        float64    `json:"score"`
	Confidence   string     `json:"confidence"`
	MatchedOn    []string   `json:"matchedOn"`
	RawSourceKey string     `json:"rawSourceKey,omitempty"`
}

type ProviderHealth struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Configured bool      `json:"configured"`
	Message    string    `json:"message"`
	CheckedAt  time.Time `json:"checkedAt"`
}

type Diagnostic struct {
	Name         string         `json:"name"`
	Configured   bool           `json:"configured"`
	Capabilities []string       `json:"capabilities"`
	Notes        []string       `json:"notes"`
	Raw          map[string]any `json:"raw,omitempty"`
}

type Provider interface {
	Name() string
	Health(ctx Context) ProviderHealth
	Diagnostics(ctx Context) Diagnostic
	Search(ctx Context, query Query) ([]SearchResult, error)
}

type Context interface {
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}
