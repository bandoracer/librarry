package wanted

import (
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

// Only live observations establish client progress. Persisted review decisions
// are attached separately and can remain actionable when the client is offline.
func downloadPhase(downloads []acquisition.DownloadStatus) string {
	phase, priority := "", 0
	for _, d := range downloads {
		if d.ImportStatus == "imported" || d.ImportStatus == "removed" {
			continue
		}
		state := strings.ToLower(d.State)
		next, rank := "queued", 2
		switch {
		case d.ImportError != "" || d.FailureReason != "" || strings.Contains(state, "error") || strings.Contains(state, "missing") || strings.Contains(state, "failed"):
			next, rank = "failed", 1
		case d.Progress >= 1 || d.ImportStatus == "ready":
			next, rank = "import_ready", 7
		case strings.Contains(state, "stalled") && strings.Contains(state, "dl"):
			next, rank = "stalled", 5
		case state == "metadl" || state == "forcedmetadl" || state == "downloading_metadata":
			next, rank = "waiting_metadata", 4
		case strings.Contains(state, "paused") || strings.Contains(state, "stopped"):
			next, rank = "paused", 3
		case state == "downloading" || state == "forceddl":
			next, rank = "downloading", 6
		}
		if rank > priority {
			phase, priority = next, rank
		}
	}
	return phase
}
