package library

import (
	"strconv"
	"strings"
	"unicode"
)

// namingTokenValues builds the template token set for destination naming.
// Author/Title/Format/Ext always render; Series/SeriesPosition/Year may be
// empty and collapse cleanly (see renderTemplate).
func namingTokenValues(parsed parsedBook, format string, ext string) map[string]string {
	return map[string]string{
		"Author":         firstNonEmpty(parsed.AuthorName, "Unknown Author"),
		"Title":          firstNonEmpty(parsed.Title, "Unknown Title"),
		"Series":         strings.TrimSpace(parsed.Series),
		"SeriesPosition": strings.TrimSpace(parsed.SeriesPosition),
		"Year":           strings.TrimSpace(parsed.Year),
		"Format":         normalizeFormat(format),
		"Ext":            strings.ToLower(ext),
	}
}

// RenderNamingTemplate renders {Token} placeholders (case-tolerant for the
// all-lowercase form) and collapses the artifacts empty values leave behind:
// dangling separators, empty brackets, and doubled-up delimiters.
func RenderNamingTemplate(template string, values map[string]string) string {
	return renderTemplate(template, values)
}

const emptyTokenMarker = "\x00"

func renderTemplate(template string, values map[string]string) string {
	rendered := template
	for key, value := range values {
		replacement := strings.TrimSpace(value)
		if replacement == "" {
			replacement = emptyTokenMarker
		}
		rendered = strings.ReplaceAll(rendered, "{"+key+"}", replacement)
		rendered = strings.ReplaceAll(rendered, "{"+strings.ToLower(key)+"}", replacement)
	}
	if !strings.Contains(rendered, emptyTokenMarker) {
		return rendered
	}
	return collapseEmptyTokens(rendered)
}

// collapseEmptyTokens first deletes bracket groups that only decorated empty
// tokens ("Title (\x00).epub" -> "Title.epub"), then removes the
// whitespace-delimited words that contained only empty tokens (plus any
// punctuation decorating them, e.g. "#"), and finally strips separator words
// left dangling at the edges or doubled up in the middle
// ("Author - - Title" -> "Author - Title").
func collapseEmptyTokens(rendered string) string {
	rendered = removeEmptyBracketGroups(rendered)
	words := strings.Fields(rendered)
	kept := make([]string, 0, len(words))
	for _, word := range words {
		if !strings.Contains(word, emptyTokenMarker) {
			kept = append(kept, word)
			continue
		}
		cleaned := strings.ReplaceAll(word, emptyTokenMarker, "")
		if hasLetterOrDigit(cleaned) {
			kept = append(kept, cleaned)
		}
	}
	result := make([]string, 0, len(kept))
	for _, word := range kept {
		if !hasLetterOrDigit(word) {
			if len(result) == 0 || !hasLetterOrDigit(result[len(result)-1]) {
				continue
			}
		}
		result = append(result, word)
	}
	for len(result) > 0 && !hasLetterOrDigit(result[len(result)-1]) {
		result = result[:len(result)-1]
	}
	return strings.Join(result, " ")
}

// removeEmptyBracketGroups deletes bracket pairs whose interior holds at least
// one empty-token marker and no letters or digits, together with the spaces
// leading into them, so "Title (\x00).epub" collapses to "Title.epub".
func removeEmptyBracketGroups(value string) string {
	closers := map[byte]byte{'(': ')', '[': ']', '{': '}'}
	for {
		changed := false
		for i := 0; i < len(value) && !changed; i++ {
			closer, ok := closers[value[i]]
			if !ok {
				continue
			}
			hasMarker := false
			for j := i + 1; j < len(value); j++ {
				c := value[j]
				if c == closer {
					if !hasMarker {
						break
					}
					start := i
					for start > 0 && value[start-1] == ' ' {
						start--
					}
					value = value[:start] + value[j+1:]
					changed = true
					break
				}
				if c == emptyTokenMarker[0] {
					hasMarker = true
					continue
				}
				// Letters, digits, non-ASCII, or nested brackets mean this
				// group carries real content; leave it alone.
				if c >= 0x80 || c == '(' || c == '[' || c == '{' ||
					(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
					break
				}
			}
		}
		if !changed {
			return value
		}
	}
}

func hasLetterOrDigit(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// yearFromString extracts a leading four-digit year from values like "2021",
// "2021-03-04", or "March 2021".
func yearFromString(value string) string {
	value = strings.TrimSpace(value)
	for index := 0; index+4 <= len(value); index++ {
		candidate := value[index : index+4]
		if isDigits(candidate) {
			if index > 0 && isDigits(value[index-1:index]) {
				continue
			}
			if index+4 < len(value) && isDigits(value[index+4:index+5]) {
				continue
			}
			if parsed, err := strconv.Atoi(candidate); err == nil && parsed >= 1000 {
				return candidate
			}
		}
	}
	return ""
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
