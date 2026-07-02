package library

import (
	"fmt"
	"strings"
	"time"

	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
)

const defaultRecycleBinRetention = 168 * time.Hour

func NormalizeConfig(config Config) Config {
	config.EbookRoot = firstNonEmpty(strings.TrimSpace(config.EbookRoot), "/data/media/books/ebooks")
	config.AudiobookRoot = firstNonEmpty(strings.TrimSpace(config.AudiobookRoot), "/data/media/books/audiobooks")
	config.NamingAuthorFolderTemplate = firstNonEmpty(strings.TrimSpace(config.NamingAuthorFolderTemplate), "{Author}")
	config.NamingBookFolderTemplate = firstNonEmpty(strings.TrimSpace(config.NamingBookFolderTemplate), "{Title}")
	config.NamingFileNameTemplate = firstNonEmpty(strings.TrimSpace(config.NamingFileNameTemplate), "{Title}{Ext}")
	config.NamingSpaceReplacement = strings.TrimSpace(config.NamingSpaceReplacement)
	config.StandardSearchLanguage = firstNonEmpty(strings.TrimSpace(config.StandardSearchLanguage), "English")
	config.RecycleBin = strings.TrimSpace(config.RecycleBin)
	if config.RecycleBinRetention <= 0 {
		config.RecycleBinRetention = defaultRecycleBinRetention
	}
	config.ImportExtraFiles = firstNonEmpty(strings.TrimSpace(config.ImportExtraFiles), ".cue")
	return config
}

func ConfigWithNamingRecord(config Config, payload map[string]any) Config {
	config = NormalizeConfig(config)
	if payload == nil {
		return config
	}
	config.NamingAuthorFolderTemplate = firstNonEmpty(
		payloadString(payload, "librarryAuthorFolderTemplate"),
		payloadString(payload, "authorFolderFormat"),
		payloadString(payload, "authorFolderTemplate"),
		config.NamingAuthorFolderTemplate,
	)
	config.NamingBookFolderTemplate = firstNonEmpty(
		payloadString(payload, "librarryBookFolderTemplate"),
		payloadString(payload, "bookFolderFormat"),
		payloadString(payload, "bookFolderTemplate"),
		config.NamingBookFolderTemplate,
	)
	config.NamingFileNameTemplate = firstNonEmpty(
		payloadString(payload, "librarryFileNameTemplate"),
		payloadString(payload, "standardBookFormat"),
		payloadString(payload, "fileNameFormat"),
		config.NamingFileNameTemplate,
	)
	if value, ok := payload["replaceSpaces"]; ok && !payloadBool(value) {
		config.NamingSpaceReplacement = ""
	} else if replacement := payloadString(payload, "replaceSpacesWith"); replacement != "" {
		config.NamingSpaceReplacement = replacement
	}
	config.StandardSearchLanguage = firstNonEmpty(
		payloadString(payload, "librarryStandardSearchLanguage"),
		payloadString(payload, "standardSearchLanguage"),
		config.StandardSearchLanguage,
	)
	return NormalizeConfig(config)
}

func ConfigWithRootFolders(config Config, roots []compatdata.RootFolder) Config {
	config = NormalizeConfig(config)
	if root := preferredRootForFormat(roots, "ebook"); root != "" {
		config.EbookRoot = root
	}
	if root := preferredRootForFormat(roots, "audiobook"); root != "" {
		config.AudiobookRoot = root
	}
	return config
}

func payloadString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func payloadBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func preferredRootForFormat(roots []compatdata.RootFolder, format string) string {
	for _, root := range roots {
		if !metadataBool(root.Metadata, "librarryLibraryRoot", false) || !rootFormatMatches(root, format) {
			continue
		}
		if path := strings.TrimSpace(root.Path); path != "" {
			return path
		}
	}
	for _, root := range roots {
		if !rootFormatMatches(root, format) {
			continue
		}
		if path := strings.TrimSpace(root.Path); path != "" {
			return path
		}
	}
	return ""
}

func rootFormatMatches(root compatdata.RootFolder, format string) bool {
	mediaFormat := strings.ToLower(strings.TrimSpace(root.MediaFormat))
	if mediaFormat == format {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(root.Name))
	switch format {
	case "ebook":
		return mediaFormat == "books" || strings.Contains(name, "ebook")
	case "audiobook":
		return strings.Contains(name, "audio")
	default:
		return false
	}
}
