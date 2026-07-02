package wanted

import (
	"context"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

// Derived book states, Readarr-style: presence is computed from monitored +
// tracked file + profile cutoff + live downloads instead of the stored
// lifecycle status, so a vanished download can never strand a book.
const (
	DerivedStateUnmonitored = "unmonitored"
	DerivedStateMissing     = "missing"
	DerivedStateDownloading = "downloading"
	DerivedStateDownloaded  = "downloaded"
	DerivedStateCutoffUnmet = "cutoffUnmet"
)

func deriveWantedState(item WantedItem, hasFile bool, cutoffUnmet bool, downloading bool) string {
	if hasFile {
		if cutoffUnmet {
			return DerivedStateCutoffUnmet
		}
		return DerivedStateDownloaded
	}
	if downloading {
		return DerivedStateDownloading
	}
	if !item.Monitored {
		return DerivedStateUnmonitored
	}
	return DerivedStateMissing
}

// AnnotateWantedStates fills DerivedState on wanted items. File presence and
// cutoff come from the database; "downloading" comes from live
// librarry-tagged client downloads and degrades to absent when the client is
// unreachable (a book never looks in-flight on stale evidence).
func (s *Service) AnnotateWantedStates(ctx context.Context, items []WantedItem) []WantedItem {
	if len(items) == 0 || !s.Available() {
		return items
	}
	withFiles, err := s.store.WantedIDsWithFiles(ctx)
	if err != nil {
		return items
	}
	cutoffUnmet := map[string]bool{}
	if unmet, err := s.ListCutoffUnmet(ctx); err == nil {
		for _, item := range unmet {
			cutoffUnmet[item.ID] = true
		}
	}
	downloading := map[string]bool{}
	if s.acquire != nil {
		if downloads, err := s.acquire.Downloads(ctx, acquisition.DownloadListQuery{Tag: "librarry"}); err == nil {
			for id := range groupDownloadsByWantedID(downloads) {
				downloading[id] = true
			}
		}
	}
	for i := range items {
		items[i].DerivedState = deriveWantedState(items[i], withFiles[items[i].ID], cutoffUnmet[items[i].ID], downloading[items[i].ID])
	}
	return items
}
