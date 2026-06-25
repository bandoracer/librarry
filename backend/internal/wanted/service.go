package wanted

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

const (
	defaultWantedMonitorLimit       = 50
	defaultWantedMonitorSearchLimit = 20
	defaultWantedSearchInterval     = 6 * time.Hour
	defaultFailedDownloadLimit      = 50
	defaultFailedDownloadStalledAge = 24 * time.Hour
	defaultUpgradeSearchInterval    = 12 * time.Hour
	defaultUpgradeScoreDelta        = 5.0
)

type Service struct {
	store   *Store
	acquire Acquisition
}

func NewService(store *Store, acquire Acquisition) *Service {
	return &Service{store: store, acquire: acquire}
}

func (s *Service) Available() bool {
	return s != nil && s.store != nil && s.store.Configured()
}

func (s *Service) Create(ctx context.Context, request CreateRequest) (WantedItem, error) {
	if !s.Available() {
		return WantedItem{}, errors.New("wanted service requires database persistence")
	}
	return s.store.CreateWanted(ctx, request)
}

func (s *Service) List(ctx context.Context, status string) ([]WantedItem, error) {
	if !s.Available() {
		return nil, errors.New("wanted service requires database persistence")
	}
	return s.store.ListWanted(ctx, status)
}

func (s *Service) SearchReleases(ctx context.Context, wantedID string, request SearchReleasesRequest) (SearchOutcome, error) {
	if !s.Available() {
		return SearchOutcome{}, errors.New("wanted service requires database persistence")
	}
	if s.acquire == nil {
		return SearchOutcome{}, errors.New("acquisition service is unavailable")
	}
	item, err := s.store.GetWanted(ctx, wantedID)
	if err != nil {
		return SearchOutcome{}, err
	}
	return s.searchReleasesForItem(ctx, item, request)
}

func (s *Service) searchReleasesForItem(ctx context.Context, item WantedItem, request SearchReleasesRequest) (SearchOutcome, error) {
	limit := request.Limit
	if limit <= 0 || limit > 50 {
		limit = defaultWantedMonitorSearchLimit
	}
	queryText := item.Title
	if strings.TrimSpace(item.AuthorName) != "" {
		queryText += " " + item.AuthorName
	}
	releases, err := s.acquire.Search(ctx, acquisition.ReleaseSearchQuery{
		Query:  queryText,
		Author: item.AuthorName,
		Format: item.Format,
		Limit:  limit,
	})
	if err != nil {
		return SearchOutcome{}, err
	}
	decisions := make([]ReleaseDecision, 0, len(releases))
	for _, release := range releases {
		decisions = append(decisions, evaluateRelease(item, release))
	}
	sort.SliceStable(decisions, func(i, j int) bool {
		if decisions[i].Approved != decisions[j].Approved {
			return decisions[i].Approved
		}
		if decisions[i].Score == decisions[j].Score {
			return decisions[i].Seeders > decisions[j].Seeders
		}
		return decisions[i].Score > decisions[j].Score
	})
	stored, err := s.store.UpsertReleaseDecisions(ctx, item.ID, decisions)
	if err != nil {
		return SearchOutcome{}, err
	}
	item, _ = s.store.GetWanted(ctx, item.ID)
	return SearchOutcome{WantedItem: item, Releases: stored}, nil
}

func (s *Service) ListReleases(ctx context.Context, wantedID string) (SearchOutcome, error) {
	if !s.Available() {
		return SearchOutcome{}, errors.New("wanted service requires database persistence")
	}
	item, err := s.store.GetWanted(ctx, wantedID)
	if err != nil {
		return SearchOutcome{}, err
	}
	releases, err := s.store.ListReleaseDecisions(ctx, wantedID)
	if err != nil {
		return SearchOutcome{}, err
	}
	return SearchOutcome{WantedItem: item, Releases: releases}, nil
}

