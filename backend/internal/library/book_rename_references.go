package library

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var cueReference = regexp.MustCompile(`(?im)^\s*FILE\s+(?:"([^"]+)"|(\S+))\s+\S+\s*$`)

func verifyBookRenameReferences(path, root string, files map[string]ImportOperationFile) error {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".cue" && ext != ".m3u" && ext != ".m3u8" && ext != ".opf" {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 4<<20+1))
	if err != nil {
		return err
	}
	if len(data) > 4<<20 || !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
		return errors.New("companion encoding or size requires review before folder rename")
	}
	text := strings.TrimPrefix(string(data), "\ufeff")
	check := func(reference string) error {
		reference = strings.ReplaceAll(strings.TrimSpace(reference), `\`, "/")
		if reference == "" || strings.Contains(reference, ":") || strings.HasPrefix(reference, "/") {
			return fmt.Errorf("companion uses an absolute or unsupported file reference: %s", path)
		}
		resolved := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(reference)))
		if !pathWithinRoot(resolved, root) {
			return fmt.Errorf("companion reference leaves the book folder: %s", path)
		}
		if _, ok := files[resolved]; !ok {
			return fmt.Errorf("companion references an unrecorded book file: %s", reference)
		}
		return nil
	}
	switch ext {
	case ".cue":
		matches := cueReference.FindAllStringSubmatch(text, -1)
		directives := 0
		for _, line := range strings.Split(text, "\n") {
			parts := strings.Fields(line)
			if len(parts) > 0 && strings.EqualFold(parts[0], "FILE") {
				directives++
			}
		}
		if directives != len(matches) {
			return errors.New("CUE contains unsupported file references; retain the book folder for review")
		}

		if len(matches) == 0 {
			return errors.New("CUE references could not be verified; retain the book folder for review")
		}
		for _, match := range matches {
			if err := check(firstNonEmpty(match[1], match[2])); err != nil {
				return err
			}
		}
	case ".m3u", ".m3u8":
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				if err := check(line); err != nil {
					return err
				}
			}
		}
	case ".opf":
		decoder := xml.NewDecoder(strings.NewReader(text))
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("OPF references could not be verified: %w", err)
			}
			start, ok := token.(xml.StartElement)
			if !ok || (start.Name.Local != "item" && start.Name.Local != "reference") {
				continue
			}
			for _, attr := range start.Attr {
				if attr.Name.Local != "href" {
					continue
				}
				ref := strings.SplitN(attr.Value, "#", 2)[0]
				if ref == "" || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://") {
					continue
				}
				if err := check(ref); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
