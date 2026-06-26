package library

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type localBookMetadata struct {
	Source       string
	Title        string
	AuthorName   string
	Authors      []string
	Publisher    string
	Language     string
	Description  string
	Series       string
	SeriesIndex  string
	Subjects     []string
	Identifiers  map[string]string
	ExtractError string
}

func parsedBookForPath(path string) parsedBook {
	parsed := parseBookFilename(path)
	metadata := localBookMetadataForPath(path)
	return parsedBook{
		Title:      firstNonEmpty(metadata.Title, parsed.Title),
		AuthorName: firstNonEmpty(metadata.AuthorName, parsed.AuthorName),
	}
}

func localBookMetadataForPath(path string) localBookMetadata {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return localBookMetadata{}
	}
	if strings.EqualFold(filepath.Ext(path), ".opf") {
		return parseOPFFile(path, "opf")
	}
	for _, candidate := range sidecarOPFCandidates(path) {
		if _, err := os.Stat(candidate); err == nil {
			return parseOPFFile(candidate, "sidecar-opf")
		}
	}
	if strings.EqualFold(filepath.Ext(path), ".epub") {
		return parseEPUBMetadata(path)
	}
	return localBookMetadata{}
}

func sidecarOPFCandidates(path string) []string {
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return []string{
		filepath.Join(dir, base+".opf"),
		filepath.Join(dir, "metadata.opf"),
	}
}

func parseOPFFile(path string, source string) localBookMetadata {
	file, err := os.Open(path)
	if err != nil {
		return localBookMetadata{Source: source, ExtractError: err.Error()}
	}
	defer file.Close()
	metadata, err := parseOPF(file)
	if err != nil {
		return localBookMetadata{Source: source, ExtractError: err.Error()}
	}
	metadata.Source = source
	return metadata
}

func parseEPUBMetadata(path string) localBookMetadata {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return localBookMetadata{Source: "epub-opf", ExtractError: err.Error()}
	}
	defer reader.Close()
	opfPath, err := epubOPFPath(reader.File)
	if err != nil {
		return localBookMetadata{Source: "epub-opf", ExtractError: err.Error()}
	}
	for _, file := range reader.File {
		if file.Name != opfPath {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			return localBookMetadata{Source: "epub-opf", ExtractError: err.Error()}
		}
		defer handle.Close()
		metadata, err := parseOPF(handle)
		if err != nil {
			return localBookMetadata{Source: "epub-opf", ExtractError: err.Error()}
		}
		metadata.Source = "epub-opf"
		return metadata
	}
	return localBookMetadata{Source: "epub-opf", ExtractError: "EPUB package document is missing"}
}

func epubOPFPath(files []*zip.File) (string, error) {
	for _, file := range files {
		if file.Name != "META-INF/container.xml" {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			return "", err
		}
		defer handle.Close()
		var container struct {
			Rootfiles []struct {
				FullPath string `xml:"full-path,attr"`
				Media    string `xml:"media-type,attr"`
			} `xml:"rootfiles>rootfile"`
		}
		if err := xml.NewDecoder(handle).Decode(&container); err != nil {
			return "", err
		}
		for _, rootfile := range container.Rootfiles {
			if strings.TrimSpace(rootfile.FullPath) != "" {
				return rootfile.FullPath, nil
			}
		}
	}
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file.Name), ".opf") {
			return file.Name, nil
		}
	}
	return "", errors.New("EPUB container does not reference an OPF package document")
}

func parseOPF(reader io.Reader) (localBookMetadata, error) {
	decoder := xml.NewDecoder(io.LimitReader(reader, 2<<20))
	metadata := localBookMetadata{Identifiers: map[string]string{}}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return metadata, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch strings.ToLower(start.Name.Local) {
		case "title":
			text := decodeElementText(decoder, start)
			metadata.Title = firstNonEmpty(metadata.Title, text)
		case "creator":
			text := decodeElementText(decoder, start)
			if text != "" {
				metadata.Authors = append(metadata.Authors, text)
				metadata.AuthorName = firstNonEmpty(metadata.AuthorName, text)
			}
		case "identifier":
			text := decodeElementText(decoder, start)
			addIdentifier(metadata.Identifiers, text, attrValue(start, "scheme"))
		case "language":
			metadata.Language = firstNonEmpty(metadata.Language, decodeElementText(decoder, start))
		case "publisher":
			metadata.Publisher = firstNonEmpty(metadata.Publisher, decodeElementText(decoder, start))
		case "description":
			metadata.Description = firstNonEmpty(metadata.Description, decodeElementText(decoder, start))
		case "subject":
			text := decodeElementText(decoder, start)
			if text != "" {
				metadata.Subjects = append(metadata.Subjects, text)
			}
		case "meta":
			readOPFMeta(decoder, start, &metadata)
		}
	}
	if len(metadata.Identifiers) == 0 {
		metadata.Identifiers = nil
	}
	return metadata, nil
}

