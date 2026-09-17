package metadata

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// exactLookup defines Google's deliberately narrow fallback contract. ISBNs are
// checksum-validated and canonicalized across equivalent ISBN-10/978 ISBN-13.
// Other book queries are literal full titles, never an author/series query DSL.
type exactLookup struct {
	isbn  string
	title string
}

func exactBookLookup(query Query) (exactLookup, bool) {
	if query.Type != "" && query.Type != SearchTypeBook {
		return exactLookup{}, false
	}
	raw := strings.TrimSpace(norm.NFKC.String(query.Query))
	explicitISBN := false
	for _, prefix := range []string{"isbn-13:", "isbn-10:", "isbn:", "isbn "} {
		if strings.HasPrefix(strings.ToLower(raw), prefix) {
			raw = strings.TrimSpace(raw[len(prefix):])
			explicitISBN = true
			break
		}
	}
	if id := canonicalISBN(raw); id != "" {
		return exactLookup{isbn: id}, true
	}
	// A mistyped ISBN must not silently become a broad numeric title search.
	if explicitISBN || (isbnCharacters(raw) != "" && len(isbnCharacters(raw)) >= 10) {
		return exactLookup{}, false
	}
	title := exactTitle(raw)
	if title == "" {
		return exactLookup{}, false
	}
	return exactLookup{title: title}, true
}
func exactTitle(value string) string {
	value = strings.ToLower(norm.NFKC.String(value))
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) }), " ")
}
func isbnCharacters(value string) string {
	var result strings.Builder
	for _, r := range norm.NFKC.String(strings.TrimSpace(value)) {
		switch {
		case r >= '0' && r <= '9':
			result.WriteRune(r)
		case r == 'x' || r == 'X':
			result.WriteByte('X')
		case unicode.IsSpace(r) || r == '-' || r == '\u2010':
		default:
			return ""
		}
	}
	return result.String()
}
func canonicalISBN(value string) string {
	digits := isbnCharacters(value)
	switch len(digits) {
	case 10:
		sum := 0
		for index, r := range digits {
			n := int(r - '0')
			if r == 'X' && index == 9 {
				n = 10
			} else if r < '0' || r > '9' {
				return ""
			}
			sum += (10 - index) * n
		}
		if sum%11 != 0 {
			return ""
		}
		base := "978" + digits[:9]
		return base + strconv.Itoa(isbn13CheckDigit(base))
	case 13:
		for _, r := range digits {
			if r < '0' || r > '9' {
				return ""
			}
		}
		if !strings.HasPrefix(digits, "978") && !strings.HasPrefix(digits, "979") {
			return ""
		}
		if int(digits[12]-'0') != isbn13CheckDigit(digits[:12]) {
			return ""
		}
		return digits
	}
	return ""
}
func isbn13CheckDigit(base string) int {
	sum := 0
	for i := range len(base) {
		weight := 1
		if i%2 == 1 {
			weight = 3
		}
		sum += weight * int(base[i]-'0')
	}
	return (10 - sum%10) % 10
}
func (q exactLookup) matches(result SearchResult) bool {
	if result.Kind != SearchTypeBook {
		return false
	}
	if q.isbn != "" {
		for _, isbn := range result.Edition.ISBNs {
			if canonicalISBN(isbn) == q.isbn {
				return true
			}
		}
		return false
	}
	return q.title != "" && (exactTitle(result.Work.Title) == q.title || exactTitle(result.Edition.Title) == q.title)
}
func resultFitsQuery(query Query, result SearchResult) bool {
	if !resultMatchesPreferredLanguage(result, query.PreferredLanguage) {
		return false
	}
	requested, actual := concreteFormat(query.Format), concreteFormat(result.Edition.Format)
	return requested == FormatAny || actual == FormatAny || requested == actual
}
func hasExactPrimaryMatch(query Query, results []SearchResult) bool {
	lookup, ok := exactBookLookup(query)
	if !ok {
		return false
	}
	for _, result := range results {
		if resultFitsQuery(query, result) && lookup.matches(result) {
			return true
		}
	}
	return false
}
