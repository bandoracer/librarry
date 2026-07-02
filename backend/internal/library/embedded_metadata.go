package library

import (
	"archive/zip"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"
)

type localBookMetadata struct {
	Source       string
	Title        string
	AuthorName   string
	Authors      []string
	Publisher    string
	Language     string
	Description  string
	Album        string
	Year         string
	Track        string
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
		Title:          firstNonEmpty(metadata.Title, parsed.Title),
		AuthorName:     firstNonEmpty(metadata.AuthorName, parsed.AuthorName),
		Series:         strings.TrimSpace(metadata.Series),
		SeriesPosition: strings.TrimSpace(metadata.SeriesIndex),
		Year:           yearFromString(metadata.Year),
		Identifiers:    metadata.Identifiers,
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
	if isAudioMetadataExtension(path) {
		return parseAudioMetadata(path)
	}
	return localBookMetadata{}
}

func isAudioMetadataExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".m4a", ".m4b", ".aac":
		return true
	default:
		return false
	}
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

func parseAudioMetadata(path string) localBookMetadata {
	file, err := os.Open(path)
	if err != nil {
		return localBookMetadata{Source: "audio-tags", ExtractError: err.Error()}
	}
	defer file.Close()
	header := make([]byte, 12)
	n, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return localBookMetadata{Source: "audio-tags", ExtractError: err.Error()}
	}
	if n >= 10 && string(header[:3]) == "ID3" {
		if _, err := file.Seek(10, io.SeekStart); err != nil {
			return localBookMetadata{Source: "id3v2", ExtractError: err.Error()}
		}
		metadata, err := parseID3v2(file, header[:10])
		if err != nil {
			return localBookMetadata{Source: "id3v2", ExtractError: err.Error()}
		}
		metadata.Source = "id3v2"
		return metadata
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".m4a" || ext == ".m4b" || ext == ".aac" || (n >= 8 && string(header[4:8]) == "ftyp") {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return localBookMetadata{Source: "mp4-tags", ExtractError: err.Error()}
		}
		metadata, err := parseMP4Metadata(file)
		if err != nil {
			return localBookMetadata{Source: "mp4-tags", ExtractError: err.Error()}
		}
		metadata.Source = "mp4-tags"
		return metadata
	}
	return localBookMetadata{}
}

func parseID3v2(reader io.Reader, header []byte) (localBookMetadata, error) {
	if len(header) < 10 {
		return localBookMetadata{}, errors.New("ID3 header is incomplete")
	}
	major := int(header[3])
	tagSize := syncSafeInt(header[6:10])
	if tagSize <= 0 {
		return localBookMetadata{}, nil
	}
	if tagSize > 4<<20 {
		tagSize = 4 << 20
	}
	payload := make([]byte, tagSize)
	if _, err := io.ReadFull(reader, payload); err != nil && err != io.ErrUnexpectedEOF {
		return localBookMetadata{}, err
	}
	metadata := localBookMetadata{Identifiers: map[string]string{}}
	offset := 0
	for offset < len(payload) {
		var frameID string
		var frameSize int
		var headerSize int
		if major == 2 {
			if offset+6 > len(payload) {
				break
			}
			frameID = string(payload[offset : offset+3])
			frameSize = int(payload[offset+3])<<16 | int(payload[offset+4])<<8 | int(payload[offset+5])
			headerSize = 6
		} else {
			if offset+10 > len(payload) {
				break
			}
			frameID = string(payload[offset : offset+4])
			if frameID == "\x00\x00\x00\x00" {
				break
			}
			if major == 4 {
				frameSize = syncSafeInt(payload[offset+4 : offset+8])
			} else {
				frameSize = int(binary.BigEndian.Uint32(payload[offset+4 : offset+8]))
			}
			headerSize = 10
		}
		if strings.Trim(frameID, "\x00") == "" || frameSize <= 0 || offset+headerSize+frameSize > len(payload) {
			break
		}
		readID3Frame(frameID, payload[offset+headerSize:offset+headerSize+frameSize], &metadata)
		offset += headerSize + frameSize
	}
	if len(metadata.Identifiers) == 0 {
		metadata.Identifiers = nil
	}
	return metadata, nil
}