func (s *Service) Grab(ctx context.Context, wantedID string, request GrabRequest) (acquisition.DownloadStatus, error) {
	if !s.Available() {
		return acquisition.DownloadStatus{}, errors.New("wanted service requires database persistence")
	}
	if s.acquire == nil {
		return acquisition.DownloadStatus{}, errors.New("acquisition service is unavailable")
	}
	item, err := s.store.GetWanted(ctx, wantedID)
	if err != nil {
		return acquisition.DownloadStatus{}, err
	}
	release, err := s.pickRelease(ctx, wantedID, request.ReleaseID)
	if err != nil {
		return acquisition.DownloadStatus{}, err
	}
	if !release.Approved {
		return acquisition.DownloadStatus{}, errors.New("release is rejected: " + release.RejectedReason)
	}
	return s.grabRelease(ctx, item, release, request.Paused, "manual")
}

func (s *Service) Monitor(ctx context.Context, request MonitorRequest) (MonitorRun, error) {
	if !s.Available() {
		return MonitorRun{}, errors.New("wanted service requires database persistence")
	}
	if s.acquire == nil {
		return MonitorRun{}, errors.New("acquisition service is unavailable")
	}

	run, err := s.store.StartMonitorRun(ctx, request.Trigger)
	if err != nil {
		return MonitorRun{}, err
	}

	limit := request.Limit
	if limit <= 0 || limit > 200 {
		limit = defaultWantedMonitorLimit
	}
	searchLimit := request.SearchLimit
	if searchLimit <= 0 || searchLimit > 50 {
		searchLimit = defaultWantedMonitorSearchLimit
	}
	minSearchInterval := defaultWantedSearchInterval
	if request.MinSearchIntervalMinutes > 0 {
		minSearchInterval = time.Duration(request.MinSearchIntervalMinutes) * time.Minute
	}

	items, err := s.store.ListDueWanted(ctx, limit, minSearchInterval, request.Force)
	if err != nil {
		run.Status = "failed"
		run.ErrorCount = 1
		run.Message = err.Error()
		finished, finishErr := s.store.FinishMonitorRun(ctx, run)
		if finishErr != nil {
			return MonitorRun{}, finishErr
		}
		return finished, err
	}

	for _, item := range items {
		select {
		case <-ctx.Done():
			run.Status = "canceled"
			run.Message = ctx.Err().Error()
			return s.store.FinishMonitorRun(context.Background(), run)
		default:
		}

		result := MonitorItemResult{WantedItem: item}
		outcome, err := s.searchReleasesForItem(ctx, item, SearchReleasesRequest{Limit: searchLimit})
		run.WantedChecked++
		if err != nil {
			result.Error = err.Error()
			run.ErrorCount++
			_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
				EventType:  "wanted_search_failed",
				EntityType: "wanted_item",
				EntityID:   item.ID,
				Severity:   "error",
				Message:    "Wanted release search failed for " + item.Title,
				Data: map[string]any{
					"error": err.Error(),
					"title": item.Title,
				},
			})
			run.Items = append(run.Items, result)
			continue
		}

		result.WantedItem = outcome.WantedItem
		result.ReleasesFound = len(outcome.Releases)
		run.ReleasesFound += len(outcome.Releases)
		for _, release := range outcome.Releases {
			if release.Approved {
				result.ApprovedCount++
				run.ApprovedCount++
			} else {
				result.RejectedCount++
				run.RejectedCount++
			}
		}
		_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
			EventType:  "wanted_searched",
			EntityType: "wanted_item",
			EntityID:   item.ID,
			Severity:   "info",
			Message:    "Searched wanted releases for " + item.Title,
			Data: map[string]any{
				"title":          item.Title,
				"releasesFound":  result.ReleasesFound,
				"approvedCount":  result.ApprovedCount,
				"rejectedCount":  result.RejectedCount,
				"monitorRunId":   run.ID,
				"qualityProfile": item.QualityProfile,
			},
		})

		if request.AutoGrab {
			if release, ok := firstApproved(outcome.Releases); ok {
				status, err := s.grabRelease(ctx, outcome.WantedItem, release, request.Paused, "monitor")
				if err != nil {
					result.Error = err.Error()
					run.ErrorCount++
					_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
						EventType:  "wanted_grab_failed",
						EntityType: "wanted_item",
						EntityID:   item.ID,
						Severity:   "error",
						Message:    "Approved release grab failed for " + item.Title,
						Data: map[string]any{
							"error":     err.Error(),
							"releaseId": release.ID,
							"title":     release.Title,
						},
					})
				} else {
					result.GrabbedDownload = &status
					run.GrabbedCount++
				}
			}
		}

		run.Items = append(run.Items, result)
	}

	if run.ErrorCount > 0 {
		run.Status = "partial_failed"
	} else {
		run.Status = "completed"
	}
	run.Message = monitorMessage(run)
	return s.store.FinishMonitorRun(ctx, run)
}

