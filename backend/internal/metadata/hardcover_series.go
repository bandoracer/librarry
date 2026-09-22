package metadata

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// This is a bounded discovery window, not a complete series bibliography.
// Fields and relationships follow hardcoverapp/hardcover-docs/schema.graphql.
const hardcoverSeriesDiscoveryQuery = `query FindSeries($query:String!) {
 search(query:$query,query_type:"Series",per_page:3,page:1) { results }
}`

const hardcoverSeriesQuery = `query SeriesMembers($ids:[Int!]!) {
 series(where:{id:{_in:$ids}},order_by:{id:asc},limit:3) {
  id name book_series(where:{compilation:{_eq:false}},order_by:[{featured:desc},{position:asc_nulls_last},{book_id:asc}],limit:25) {
   book_id position featured compilation book { ` + hardcoverWorkFields + hardcoverDefaultEditions + ` }
  }
 }
}`

type hardcoverSeriesEntry struct {
	BookID      int64               `json:"book_id"`
	Position    *float64            `json:"position"`
	Featured    bool                `json:"featured"`
	Compilation bool                `json:"compilation"`
	Book        *hardcoverGraphBook `json:"book"`
}
type hardcoverSeriesRecord struct {
	ID    int64                  `json:"id"`
	Name  string                 `json:"name"`
	Books []hardcoverSeriesEntry `json:"book_series"`
}

func (p *HardcoverProvider) searchSeries(ctx Context, query Query) ([]SearchResult, error) {
	name := strings.TrimSpace(query.Query)
	if name == "" {
		return nil, fmt.Errorf("Enter a series name")
	}
	// The live API forbids LIKE operators. Discover identities through its
	// search endpoint, then fetch explicit memberships only for those IDs.
	var discovered struct {
		Search struct {
			Results json.RawMessage `json:"results"`
		} `json:"search"`
	}
	ids := []int64{}
	wanted := map[int64]bool{}
	err := p.graphQL(ctx, hardcoverSeriesDiscoveryQuery, map[string]any{"query": name}, &discovered, func() error {
		docs, err := hardcoverSearchDocuments(discovered.Search.Results)
		if err != nil {
			return err
		}
		if len(docs) > 3 {
			return providerValidationError("Hardcover series discovery exceeds its result bound")
		}
		seen := map[int64]bool{}
		for _, doc := range docs {
			id := hardcoverDocumentID(doc["id"])
			title := strings.TrimSpace(stringValue(doc["name"]))
			if id == 0 || title == "" || seen[id] {
				return providerValidationError("Hardcover returned invalid series discovery identity")
			}
			seen[id] = true
			if !strings.Contains(strings.ToLower(title), strings.ToLower(name)) {
				continue
			}
			ids = append(ids, id)
			wanted[id] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []SearchResult{}, nil
	}
	var data struct {
		Series []hardcoverSeriesRecord `json:"series"`
	}
	rows := []SearchResult{}
	err = p.graphQL(ctx, hardcoverSeriesQuery, map[string]any{"ids": ids}, &data, func() error {
		if data.Series == nil || len(data.Series) > 3 {
			return providerValidationError("Hardcover series search is incomplete or exceeds its result bound")
		}
		seenSeries := map[int64]bool{}
		for _, series := range data.Series {
			if series.ID <= 0 || series.ID > 2147483647 || !wanted[series.ID] || seenSeries[series.ID] || !strings.Contains(strings.ToLower(series.Name), strings.ToLower(name)) || series.Books == nil || len(series.Books) > 25 {
				return providerValidationError("Hardcover returned invalid series identity, name or membership")
			}
			seenSeries[series.ID] = true
		}
		// Exact names precede partial names; stable IDs separate namesakes. No
		// title/popularity/publication-year heuristic changes the numbered order.
		sort.SliceStable(data.Series, func(i, j int) bool {
			a, b := strings.EqualFold(data.Series[i].Name, name), strings.EqualFold(data.Series[j].Name, name)
			if a != b {
				return a
			}
			return data.Series[i].ID < data.Series[j].ID
		})
		for _, series := range data.Series {
			seenBooks := map[int64]bool{}
			for _, entry := range series.Books {
				if entry.Book == nil || entry.BookID != entry.Book.ID || seenBooks[entry.BookID] || entry.Compilation || (entry.Position != nil && (math.IsNaN(*entry.Position) || math.IsInf(*entry.Position, 0))) {
					return providerValidationError("Hardcover returned invalid series book membership")
				}
				seenBooks[entry.BookID] = true
			}
			sort.SliceStable(series.Books, func(i, j int) bool {
				a, b := series.Books[i], series.Books[j]
				if a.Position == nil || b.Position == nil {
					if (a.Position == nil) != (b.Position == nil) {
						return a.Position != nil
					}
				} else if *a.Position != *b.Position {
					return *a.Position < *b.Position
				}
				if a.Featured != b.Featured {
					return a.Featured
				}
				return a.BookID < b.BookID
			})
			start := len(rows)
			for _, entry := range series.Books {
				results, err := hardcoverResults(*entry.Book, query)
				if err != nil {
					return err
				}
				for _, result := range results {
					result.Work.SeriesID = fmt.Sprintf("hardcover-series:%d", series.ID)
					result.Work.Series = series.Name
					if entry.Position != nil {
						result.Work.SeriesPosition = strconv.FormatFloat(*entry.Position, 'f', -1, 64)
					}
					result.MatchedOn = append(result.MatchedOn, "series")
					rows = append(rows, result)
				}
			}
			// Keep usable editions in numbered order ahead of work-only records.
			// Sparse translated/duplicate catalog entries must not bury the next
			// volume. Unknown evidence remains available after verified editions.
			group := rows[start:]
			sort.SliceStable(group, func(i, j int) bool {
				return concreteFormat(group[i].Edition.Format) != FormatAny && concreteFormat(group[j].Edition.Format) == FormatAny
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}
