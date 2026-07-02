package library

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// copyImportExtras copies (hardlink-or-copy) sibling files that share the
// imported source's basename and carry one of the configured extra extensions
// (default ".cue") into the destination folder, renamed to match the imported
// file. Nothing is tracked; failures only log at debug.
func (s *Service) copyImportExtras(source string, destination string) {
	extensions := importExtraExtensions(s.Config().ImportExtraFiles)
	if len(extensions) == 0 {
		return
	}
	extras, err := siblingExtraFiles(source, extensions)
	if err != nil {
		slog.Debug("import extras lookup failed", "source", source, "error", err)
		return
	}
	if len(extras) == 0 {
		return
	}
	destinationDir := filepath.Dir(destination)
	destinationBase := strings.TrimSuffix(filepath.Base(destination), filepath.Ext(destination))
	for _, extra := range extras {
		target := filepath.Join(destinationDir, destinationBase+strings.ToLower(filepath.Ext(extra)))
		if filepath.Clean(extra) == filepath.Clean(target) {
			continue
		}
		if _, err := os.Stat(target); err == nil {
			slog.Debug("import extra already present", "source", extra, "target", target)
			continue
		}
		if _, err := importFile(extra, target, "hardlinkOrCopy", false); err != nil {
			slog.Debug("import extra copy failed", "source", extra, "target", target, "error", err)
			continue
		}
		slog.Debug("imported extra file", "source", extra, "target", target)
	}
}

// siblingExtraFiles finds files next to source that share its basename and
// carry one of the wanted extensions (case-insensitive).
func siblingExtraFiles(source string, extensions []string) ([]string, error) {
	source = filepath.Clean(strings.TrimSpace(source))
	if source == "" || source == "." {
		return nil, nil
	}
	wanted := map[string]bool{}
	for _, ext := range extensions {
		if ext != "" {
			wanted[ext] = true
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}
	dir := filepath.Dir(source)
	base := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var extras []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !wanted[ext] {
			continue
		}
		if !strings.EqualFold(strings.TrimSuffix(name, filepath.Ext(name)), base) {
			continue
		}
		candidate := filepath.Join(dir, name)
		if candidate == source {
			continue
		}
		extras = append(extras, candidate)
	}
	return extras, nil
}

// importExtraExtensions parses the comma-separated extra-file extension list
// (".cue,.nfo") into normalized lowercase dot-prefixed extensions.
func importExtraExtensions(value string) []string {
	parts := strings.Split(value, ",")
	extensions := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		ext := strings.ToLower(strings.TrimSpace(part))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if ext == "." || seen[ext] {
			continue
		}
		seen[ext] = true
		extensions = append(extensions, ext)
	}
	return extensions
}