func (s *Service) FeedSync(ctx context.Context, request FeedSyncRequest) (FeedSyncRun, error) {
	if !s.Available() {
		return FeedSyncRun{}, errors.New("wanted service requires database persistence")
	}
	if s.acquire == nil {
		return FeedSyncRun{}, errors.New("acquisition service is unavailable")
	}

	run, err := s.store.StartFeedSyncRun(ctx, request.Trigger)
	if err != nil {
		return FeedSyncRun{}, err
	}
	limit := request.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	releases, err := s.acquire.Feed(ctx, acquisition.ReleaseFeedQuery{
		Format: request.Format,
		Limit:  limit,
	})
	if err != nil {
		run.Status = "failed"
		run.ErrorCount = 1
		run.Message = err.Error()
		finished, finishErr := s.store.FinishFeedSyncRun(ctx, run)
		if finishErr != nil {
			return FeedSyncRun{}, finishErr
		}
		return finished, err
	}
	run.ReleasesSeen = len(releases)
	if err := s.store.UpsertFeedReleases(ctx, releases); err != nil {
		run.Status = "failed"
		run.ErrorCount = 1
		run.Message = err.Error()
		finished, finishErr := s.store.FinishFeedSyncRun(ctx, run)
		if finishErr != nil {
			return FeedSyncRun{}, finishErr
		}
		return finished, err
	}

	items, err := s.store.ListWanted(ctx, "wanted")
	if err != nil {
		run.Status = "failed"
		run.ErrorCount = 1
		run.Message = err.Error()
		finished, finishErr := s.store.FinishFeedSyncRun(ctx, run)
		if finishErr != nil {
			return FeedSyncRun{}, finishErr
		}
		return finished, err
	}

	grabbedWanted := map[string]bool{}
	for _, item := range items {
		if !formatMatchesRequest(request.Format, item.Format) {
			continue
		}
		var decisions []ReleaseDecision
		for _, release := range releases {
			if !feedReleaseMatchesWanted(item, release) {
				continue
			}
			decisions = append(decisions, evaluateRelease(item, release))
		}
		if len(decisions) == 0 {
			continue
		}
		sort.SliceStable(decisions, func(i, j int) bool {
			if decisions[i].Approved != decisions[j].Approved {
				return decisions[i].Approved
			}
			if decisions[i].Score == decisions[j].Score {
				return decisions[i].Seeders > decisions[j].Seeders
			}
			return decisions[i].Score > decisions[j].Score
		})
		stored, err := s.store.UpsertReleaseDecisions(ctx, item.ID, decisions)
		if err != nil {
			run.ErrorCount++
			run.Matches = append(run.Matches, FeedSyncMatch{WantedItem: item, Error: err.Error()})
			continue
		}
		for _, decision := range stored {
			match := FeedSyncMatch{WantedItem: item, Release: decision}
			run.MatchedCount++
			if decision.Approved {
				run.ApprovedCount++
			} else {
				run.RejectedCount++
			}
			if request.AutoGrab && decision.Approved && !grabbedWanted[item.ID] {
				status, err := s.grabRelease(ctx, item, decision, request.Paused, "feed")
				if err != nil {
					match.Error = err.Error()
					run.ErrorCount++
					_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
						EventType:  "feed_grab_failed",
						EntityType: "wanted_item",
						EntityID:   item.ID,
						Severity:   "error",
						Message:    "Feed grab failed for " + item.Title,
						Data: map[string]any{
							"error":     err.Error(),
							"releaseId": decision.ID,
							"title":     decision.Title,
						},
					})
				} else {
					grabbedWanted[item.ID] = true
					match.GrabbedDownload = &status
					run.GrabbedCount++
				}
			}
			run.Matches = append(run.Matches, match)
		}
	}

	_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
		EventType:  "feed_synced",
		EntityType: "feed_sync_run",
		EntityID:   run.ID,
		Severity:   "info",
		Message:    "Synced indexer feed",
		Data: map[string]any{
			"releasesSeen":  run.ReleasesSeen,
			"matchedCount":  run.MatchedCount,
			"approvedCount": run.ApprovedCount,
			"grabbedCount":  run.GrabbedCount,
			"errorCount":    run.ErrorCount,
		},
	})
	if run.ErrorCount > 0 {
		run.Status = "partial_failed"
	} else {
		run.Status = "completed"
	}
	run.Message = feedSyncMessage(run)
	return s.store.FinishFeedSyncRun(ctx, run)
}