func readID3Frame(frameID string, payload []byte, metadata *localBookMetadata) {
	text := id3Text(payload)
	if text == "" {
		return
	}
	switch frameID {
	case "TIT2", "TT2":
		metadata.Title = firstNonEmpty(metadata.Title, text)
	case "TPE1", "TP1":
		metadata.AuthorName = firstNonEmpty(metadata.AuthorName, text)
		if metadata.AuthorName == text {
			metadata.Authors = append(metadata.Authors, text)
		}
	case "TPE2", "TP2":
		if metadata.AuthorName == "" {
			metadata.AuthorName = text
			metadata.Authors = append(metadata.Authors, text)
		}
	case "TALB", "TAL":
		metadata.Album = firstNonEmpty(metadata.Album, text)
		metadata.Series = firstNonEmpty(metadata.Series, text)
	case "TCON", "TCO":
		metadata.Subjects = append(metadata.Subjects, text)
	case "TDRC", "TYER", "TYE":
		metadata.Year = firstNonEmpty(metadata.Year, text)
	case "TRCK", "TRK":
		metadata.Track = firstNonEmpty(metadata.Track, text)
	case "COMM", "COM":
		metadata.Description = firstNonEmpty(metadata.Description, text)
	case "TSRC":
		addIdentifier(metadata.Identifiers, text, "isrc")
	}
}

func id3Text(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	encoding := payload[0]
	data := payload[1:]
	switch encoding {
	case 1, 2:
		return decodeUTF16Text(data)
	case 3:
		return cleanTagText(string(data))
	default:
		return cleanTagText(string(data))
	}
}

func decodeUTF16Text(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	var order binary.ByteOrder = binary.BigEndian
	if data[0] == 0xff && data[1] == 0xfe {
		order = binary.LittleEndian
		data = data[2:]
	} else if data[0] == 0xfe && data[1] == 0xff {
		data = data[2:]
	}
	if len(data)%2 == 1 {
		data = data[:len(data)-1]
	}
	values := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		value := order.Uint16(data[i : i+2])
		if value == 0 {
			continue
		}
		values = append(values, value)
	}
	return cleanTagText(string(utf16.Decode(values)))
}

func cleanTagText(value string) string {
	value = strings.ReplaceAll(value, "\x00", " ")
	value = strings.ReplaceAll(value, "\u0000", " ")
	return strings.Join(strings.Fields(value), " ")
}

func syncSafeInt(value []byte) int {
	if len(value) < 4 {
		return 0
	}
	return int(value[0]&0x7f)<<21 | int(value[1]&0x7f)<<14 | int(value[2]&0x7f)<<7 | int(value[3]&0x7f)
}

func parseMP4Metadata(file *os.File) (localBookMetadata, error) {
	info, err := file.Stat()
	if err != nil {
		return localBookMetadata{}, err
	}
	metadata := localBookMetadata{Identifiers: map[string]string{}}
	if err := walkMP4Atoms(file, 0, info.Size(), 0, &metadata); err != nil {
		return metadata, err
	}
	if len(metadata.Identifiers) == 0 {
		metadata.Identifiers = nil
	}
	return metadata, nil
}

