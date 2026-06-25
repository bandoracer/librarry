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