func (s *Service) RecoverFailedDownloads(ctx context.Context, request FailedDownloadRequest) (FailedDownloadRun, error) {
	if !s.Available() {
		return FailedDownloadRun{}, errors.New("wanted service requires database persistence")
	}
	if s.acquire == nil {
		return FailedDownloadRun{}, errors.New("acquisition service is unavailable")
	}

	run, err := s.store.StartFailedDownloadRun(ctx, request.Trigger)
	if err != nil {
		return FailedDownloadRun{}, err
	}
	limit := request.Limit
	if limit <= 0 || limit > 200 {
		limit = defaultFailedDownloadLimit
	}
	searchLimit := request.SearchLimit
	if searchLimit <= 0 || searchLimit > 50 {
		searchLimit = defaultWantedMonitorSearchLimit
	}
	stalledAge := defaultFailedDownloadStalledAge
	if request.MinStalledMinutes > 0 {
		stalledAge = time.Duration(request.MinStalledMinutes) * time.Minute
	}

	downloads, err := s.acquire.Downloads(ctx, acquisition.DownloadListQuery{
		Tag: "librarry",
		IDs: request.DownloadIDs,
	})
	if err != nil {
		run.Status = "failed"
		run.ErrorCount = 1
		run.Message = err.Error()
		finished, finishErr := s.store.FinishFailedDownloadRun(ctx, run)
		if finishErr != nil {
			return FailedDownloadRun{}, finishErr
		}
		return finished, err
	}

	allowedIDs := stringSet(request.DownloadIDs)
	now := time.Now().UTC()
	for _, download := range downloads {
		if run.DownloadsChecked >= limit {
			break
		}
		if len(allowedIDs) > 0 && !allowedIDs[download.ID] {
			continue
		}
		run.DownloadsChecked++

		reason := failedDownloadReason(download, stalledAge, now)
		if reason == "" && request.Force && len(allowedIDs) > 0 {
			reason = "manual retry requested"
		}
		if reason == "" {
			continue
		}

		result := FailedDownloadResult{Download: download, FailureReason: reason}
		run.FailedCount++
		if err := s.acquire.MarkDownloadFailed(ctx, download.ID, reason); err != nil {
			result.Error = err.Error()
			run.ErrorCount++
			run.Items = append(run.Items, result)
			continue
		}

		wantedID := wantedIDFromTags(download.Tags)
		if wantedID == "" {
			result.Error = "download is not linked to a wanted item"
			run.ErrorCount++
			run.Items = append(run.Items, result)
			continue
		}
		item, err := s.store.GetWanted(ctx, wantedID)
		if err != nil {
			result.Error = err.Error()
			run.ErrorCount++
			run.Items = append(run.Items, result)
			continue
		}
		result.WantedItem = item
		_ = s.store.MarkWantedStatus(ctx, item.ID, "wanted")
		_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
			EventType:  "download_failed",
			EntityType: "wanted_item",
			EntityID:   item.ID,
			Severity:   "warning",
			Message:    "Download failed for " + item.Title,
			Data: map[string]any{
				"downloadId": download.ID,
				"name":       download.Name,
				"state":      download.State,
				"reason":     reason,
				"runId":      run.ID,
			},
		})

		outcome, err := s.searchReleasesForItem(ctx, item, SearchReleasesRequest{Limit: searchLimit})
		if err != nil {
			result.Error = err.Error()
			run.ErrorCount++
			run.Items = append(run.Items, result)
			continue
		}
		result.WantedItem = outcome.WantedItem
		result.ReplacementReleases = outcome.Releases
		replacement, ok := firstApprovedReplacement(outcome.Releases, download)
		if ok {
			run.ReplacementsFound++
			result.ReplacementRelease = &replacement
		}

		if request.AutoGrab {
			if !ok {
				result.Error = "no approved replacement release is available"
				run.ErrorCount++
			} else {
				status, err := s.grabRelease(ctx, outcome.WantedItem, replacement, request.Paused, "failed-download")
				if err != nil {
					result.Error = err.Error()
					run.ErrorCount++
					_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
						EventType:  "failed_download_replacement_grab_failed",
						EntityType: "wanted_item",
						EntityID:   item.ID,
						Severity:   "error",
						Message:    "Failed download replacement grab failed for " + item.Title,
						Data: map[string]any{
							"downloadId": download.ID,
							"releaseId":  replacement.ID,
							"title":      replacement.Title,
							"error":      err.Error(),
						},
					})
				} else {
					result.ReplacementDownload = &status
					run.GrabbedCount++
					_ = s.acquire.MarkDownloadReplacement(ctx, download.ID, status.ID)
				}
			}
		}

		if request.RemoveFailed {
			action, err := s.acquire.DownloadAction(ctx, acquisition.DownloadActionRequest{
				Action:      acquisition.DownloadActionDelete,
				IDs:         []string{download.ID},
				DeleteFiles: request.DeleteFailedFiles,
			})
			if err != nil {
				if result.Error == "" {
					result.Error = err.Error()
				}
				run.ErrorCount++
			} else if action.Applied {
				result.Removed = true
				run.RemovedCount++
			}
		}

		run.Items = append(run.Items, result)
	}

	_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
		EventType:  "failed_downloads_checked",
		EntityType: "failed_download_run",
		EntityID:   run.ID,
		Severity:   "info",
		Message:    "Checked failed downloads",
		Data: map[string]any{
			"downloadsChecked":  run.DownloadsChecked,
			"failedCount":       run.FailedCount,
			"replacementsFound": run.ReplacementsFound,
			"grabbedCount":      run.GrabbedCount,
			"removedCount":      run.RemovedCount,
			"errorCount":        run.ErrorCount,
		},
	})
	if run.ErrorCount > 0 {
		run.Status = "partial_failed"
	} else {
		run.Status = "completed"
	}
	run.Message = failedDownloadMessage(run)
	return s.store.FinishFailedDownloadRun(ctx, run)
}

