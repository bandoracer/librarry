package wanted

import (
	"context"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

// Derived book states use recorded file/import evidence and fresh download
// responses. Incomplete or unavailable evidence remains explicit instead of
// deriving certainty from the stored lifecycle status.
const (
	DerivedStateUnmonitored = "unmonitored"
	DerivedStateMissing     = "missing"
	DerivedStateDownloading = "downloading"
	DerivedStateDownloaded  = "downloaded"
	DerivedStateCutoffUnmet = "cutoffUnmet"
	DerivedStateUnknown     = "unknown"
	DerivedStateIncomplete  = "incomplete"
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

// BookStateEvidence keeps uncertainty separate from the legacy lifecycle status.
type BookStateEvidence struct {
	Files     FileEvidence `json:"files"`
	Downloads string       `json:"downloads"`
	Quality   string       `json:"quality"`
	Message   string       `json:"message,omitempty"`
}

type liveDownloadEvidenceSource interface {
	LiveDownloadEvidence(context.Context, acquisition.DownloadListQuery) acquisition.DownloadEvidence
}

// AnnotateWantedStates uses page-scoped persisted file evidence and fresh client
// responses. Saved download rows never establish current download presence.
func (s *Service) AnnotateWantedStates(ctx context.Context, items []WantedItem) []WantedItem {
	if len(items) == 0 || !s.Available() {
		return items
	}
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	files, fileErr := s.store.WantedFileEvidence(ctx, ids)
	profiles, profileErr := s.store.ListQualityProfiles(ctx)
	downloads := s.liveBookDownloads(ctx)
	inFlight := groupDownloadsByWantedID(downloads.Downloads)
	for i := range items {
		item := &items[i]
		evidence := BookStateEvidence{Files: files[item.ID], Downloads: downloads.Status, Quality: "available"}
		if fileErr != nil || evidence.Files.State == "" {
			evidence.Files = FileEvidence{State: "unavailable", Reason: "Library file evidence is unavailable."}
		}
		if profileErr != nil {
			evidence.Quality = "unavailable"
		}
		item.DownloadState = downloadPhase(inFlight[item.ID])
		downloading := false
		for _, d := range inFlight[item.ID] {
			if downloadSupportsInFlight(d) {
				downloading = true
				break
			}
		}
		unmet := false
		if evidence.Files.State == "present" && item.Monitored && profileErr == nil {
			profile := profileFromList(profiles, *item)
			unmet = cutoffUnmet(profile, s.currentReleaseScore(ctx, *item))
		}
		item.DerivedState = deriveWantedState(*item, evidence.Files.State == "present", unmet, downloading)
		switch {
		case evidence.Files.State == "present":
		case downloading:
		case evidence.Files.State == "incomplete":
			item.DerivedState = DerivedStateIncomplete
		case evidence.Files.State == "unknown" || evidence.Files.State == "unavailable" || downloads.Status == "partial" || downloads.Status == "unavailable":
			item.DerivedState = DerivedStateUnknown
		}
		if evidence.Files.State != "present" && evidence.Files.State != "missing" {
			evidence.Message = evidence.Files.Reason
		}
		if downloads.Status == "partial" || downloads.Status == "unavailable" {
			if evidence.Message != "" {
				evidence.Message += " "
			}
			evidence.Message += "Download-client evidence is unavailable or incomplete."
		}
		if profileErr != nil {
			if evidence.Message != "" {
				evidence.Message += " "
			}
			evidence.Message += "Quality cutoff could not be checked."
		}
		item.StateEvidence = &evidence
	}
	return items
}

func profileFromList(profiles []QualityProfile, item WantedItem) QualityProfile {
	name, format := normalizeQualityProfile(item.QualityProfile), normalizeProfileFormat(item.Format)
	for _, preferred := range []string{format, "any"} {
		for _, profile := range profiles {
			if profile.Name == name && profile.MediaFormat == preferred {
				return normalizeProfile(profile, item)
			}
		}
	}
	return defaultQualityProfile(item.QualityProfile, item.Format)
}

func downloadSupportsInFlight(d acquisition.DownloadStatus) bool {
	state := strings.ToLower(strings.TrimSpace(d.State))
	return d.ImportStatus != "imported" && d.ImportStatus != "removed" && state != "" && !strings.Contains(state, "error") && !strings.Contains(state, "missing") && !strings.Contains(state, "fail") && state != "removed" && state != "deleted"
}

func (s *Service) liveBookDownloads(ctx context.Context) acquisition.DownloadEvidence {
	downloads := acquisition.DownloadEvidence{Status: "notConfigured"}
	if s.acquire != nil {
		downloads.Status = "unavailable"
		if source, ok := s.acquire.(liveDownloadEvidenceSource); ok {
			liveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			downloads = source.LiveDownloadEvidence(liveCtx, acquisition.DownloadListQuery{Tag: "librarry"})
			cancel()
		}
	}
	return downloads
}
