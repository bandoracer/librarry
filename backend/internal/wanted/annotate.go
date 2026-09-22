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
	downloads = annotateDownloadsWithItems(downloads, items)
	ids := make([]string, len(downloads))
	for i, download := range downloads {
		ids[i] = download.ID
	}
	rows, err := s.store.db.QueryContext(ctx, `select download_id,coalesce(metadata->>'downloadClient',''),id::text,reason from import_reviews where status='pending' and download_id=any($1::text[]) order by created_at,id`, ids)
	if err != nil {
		return downloads
	}
	defer rows.Close()
	for rows.Next() {
		var downloadID, client, reviewID, reason string
		if rows.Scan(&downloadID, &client, &reviewID, &reason) != nil {
			return downloads
		}
		for i := range downloads {
			if downloads[i].ID == downloadID && strings.EqualFold(downloads[i].Client, client) && downloads[i].ImportStatus != "imported" {
				downloads[i].ImportReviewID, downloads[i].ImportReviewReason = reviewID, reason
			}
		}
	}
	return downloads
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