func (s *Service) SearchUpgrades(ctx context.Context, request UpgradeRequest) (UpgradeRun, error) {
	if !s.Available() {
		return UpgradeRun{}, errors.New("wanted service requires database persistence")
	}
	if s.acquire == nil {
		return UpgradeRun{}, errors.New("acquisition service is unavailable")
	}

	run, err := s.store.StartUpgradeRun(ctx, request.Trigger)
	if err != nil {
		return UpgradeRun{}, err
	}
	limit := request.Limit
	if limit <= 0 || limit > 200 {
		limit = defaultWantedMonitorLimit
	}
	searchLimit := request.SearchLimit
	if searchLimit <= 0 || searchLimit > 50 {
		searchLimit = defaultWantedMonitorSearchLimit
	}
	minSearchInterval := defaultUpgradeSearchInterval
	if request.MinSearchIntervalMinutes > 0 {
		minSearchInterval = time.Duration(request.MinSearchIntervalMinutes) * time.Minute
	}
	minDelta := request.MinScoreDelta
	if minDelta <= 0 {
		minDelta = defaultUpgradeScoreDelta
	}

	items, err := s.store.ListUpgradeWanted(ctx, request.WantedIDs, limit, minSearchInterval, request.Force)
	if err != nil {
		run.Status = "failed"
		run.ErrorCount = 1
		run.Message = err.Error()
		finished, finishErr := s.store.FinishUpgradeRun(ctx, run)
		if finishErr != nil {
			return UpgradeRun{}, finishErr
		}
		return finished, err
	}

	for _, item := range items {
		select {
		case <-ctx.Done():
			run.Status = "canceled"
			run.Message = ctx.Err().Error()
			return s.store.FinishUpgradeRun(context.Background(), run)
		default:
		}

		currentScore := s.currentReleaseScore(ctx, item)
		cutoff := upgradeCutoffFor(item)
		result := UpgradeItemResult{
			WantedItem:   item,
			CurrentScore: currentScore,
			CutoffScore:  cutoff,
		}
		outcome, err := s.searchReleasesForItem(ctx, item, SearchReleasesRequest{Limit: searchLimit})
		run.WantedChecked++
		_ = s.store.MarkWantedUpgradeSearched(ctx, item.ID)
		if err != nil {
			result.Error = err.Error()
			run.ErrorCount++
			run.Items = append(run.Items, result)
			continue
		}
		result.WantedItem = outcome.WantedItem
		result.ReleasesFound = len(outcome.Releases)
		run.ReleasesFound += len(outcome.Releases)

		if release, ok := bestUpgradeRelease(outcome.Releases, currentScore, cutoff, minDelta); ok {
			candidate := release
			result.UpgradeRelease = &candidate
			run.UpgradeCount++
			_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
				EventType:  "upgrade_available",
				EntityType: "wanted_item",
				EntityID:   item.ID,
				Severity:   "info",
				Message:    "Upgrade available for " + item.Title,
				Data: map[string]any{
					"title":        item.Title,
					"currentScore": currentScore,
					"cutoffScore":  cutoff,
					"releaseId":    release.ID,
					"releaseTitle": release.Title,
					"releaseScore": release.Score,
					"runId":        run.ID,
				},
			})
			if request.AutoGrab {
				status, err := s.grabRelease(ctx, outcome.WantedItem, release, request.Paused, "upgrade")
				if err != nil {
					result.Error = err.Error()
					run.ErrorCount++
					_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
						EventType:  "upgrade_grab_failed",
						EntityType: "wanted_item",
						EntityID:   item.ID,
						Severity:   "error",
						Message:    "Upgrade grab failed for " + item.Title,
						Data: map[string]any{
							"error":     err.Error(),
							"releaseId": release.ID,
							"title":     release.Title,
						},
					})
				} else {
					result.GrabbedDownload = &status
					run.GrabbedCount++
				}
			}
		}
		run.Items = append(run.Items, result)
	}

	_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
		EventType:  "upgrades_checked",
		EntityType: "upgrade_run",
		EntityID:   run.ID,
		Severity:   "info",
		Message:    "Checked wanted items for upgrades",
		Data: map[string]any{
			"wantedChecked": run.WantedChecked,
			"releasesFound": run.ReleasesFound,
			"upgradeCount":  run.UpgradeCount,
			"grabbedCount":  run.GrabbedCount,
			"errorCount":    run.ErrorCount,
		},
	})
	if run.ErrorCount > 0 {
		run.Status = "partial_failed"
	} else {
		run.Status = "completed"
	}
	run.Message = upgradeMessage(run)
	return s.store.FinishUpgradeRun(ctx, run)
}

