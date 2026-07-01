package wanted

import (
	"context"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

// AnnotateDownloads fills wanted-item linkage (id, title, author) on download
// rows that carry a `wanted:<id>` tag so queue UIs can show which book a job
// belongs to. Rows without a matching wanted item pass through unchanged, and
// lookup failures degrade to unannotated rows rather than failing the listing.
func (s *Service) AnnotateDownloads(ctx context.Context, downloads []acquisition.DownloadStatus) []acquisition.DownloadStatus {
	if len(downloads) == 0 || !s.Available() {
		return downloads
	}
	tagged := false
	for _, download := range downloads {
		if downloadWantedID(download) != "" {
			tagged = true
			break
		}
	}
	if !tagged {
		return downloads
	}
	items, err := s.store.ListWanted(ctx, "")
	if err != nil {
		return downloads
	}
	return annotateDownloadsWithItems(downloads, items)
}

func annotateDownloadsWithItems(downloads []acquisition.DownloadStatus, items []WantedItem) []acquisition.DownloadStatus {
	byID := make(map[string]WantedItem, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	for i := range downloads {
		item, ok := byID[downloadWantedID(downloads[i])]
		if !ok {
			continue
		}
		downloads[i].WantedID = item.ID
		downloads[i].WantedTitle = item.Title
		downloads[i].WantedAuthor = item.AuthorName
	}
	return downloads
}

func downloadWantedID(download acquisition.DownloadStatus) string {
	for _, tag := range download.Tags {
		tag = strings.TrimSpace(tag)
		if !strings.HasPrefix(tag, "wanted:") {
			continue
		}
		if id := strings.TrimSpace(strings.TrimPrefix(tag, "wanted:")); id != "" {
			return id
		}
	}
	return ""
}