func walkMP4Atoms(reader io.ReaderAt, start int64, end int64, depth int, metadata *localBookMetadata) error {
	if depth > 8 || start < 0 || end <= start {
		return nil
	}
	offset := start
	header := make([]byte, 16)
	for offset+8 <= end {
		if _, err := reader.ReadAt(header[:8], offset); err != nil {
			return err
		}
		size := int64(binary.BigEndian.Uint32(header[:4]))
		atomType := string(header[4:8])
		headerSize := int64(8)
		if size == 1 {
			if _, err := reader.ReadAt(header[:16], offset); err != nil {
				return err
			}
			size = int64(binary.BigEndian.Uint64(header[8:16]))
			headerSize = 16
		} else if size == 0 {
			size = end - offset
		}
		if size < headerSize || offset+size > end {
			break
		}
		payloadStart := offset + headerSize
		payloadEnd := offset + size
		switch atomType {
		case "moov", "udta", "ilst":
			if err := walkMP4Atoms(reader, payloadStart, payloadEnd, depth+1, metadata); err != nil {
				return err
			}
		case "meta":
			if payloadStart+4 <= payloadEnd {
				if err := walkMP4Atoms(reader, payloadStart+4, payloadEnd, depth+1, metadata); err != nil {
					return err
				}
			}
		case "\xa9nam", "\xa9ART", "aART", "\xa9alb", "\xa9gen", "\xa9day", "desc", "\xa9des", "trkn":
			if err := readMP4MetadataItem(reader, atomType, payloadStart, payloadEnd, metadata); err != nil {
				return err
			}
		}
		offset += size
	}
	return nil
}

func readMP4MetadataItem(reader io.ReaderAt, atomType string, start int64, end int64, metadata *localBookMetadata) error {
	offset := start
	header := make([]byte, 16)
	for offset+16 <= end {
		if _, err := reader.ReadAt(header[:8], offset); err != nil {
			return err
		}
		size := int64(binary.BigEndian.Uint32(header[:4]))
		childType := string(header[4:8])
		headerSize := int64(8)
		if size == 1 {
			if _, err := reader.ReadAt(header[:16], offset); err != nil {
				return err
			}
			size = int64(binary.BigEndian.Uint64(header[8:16]))
			headerSize = 16
		}
		if size < headerSize || offset+size > end {
			break
		}
		if childType == "data" {
			dataStart := offset + headerSize + 8
			if dataStart > offset+size {
				return nil
			}
			data := make([]byte, offset+size-dataStart)
			if _, err := reader.ReadAt(data, dataStart); err != nil {
				return err
			}
			applyMP4MetadataValue(atomType, data, metadata)
			return nil
		}
		offset += size
	}
	return nil
}

func applyMP4MetadataValue(atomType string, data []byte, metadata *localBookMetadata) {
	text := cleanTagText(string(data))
	if atomType == "trkn" {
		metadata.Track = firstNonEmpty(metadata.Track, mp4TrackNumber(data))
		return
	}
	if text == "" {
		return
	}
	switch atomType {
	case "\xa9nam":
		metadata.Title = firstNonEmpty(metadata.Title, text)
	case "\xa9ART", "aART":
		metadata.AuthorName = firstNonEmpty(metadata.AuthorName, text)
		if metadata.AuthorName == text {
			metadata.Authors = append(metadata.Authors, text)
		}
	case "\xa9alb":
		metadata.Album = firstNonEmpty(metadata.Album, text)
		metadata.Series = firstNonEmpty(metadata.Series, text)
	case "\xa9gen":
		metadata.Subjects = append(metadata.Subjects, text)
	case "\xa9day":
		metadata.Year = firstNonEmpty(metadata.Year, text)
	case "desc", "\xa9des":
		metadata.Description = firstNonEmpty(metadata.Description, text)
	}
}

func mp4TrackNumber(data []byte) string {
	if len(data) < 4 {
		return ""
	}
	for i := 0; i+3 < len(data); i++ {
		track := binary.BigEndian.Uint16(data[i+2 : i+4])
		if track > 0 {
			return strconv.Itoa(int(track))
		}
	}
	return ""
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
	if metadata.Album != "" {
		local["album"] = metadata.Album
	}
	if metadata.Year != "" {
		local["year"] = metadata.Year
	}
	if metadata.Track != "" {
		local["track"] = metadata.Track
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