func (s *Service) History(ctx context.Context, query HistoryQuery) ([]HistoryEvent, error) {
	if !s.Available() {
		return nil, errors.New("wanted service requires database persistence")
	}
	return s.store.ListHistory(ctx, query)
}

func (s *Service) grabRelease(ctx context.Context, item WantedItem, release ReleaseDecision, paused bool, trigger string) (acquisition.DownloadStatus, error) {
	status, err := s.acquire.Grab(ctx, acquisition.DownloadRequest{
		ReleaseURL: release.DownloadURL,
		InfoHash:   release.InfoHash,
		Title:      release.Title,
		Category:   s.acquire.CategoryForFormat(item.Format),
		SavePath:   s.acquire.TorrentRoot(),
		Paused:     paused,
		Tags:       []string{"librarry", "wanted:" + item.ID},
	})
	if err != nil {
		return acquisition.DownloadStatus{}, err
	}
	if err := s.store.MarkWantedStatus(ctx, item.ID, "grabbed"); err != nil {
		return status, err
	}
	if err := s.store.MarkWantedCurrentRelease(ctx, item.ID, release); err != nil {
		return status, err
	}
	_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
		EventType:  "release_grabbed",
		EntityType: "wanted_item",
		EntityID:   item.ID,
		Severity:   "info",
		Message:    "Grabbed approved release for " + item.Title,
		Data: map[string]any{
			"trigger":    trigger,
			"releaseId":  release.ID,
			"sourceId":   release.SourceID,
			"downloadId": status.ID,
			"title":      release.Title,
			"paused":     paused,
		},
	})
	return status, nil
}

