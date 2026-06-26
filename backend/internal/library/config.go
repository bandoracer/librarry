package library

import (
	"strings"

	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
)

func NormalizeConfig(config Config) Config {
	config.EbookRoot = firstNonEmpty(strings.TrimSpace(config.EbookRoot), "/data/media/books/ebooks")
	config.AudiobookRoot = firstNonEmpty(strings.TrimSpace(config.AudiobookRoot), "/data/media/books/audiobooks")
	config.NamingAuthorFolderTemplate = firstNonEmpty(strings.TrimSpace(config.NamingAuthorFolderTemplate), "{Author}")
	config.NamingBookFolderTemplate = firstNonEmpty(strings.TrimSpace(config.NamingBookFolderTemplate), "{Title}")
	config.NamingFileNameTemplate = firstNonEmpty(strings.TrimSpace(config.NamingFileNameTemplate), "{Title}{Ext}")
	config.NamingSpaceReplacement = strings.TrimSpace(config.NamingSpaceReplacement)
	return config
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