func decodeElementText(decoder *xml.Decoder, start xml.StartElement) string {
	var value string
	if err := decoder.DecodeElement(&value, &start); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func readOPFMeta(decoder *xml.Decoder, start xml.StartElement, metadata *localBookMetadata) {
	name := strings.ToLower(attrValue(start, "name"))
	property := strings.ToLower(attrValue(start, "property"))
	content := strings.TrimSpace(attrValue(start, "content"))
	if content == "" {
		content = decodeElementText(decoder, start)
	} else {
		var discard string
		_ = decoder.DecodeElement(&discard, &start)
	}
	switch {
	case name == "calibre:series" || property == "belongs-to-collection":
		metadata.Series = firstNonEmpty(metadata.Series, content)
	case name == "calibre:series_index" || property == "group-position":
		metadata.SeriesIndex = firstNonEmpty(metadata.SeriesIndex, content)
	case name == "cover":
		return
	case strings.Contains(name, "identifier") || strings.Contains(property, "identifier"):
		addIdentifier(metadata.Identifiers, content, name)
	}
}

func attrValue(start xml.StartElement, local string) string {
	for _, attr := range start.Attr {
		if strings.EqualFold(attr.Name.Local, local) {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func addIdentifier(identifiers map[string]string, raw string, scheme string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return
	}
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	normalizedISBN := normalizeISBN(value)
	switch {
	case normalizedISBN != "":
		identifiers["isbn"] = normalizedISBN
		if len(normalizedISBN) == 13 {
			identifiers["isbn13"] = normalizedISBN
		}
		if len(normalizedISBN) == 10 {
			identifiers["isbn10"] = normalizedISBN
		}
	case strings.Contains(scheme, "isbn"):
		identifiers["isbn"] = value
	case strings.Contains(scheme, "asin") || strings.Contains(scheme, "amazon"):
		identifiers["asin"] = value
	case strings.Contains(scheme, "openlibrary") || strings.HasPrefix(value, "OL"):
		identifiers["openlibrary"] = value
	}
}

func normalizeISBN(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(strings.ToLower(value), "urn:isbn:")
	var builder strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		} else if (r == 'x' || r == 'X') && builder.Len() == 9 {
			builder.WriteRune('X')
		}
	}
	candidate := builder.String()
	if len(candidate) == 10 || len(candidate) == 13 {
		return candidate
	}
	return ""
}

func applyLocalMetadataToMap(target map[string]any, metadata localBookMetadata) {
	if target == nil || (metadata.Source == "" && metadata.ExtractError == "") {
		return
	}
	local := map[string]any{"source": metadata.Source}
	if metadata.Title != "" {
		local["title"] = metadata.Title
	}
	if len(metadata.Authors) > 0 {
		local["authors"] = metadata.Authors
	}
	if metadata.Publisher != "" {
		local["publisher"] = metadata.Publisher
	}
	if metadata.Language != "" {
		local["language"] = metadata.Language
	}
	if metadata.Description != "" {
		local["description"] = metadata.Description
	}
	if metadata.Series != "" {
		local["series"] = metadata.Series
	}
	if metadata.SeriesIndex != "" {
		local["seriesIndex"] = metadata.SeriesIndex
		if parsed, err := strconv.ParseFloat(metadata.SeriesIndex, 64); err == nil {
			local["seriesIndexNumber"] = parsed
		}
	}
	if len(metadata.Subjects) > 0 {
		local["subjects"] = metadata.Subjects
	}
	if len(metadata.Identifiers) > 0 {
		local["identifiers"] = metadata.Identifiers
		for key, value := range metadata.Identifiers {
			target[key] = value
		}
	}
	if metadata.ExtractError != "" {
		local["error"] = metadata.ExtractError
	}
	target["localMetadata"] = local
	target["metadataSource"] = metadata.Source
}