func (s *Service) pickRelease(ctx context.Context, wantedID string, releaseID string) (ReleaseDecision, error) {
	if strings.TrimSpace(releaseID) != "" {
		return s.store.GetReleaseDecision(ctx, wantedID, releaseID)
	}
	releases, err := s.store.ListReleaseDecisions(ctx, wantedID)
	if err != nil {
		return ReleaseDecision{}, err
	}
	for _, release := range releases {
		if release.Approved {
			return release, nil
		}
	}
	if len(releases) == 0 {
		return ReleaseDecision{}, sql.ErrNoRows
	}
	return ReleaseDecision{}, errors.New("no approved release is available")
}

func firstApproved(releases []ReleaseDecision) (ReleaseDecision, bool) {
	for _, release := range releases {
		if release.Approved {
			return release, true
		}
	}
	return ReleaseDecision{}, false
}

func firstApprovedReplacement(releases []ReleaseDecision, failed acquisition.DownloadStatus) (ReleaseDecision, bool) {
	failedID := strings.ToLower(strings.TrimSpace(failed.ID))
	for _, release := range releases {
		if !release.Approved {
			continue
		}
		if failedID != "" {
			if strings.EqualFold(release.InfoHash, failedID) || strings.EqualFold(release.SourceID, failedID) || strings.EqualFold(release.ID, failedID) {
				continue
			}
		}
		return release, true
	}
	return ReleaseDecision{}, false
}

func (s *Service) currentReleaseScore(ctx context.Context, item WantedItem) float64 {
	if item.CurrentReleaseScore > 0 {
		return item.CurrentReleaseScore
	}
	releases, err := s.store.ListReleaseDecisions(ctx, item.ID)
	if err != nil {
		return 0
	}
	for _, release := range releases {
		if item.CurrentReleaseID != "" && release.ID == item.CurrentReleaseID {
			return release.Score
		}
	}
	for _, release := range releases {
		if release.Approved {
			return release.Score
		}
	}
	return 0
}

func bestUpgradeRelease(releases []ReleaseDecision, currentScore float64, cutoffScore float64, minDelta float64) (ReleaseDecision, bool) {
	if currentScore >= cutoffScore {
		return ReleaseDecision{}, false
	}
	if minDelta <= 0 {
		minDelta = defaultUpgradeScoreDelta
	}
	var best ReleaseDecision
	for _, release := range releases {
		if !release.Approved {
			continue
		}
		if release.Score < currentScore+minDelta {
			continue
		}
		if best.ID == "" || release.Score > best.Score || (release.Score == best.Score && release.Seeders > best.Seeders) {
			best = release
		}
	}
	return best, best.ID != ""
}

func upgradeCutoffFor(item WantedItem) float64 {
	switch normalizeQualityProfile(item.QualityProfile) {
	case "large", "best", "preferred":
		return 90
	case "strict":
		return 95
	default:
		return 85
	}
}

func monitorMessage(run MonitorRun) string {
	if run.WantedChecked == 0 {
		return strings.TrimSpace(run.Status) + ": no due wanted items"
	}
	return fmt.Sprintf(
		"%s: checked %d wanted items, found %d releases, approved %d, grabbed %d, errors %d",
		strings.TrimSpace(run.Status),
		run.WantedChecked,
		run.ReleasesFound,
		run.ApprovedCount,
		run.GrabbedCount,
		run.ErrorCount,
	)
}

func upgradeMessage(run UpgradeRun) string {
	if run.WantedChecked == 0 {
		return strings.TrimSpace(run.Status) + ": no grabbed or imported wanted items due for upgrade search"
	}
	return fmt.Sprintf(
		"%s: checked %d wanted items, found %d releases, upgrades %d, grabbed %d, errors %d",
		strings.TrimSpace(run.Status),
		run.WantedChecked,
		run.ReleasesFound,
		run.UpgradeCount,
		run.GrabbedCount,
		run.ErrorCount,
	)
}

func feedReleaseMatchesWanted(item WantedItem, release acquisition.Release) bool {
	releaseTitle := normalizeText(release.Title)
	wantedTitle := normalizeText(item.Title)
	if wantedTitle == "" || releaseTitle == "" {
		return false
	}
	if strings.Contains(releaseTitle, wantedTitle) {
		return true
	}
	overlap := tokenOverlap(wantedTitle, releaseTitle)
	if overlap >= 0.67 {
		return true
	}
	author := normalizeText(item.AuthorName)
	return author != "" && strings.Contains(releaseTitle, author) && overlap >= 0.5
}

func formatMatchesRequest(requested string, actual string) bool {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested == "" || requested == "any" {
		return true
	}
	return requested == normalizeFormat(actual)
}

func feedSyncMessage(run FeedSyncRun) string {
	return fmt.Sprintf(
		"%s: saw %d releases, matched %d, approved %d, grabbed %d, errors %d",
		strings.TrimSpace(run.Status),
		run.ReleasesSeen,
		run.MatchedCount,
		run.ApprovedCount,
		run.GrabbedCount,
		run.ErrorCount,
	)
}

func failedDownloadReason(download acquisition.DownloadStatus, stalledAge time.Duration, now time.Time) string {
	state := strings.ToLower(strings.TrimSpace(download.State))
	switch {
	case strings.Contains(state, "missing"):
		return "qBittorrent reports missing files"
	case strings.Contains(state, "error"):
		return "qBittorrent reports an error state"
	}
	if download.Progress >= 1 {
		return ""
	}
	if !strings.Contains(state, "stalled") || !strings.Contains(state, "dl") {
		return ""
	}
	if download.Seeders > 0 || download.DownloadRate > 0 {
		return ""
	}
	reference := download.LastActivityAt
	if reference == nil {
		reference = download.AddedAt
	}
	if reference == nil {
		return ""
	}
	if stalledAge <= 0 {
		stalledAge = defaultFailedDownloadStalledAge
	}
	stalledFor := now.Sub(reference.UTC())
	if stalledFor < stalledAge {
		return ""
	}
	return fmt.Sprintf("stalled with no seeders for %s", roundDuration(stalledFor))
}

func failedDownloadMessage(run FailedDownloadRun) string {
	if run.FailedCount == 0 {
		return strings.TrimSpace(run.Status) + ": no failed downloads"
	}
	return fmt.Sprintf(
		"%s: checked %d downloads, failed %d, replacements %d, grabbed %d, removed %d, errors %d",
		strings.TrimSpace(run.Status),
		run.DownloadsChecked,
		run.FailedCount,
		run.ReplacementsFound,
		run.GrabbedCount,
		run.RemovedCount,
		run.ErrorCount,
	)
}

func wantedIDFromTags(tags []string) string {
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, "wanted:") {
			return strings.TrimSpace(strings.TrimPrefix(tag, "wanted:"))
		}
	}
	return ""
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func roundDuration(value time.Duration) string {
	if value >= time.Hour {
		return value.Truncate(time.Hour).String()
	}
	if value >= time.Minute {
		return value.Truncate(time.Minute).String()
	}
	return value.Truncate(time.Second).String()
}
