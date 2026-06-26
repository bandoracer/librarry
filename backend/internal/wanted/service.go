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
	"github.com/bandoracer/librarry/backend/internal/metadata"
)

const (
	defaultWantedMonitorLimit       = 50
	defaultWantedMonitorSearchLimit = 20
	defaultAuthorSkippedItemsLimit  = 12
	defaultWantedSearchInterval     = 6 * time.Hour
	defaultAuthorSyncInterval       = 24 * time.Hour
	defaultFailedDownloadLimit      = 50
	defaultFailedDownloadStalledAge = 24 * time.Hour
	defaultUpgradeSearchInterval    = 12 * time.Hour
	defaultUpgradeScoreDelta        = 5.0
)

type Service struct {
	store        *Store
	acquire      Acquisition
	metadata     MetadataSearch
	restrictions ReleaseRestrictionProvider
}

type MetadataSearch interface {
	Search(ctx context.Context, query metadata.Query) ([]metadata.SearchResult, error)
}

type ReleaseRestrictionProvider interface {
	ListReleaseRestrictions(ctx context.Context) ([]ReleaseRestriction, error)
}

func NewService(store *Store, acquire Acquisition, metadataSearch ...MetadataSearch) *Service {
	var search MetadataSearch
	if len(metadataSearch) > 0 {
		search = metadataSearch[0]
	}
	return &Service{store: store, acquire: acquire, metadata: search}
}

func (s *Service) WithReleaseRestrictionProvider(provider ReleaseRestrictionProvider) *Service {
	if s != nil {
		s.restrictions = provider
	}
	return s
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
		return []WantedItem{}, nil
	}
	return s.store.ListWanted(ctx, status)
}

func (s *Service) UpdateWanted(ctx context.Context, id string, request WantedUpdateRequest) (WantedItem, error) {
	if !s.Available() {
		return WantedItem{}, errors.New("wanted service requires database persistence")
	}
	return s.store.UpdateWanted(ctx, id, request)
}

func (s *Service) DeleteWanted(ctx context.Context, id string) error {
	if !s.Available() {
		return errors.New("wanted service requires database persistence")
	}
	return s.store.DeleteWanted(ctx, id)
}

func (s *Service) ClearWantedManualOverrides(ctx context.Context, id string, fields []string) (WantedItem, error) {
	if !s.Available() {
		return WantedItem{}, errors.New("wanted service requires database persistence")
	}
	return s.store.ClearWantedManualOverrides(ctx, id, fields)
}

func (s *Service) MetadataProvenance(ctx context.Context, id string) (MetadataProvenance, error) {
	if !s.Available() {
		return MetadataProvenance{}, errors.New("wanted service requires database persistence")
	}
	return s.store.WantedMetadataProvenance(ctx, id)
}

func (s *Service) MetadataReviewQueue(ctx context.Context) (MetadataReviewQueue, error) {
	if !s.Available() {
		return MetadataReviewQueue{Items: []MetadataReviewItem{}, GeneratedAt: time.Now().UTC()}, nil
	}
	return s.store.WantedMetadataReviewQueue(ctx)
}

func (s *Service) ApplyMetadataCorrection(ctx context.Context, id string, request MetadataCorrectionRequest) (MetadataProvenance, error) {
	if !s.Available() {
		return MetadataProvenance{}, errors.New("wanted service requires database persistence")
	}
	return s.store.ApplyWantedMetadataCorrection(ctx, id, request)
}

func (s *Service) ApplyMetadataCorrections(ctx context.Context, id string, request MetadataCorrectionBatchRequest) (MetadataProvenance, error) {
	if !s.Available() {
		return MetadataProvenance{}, errors.New("wanted service requires database persistence")
	}
	return s.store.ApplyWantedMetadataCorrections(ctx, id, request)
}

func (s *Service) ListQualityProfiles(ctx context.Context) ([]QualityProfile, error) {
	if !s.Available() {
		return DefaultQualityProfiles(), nil
	}
	return s.store.ListQualityProfiles(ctx)
}

func (s *Service) SaveQualityProfile(ctx context.Context, profile QualityProfile) (QualityProfile, error) {
	if !s.Available() {
		return QualityProfile{}, errors.New("wanted service requires database persistence")
	}
	profile = normalizeProfileForStorage(profile)
	if strings.TrimSpace(profile.Name) == "" {
		return QualityProfile{}, errors.New("quality profile name is required")
	}
	return s.store.UpsertQualityProfile(ctx, profile)
}

func (s *Service) DeleteQualityProfile(ctx context.Context, idOrName string) error {
	if !s.Available() {
		return errors.New("wanted service requires database persistence")
	}
	if strings.TrimSpace(idOrName) == "" {
		return errors.New("quality profile id is required")
	}
	return s.store.DeleteQualityProfile(ctx, idOrName)
}

func (s *Service) SubscribeAuthor(ctx context.Context, request AuthorSubscribeRequest) (AuthorSubscription, error) {
	if !s.Available() {
		return AuthorSubscription{}, errors.New("wanted service requires database persistence")
	}
	subscription := authorSubscriptionFromRequest(request)
	if strings.TrimSpace(subscription.AuthorName) == "" {
		return AuthorSubscription{}, errors.New("author name is required")
	}
	return s.store.UpsertAuthorSubscription(ctx, subscription)
}

func (s *Service) ListAuthorSubscriptions(ctx context.Context, status string) ([]AuthorSubscription, error) {
	if !s.Available() {
		return []AuthorSubscription{}, nil
	}
	return s.store.ListAuthorSubscriptions(ctx, status)
}

func (s *Service) UpdateAuthorSubscription(ctx context.Context, id string, request AuthorUpdateRequest) (AuthorSubscription, error) {
	if !s.Available() {
		return AuthorSubscription{}, errors.New("wanted service requires database persistence")
	}
	return s.store.UpdateAuthorSubscription(ctx, id, request)
}

func (s *Service) DeleteAuthorSubscription(ctx context.Context, id string) error {
	if !s.Available() {
		return errors.New("wanted service requires database persistence")
	}
	return s.store.DeleteAuthorSubscription(ctx, id)
}

func (s *Service) MonitorAuthors(ctx context.Context, request AuthorMonitorRequest) (AuthorMonitorRun, error) {
	if !s.Available() {
		return AuthorMonitorRun{}, errors.New("wanted service requires database persistence")
	}
	if s.metadata == nil {
		return AuthorMonitorRun{}, errors.New("metadata service is unavailable")
	}
	run, err := s.store.StartAuthorMonitorRun(ctx, request.Trigger)
	if err != nil {
		return AuthorMonitorRun{}, err
	}
	limit := request.Limit
	if limit <= 0 || limit > 200 {
		limit = defaultWantedMonitorLimit
	}
	searchLimit := request.SearchLimit
	if searchLimit <= 0 || searchLimit > 50 {
		searchLimit = defaultWantedMonitorSearchLimit
	}
	minSyncInterval := defaultAuthorSyncInterval
	if request.MinSyncIntervalMinutes > 0 {
		minSyncInterval = time.Duration(request.MinSyncIntervalMinutes) * time.Minute
	}
	subscriptions, err := s.store.ListDueAuthorSubscriptions(ctx, limit, minSyncInterval, request.Force, request.AuthorIDs, request.ProviderKeys)
	if err != nil {
		run.Status = "failed"
		run.ErrorCount = 1
		run.Message = err.Error()
		finished, finishErr := s.store.FinishAuthorMonitorRun(ctx, run)
		if finishErr != nil {
			return AuthorMonitorRun{}, finishErr
		}
		return finished, err
	}

	for _, subscription := range subscriptions {
		select {
		case <-ctx.Done():
			run.Status = "canceled"
			run.Message = ctx.Err().Error()
			return s.store.FinishAuthorMonitorRun(context.Background(), run)
		default:
		}

		result := AuthorMonitorItemResult{Subscription: subscription}
		results, err := s.metadata.Search(ctx, metadata.Query{
			Query:       subscription.AuthorName,
			Type:        metadata.SearchTypeAuthorWorks,
			Format:      metadata.MediaFormat(subscription.Format),
			Limit:       searchLimit,
			ProviderKey: subscription.ProviderKey,
		})
		run.AuthorsChecked++
		if err != nil {
			result.Error = err.Error()
			run.ErrorCount++
			run.Items = append(run.Items, result)
			continue
		}
		for _, candidate := range results {
			if !authorResultMatchesSubscription(subscription, candidate) {
				continue
			}
			result.ResultsFound++
			if allowed, reason := authorResultAllowedByMissingPolicy(subscription, candidate, time.Now().UTC()); !allowed {
				result.SkippedCount++
				reviewID := ""
				includeSkippedItem := true
				review, reviewErr := s.store.UpsertAuthorMetadataReview(ctx, authorMetadataReviewFromSkipped(subscription, candidate, reason))
				if reviewErr != nil {
					result.Error = reviewErr.Error()
					run.ErrorCount++
				} else {
					reviewID = review.ID
					includeSkippedItem = review.Status == "pending"
				}
				if includeSkippedItem && len(result.SkippedItems) < defaultAuthorSkippedItemsLimit {
					result.SkippedItems = append(result.SkippedItems, AuthorSkippedItem{
						ReviewID: reviewID,
						Result:   candidate,
						Policy:   normalizeAuthorMissingBookPolicy(subscription.MissingBookPolicy, subscription.MonitorNewItems),
						Reason:   reason,
					})
				}
				continue
			}
			item, err := s.store.CreateWanted(ctx, CreateRequest{
				Result:         candidate,
				Format:         subscription.Format,
				QualityProfile: subscription.QualityProfile,
				Tags:           subscription.Tags,
			})
			if err != nil {
				result.Error = err.Error()
				run.ErrorCount++
				continue
			}
			result.WantedItems = append(result.WantedItems, item)
			result.WantedCreated++
		}
		run.ItemsFound += result.ResultsFound
		run.WantedCreated += result.WantedCreated
		if err := s.store.MarkAuthorSubscriptionSynced(ctx, subscription.ID); err != nil {
			result.Error = err.Error()
			run.ErrorCount++
		}
		_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
			EventType:  "author_subscription_synced",
			EntityType: "author_subscription",
			EntityID:   subscription.ID,
			Severity:   "info",
			Message:    "Synced author subscription for " + subscription.AuthorName,
			Data: map[string]any{
				"authorName":    subscription.AuthorName,
				"format":        subscription.Format,
				"missingPolicy": subscription.MissingBookPolicy,
				"resultsFound":  result.ResultsFound,
				"skippedCount":  result.SkippedCount,
				"wantedCreated": result.WantedCreated,
				"runId":         run.ID,
			},
		})
		run.Items = append(run.Items, result)
	}

	if run.ErrorCount > 0 {
		run.Status = "completed_with_errors"
	} else {
		run.Status = "completed"
	}
	run.Message = authorMonitorMessage(run)
	return s.store.FinishAuthorMonitorRun(ctx, run)
}

func (s *Service) ListAuthorMetadataReviews(ctx context.Context, query AuthorMetadataReviewQuery) ([]AuthorMetadataReview, error) {
	if !s.Available() {
		return []AuthorMetadataReview{}, nil
	}
	return s.store.ListAuthorMetadataReviews(ctx, query)
}

func (s *Service) ResolveAuthorMetadataReview(ctx context.Context, id string, request AuthorMetadataReviewDecisionRequest) (AuthorMetadataReviewDecision, error) {
	if !s.Available() {
		return AuthorMetadataReviewDecision{}, errors.New("wanted service requires database persistence")
	}
	review, err := s.store.GetAuthorMetadataReview(ctx, id)
	if err != nil {
		return AuthorMetadataReviewDecision{}, err
	}
	if strings.TrimSpace(review.Status) != "pending" {
		return AuthorMetadataReviewDecision{}, errors.New("author metadata review is already resolved")
	}
	switch strings.ToLower(strings.TrimSpace(request.Action)) {
	case "wanted", "mark_wanted", "mark-wanted":
		item, err := s.store.CreateWanted(ctx, CreateRequest{
			Result:         review.Result,
			Format:         review.Format,
			QualityProfile: review.QualityProfile,
			Tags:           review.Tags,
		})
		if err != nil {
			return AuthorMetadataReviewDecision{}, err
		}
		resolved, err := s.store.ResolveAuthorMetadataReview(ctx, review.ID, "wanted", "wanted", item.ID)
		if err != nil {
			return AuthorMetadataReviewDecision{}, err
		}
		_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
			EventType:  "author_metadata_review_wanted",
			EntityType: "wanted_item",
			EntityID:   item.ID,
			Severity:   "info",
			Message:    "Marked author metadata candidate wanted for " + item.Title,
			Data: map[string]any{
				"reviewId": resolved.ID,
				"policy":   resolved.Policy,
				"reason":   resolved.Reason,
			},
		})
		return AuthorMetadataReviewDecision{Review: resolved, WantedItem: &item}, nil
	case "ignore", "ignored", "skip":
		resolved, err := s.store.ResolveAuthorMetadataReview(ctx, review.ID, "ignored", "ignored", "")
		if err != nil {
			return AuthorMetadataReviewDecision{}, err
		}
		_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
			EventType:  "author_metadata_review_ignored",
			EntityType: "author_metadata_review",
			EntityID:   resolved.ID,
			Severity:   "info",
			Message:    "Ignored author metadata candidate for " + resolved.Title,
			Data: map[string]any{
				"policy": resolved.Policy,
				"reason": resolved.Reason,
			},
		})
		return AuthorMetadataReviewDecision{Review: resolved}, nil
	default:
		return AuthorMetadataReviewDecision{}, errors.New("author metadata review action must be wanted or ignore")
	}
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
	profile := s.qualityProfileForItem(ctx, item)
	restrictions, err := s.releaseRestrictions(ctx)
	if err != nil {
		return SearchOutcome{}, err
	}
	decisions := make([]ReleaseDecision, 0, len(releases))
	for _, release := range releases {
		decisions = append(decisions, evaluateReleaseWithPolicy(item, release, profile, restrictions))
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

func (s *Service) AcquisitionQueue(ctx context.Context, query AcquisitionQueueQuery) (AcquisitionQueue, error) {
	if !s.Available() {
		return AcquisitionQueue{}, errors.New("wanted service requires database persistence")
	}
	status := strings.TrimSpace(query.Status)
	if strings.EqualFold(status, "all") {
		status = ""
	}
	items, err := s.store.ListWanted(ctx, status)
	if err != nil {
		return AcquisitionQueue{}, err
	}
	limit := query.Limit
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	if len(items) > limit {
		items = items[:limit]
	}

	downloadsByWanted := map[string][]acquisition.DownloadStatus{}
	if s.acquire != nil {
		downloads, err := s.acquire.Downloads(ctx, acquisition.DownloadListQuery{Tag: "librarry"})
		if err == nil {
			downloadsByWanted = groupDownloadsByWantedID(downloads)
		}
	}

	queue := AcquisitionQueue{
		Items:       make([]AcquisitionQueueItem, 0, len(items)),
		GeneratedAt: time.Now().UTC(),
	}
	for _, item := range items {
		if item.Status == "removed" || item.Status == "ignored" {
			continue
		}
		releases, err := s.store.ListReleaseDecisions(ctx, item.ID)
		if err != nil {
			return AcquisitionQueue{}, err
		}
		row := acquisitionQueueItem(item, releases, downloadsByWanted[item.ID])
		queue.Items = append(queue.Items, row)
		queue.Summary.Total++
		switch row.State {
		case "needs_search":
			queue.Summary.NeedsSearch++
		case "ready_to_grab":
			queue.Summary.ReadyToGrab++
		case "queued", "downloading":
			queue.Summary.Queued++
		case "import_ready":
			queue.Summary.ImportReady++
		case "imported":
			queue.Summary.Imported++
		default:
			if row.State == "blocked" {
				queue.Summary.Blocked++
			}
		}
	}
	return queue, nil
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
	if !release.Approved && !request.Force {
		return acquisition.DownloadStatus{}, errors.New("release is rejected: " + release.RejectedReason)
	}
	return s.grabRelease(ctx, item, release, request.Paused, request.Client, "manual", request.Force)
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
				status, err := s.grabRelease(ctx, outcome.WantedItem, release, request.Paused, "", "monitor", false)
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
		profile := s.qualityProfileForItem(ctx, item)
		restrictions, err := s.releaseRestrictions(ctx)
		if err != nil {
			run.ErrorCount++
			run.Matches = append(run.Matches, FeedSyncMatch{WantedItem: item, Error: err.Error()})
			continue
		}
		var decisions []ReleaseDecision
		for _, release := range releases {
			if !feedReleaseMatchesWanted(item, release) {
				continue
			}
			decisions = append(decisions, evaluateReleaseWithPolicy(item, release, profile, restrictions))
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
				status, err := s.grabRelease(ctx, item, decision, request.Paused, "", "feed", false)
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
				status, err := s.grabRelease(ctx, outcome.WantedItem, replacement, request.Paused, "", "failed-download", false)
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

		profile := s.qualityProfileForItem(ctx, item)
		currentScore := s.currentReleaseScore(ctx, item)
		cutoff := profile.CutoffScore
		result := UpgradeItemResult{
			WantedItem:   item,
			CurrentScore: currentScore,
			CutoffScore:  cutoff,
		}
		if !profile.UpgradeAllowed {
			run.WantedChecked++
			_ = s.store.MarkWantedUpgradeSearched(ctx, item.ID)
			run.Items = append(run.Items, result)
			continue
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
				status, err := s.grabRelease(ctx, outcome.WantedItem, release, request.Paused, "", "upgrade", false)
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
		return []HistoryEvent{}, nil
	}
	return s.store.ListHistory(ctx, query)
}

func (s *Service) grabRelease(ctx context.Context, item WantedItem, release ReleaseDecision, paused bool, client string, trigger string, forced bool) (acquisition.DownloadStatus, error) {
	status, err := s.acquire.Grab(ctx, acquisition.DownloadRequest{
		Client:     client,
		ReleaseURL: release.DownloadURL,
		InfoHash:   release.InfoHash,
		Title:      release.Title,
		Protocol:   release.Protocol,
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
		Message:    grabHistoryMessage(item, forced),
		Data: map[string]any{
			"trigger":        trigger,
			"releaseId":      release.ID,
			"sourceId":       release.SourceID,
			"downloadId":     status.ID,
			"title":          release.Title,
			"paused":         paused,
			"forced":         forced,
			"rejectedReason": release.RejectedReason,
		},
	})
	return status, nil
}

func grabHistoryMessage(item WantedItem, forced bool) string {
	if forced {
		return "Force grabbed manually selected release for " + item.Title
	}
	return "Grabbed approved release for " + item.Title
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

func acquisitionQueueItem(item WantedItem, releases []ReleaseDecision, downloads []acquisition.DownloadStatus) AcquisitionQueueItem {
	row := AcquisitionQueueItem{
		WantedItem:     item,
		Downloads:      downloads,
		LastActivityAt: timePtr(item.UpdatedAt),
	}
	for i, release := range releases {
		if i == 0 {
			row.BestRelease = releasePtr(release)
		}
		row.ReleaseCount++
		if release.Approved {
			row.ApprovedCount++
			if row.BestRelease == nil {
				row.BestRelease = releasePtr(release)
			}
		} else {
			row.RejectedCount++
		}
		if item.CurrentReleaseID != "" && release.ID == item.CurrentReleaseID {
			row.CurrentRelease = releasePtr(release)
		}
		row.LastActivityAt = laterTimePtr(row.LastActivityAt, release.SearchedAt)
		row.LastActivityAt = laterTimePtr(row.LastActivityAt, release.CreatedAt)
	}
	for _, download := range downloads {
		row.LastActivityAt = laterDownloadActivity(row.LastActivityAt, download)
	}

	row.State, row.NextAction = acquisitionQueueState(item, row)
	return row
}

func acquisitionQueueState(item WantedItem, row AcquisitionQueueItem) (string, string) {
	switch {
	case item.Status == "imported" || downloadsHaveImportStatus(row.Downloads, "imported"):
		return "imported", "Complete"
	case downloadsHaveFailure(row.Downloads):
		return "blocked", "Recover failed download"
	case downloadsHaveImportReady(row.Downloads):
		return "import_ready", "Import completed download"
	case downloadsHaveActiveState(row.Downloads):
		return "downloading", "Wait for external client"
	case len(row.Downloads) > 0:
		return "queued", "Monitor external client"
	case row.ApprovedCount > 0:
		return "ready_to_grab", "Grab approved release"
	case row.ReleaseCount > 0:
		return "blocked", "Review rejected releases"
	case item.LastSearchAt == nil:
		return "needs_search", "Search releases"
	case item.Status == "grabbed":
		return "blocked", "Refresh queue or regrab"
	default:
		return "needs_search", "Search releases"
	}
}

func groupDownloadsByWantedID(downloads []acquisition.DownloadStatus) map[string][]acquisition.DownloadStatus {
	grouped := map[string][]acquisition.DownloadStatus{}
	for _, download := range downloads {
		for _, tag := range download.Tags {
			tag = strings.TrimSpace(tag)
			if !strings.HasPrefix(tag, "wanted:") {
				continue
			}
			id := strings.TrimSpace(strings.TrimPrefix(tag, "wanted:"))
			if id == "" {
				continue
			}
			grouped[id] = append(grouped[id], download)
		}
	}
	return grouped
}

func releasePtr(release ReleaseDecision) *ReleaseDecision {
	return &release
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func laterTimePtr(current *time.Time, candidate time.Time) *time.Time {
	if candidate.IsZero() {
		return current
	}
	candidate = candidate.UTC()
	if current == nil || candidate.After(*current) {
		return &candidate
	}
	return current
}

func laterDownloadActivity(current *time.Time, download acquisition.DownloadStatus) *time.Time {
	candidates := []*time.Time{
		download.LastSeenAt,
		download.LastActivityAt,
		download.CompletedAt,
		download.AddedAt,
		download.ImportedAt,
		download.FailedAt,
	}
	for _, candidate := range candidates {
		if candidate != nil {
			current = laterTimePtr(current, *candidate)
		}
	}
	return current
}

func downloadsHaveImportStatus(downloads []acquisition.DownloadStatus, status string) bool {
	for _, download := range downloads {
		if strings.EqualFold(strings.TrimSpace(download.ImportStatus), status) {
			return true
		}
	}
	return false
}

func downloadsHaveFailure(downloads []acquisition.DownloadStatus) bool {
	for _, download := range downloads {
		state := strings.ToLower(strings.TrimSpace(download.State))
		if strings.TrimSpace(download.FailureReason) != "" ||
			strings.TrimSpace(download.ImportError) != "" ||
			strings.Contains(state, "error") ||
			strings.Contains(state, "missing") ||
			strings.Contains(state, "failed") {
			return true
		}
	}
	return false
}

func downloadsHaveImportReady(downloads []acquisition.DownloadStatus) bool {
	for _, download := range downloads {
		if strings.EqualFold(strings.TrimSpace(download.ImportStatus), "ready") ||
			download.CompletedAt != nil ||
			download.Progress >= 1 {
			return true
		}
	}
	return false
}

func downloadsHaveActiveState(downloads []acquisition.DownloadStatus) bool {
	for _, download := range downloads {
		state := strings.ToLower(strings.TrimSpace(download.State))
		if strings.Contains(state, "download") ||
			strings.Contains(state, "upload") ||
			strings.Contains(state, "queued") ||
			strings.Contains(state, "stalled") ||
			strings.Contains(state, "check") ||
			strings.Contains(state, "paused") ||
			strings.Contains(state, "stopped") {
			return true
		}
	}
	return false
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

func (s *Service) qualityProfileForItem(ctx context.Context, item WantedItem) QualityProfile {
	if s.store == nil {
		return defaultQualityProfile(item.QualityProfile, item.Format)
	}
	profile, err := s.store.GetQualityProfile(ctx, item.QualityProfile, item.Format)
	if err != nil {
		return defaultQualityProfile(item.QualityProfile, item.Format)
	}
	return normalizeProfile(profile, item)
}

func (s *Service) releaseRestrictions(ctx context.Context) ([]ReleaseRestriction, error) {
	if s.restrictions == nil {
		return nil, nil
	}
	restrictions, err := s.restrictions.ListReleaseRestrictions(ctx)
	if err != nil {
		return nil, err
	}
	return restrictions, nil
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
	return defaultQualityProfile(item.QualityProfile, item.Format).CutoffScore
}

func authorSubscriptionFromRequest(request AuthorSubscribeRequest) AuthorSubscription {
	authorName := strings.TrimSpace(request.AuthorName)
	provider := strings.TrimSpace(request.Provider)
	providerKey := strings.TrimSpace(request.ProviderKey)
	if len(request.Result.Work.Authors) > 0 {
		author := request.Result.Work.Authors[0]
		authorName = firstNonEmpty(authorName, author.Name)
		providerKey = firstNonEmpty(providerKey, author.ID)
	}
	provider = firstNonEmpty(provider, request.Result.Provider, "manual")
	if providerKey == "" && authorName != "" {
		providerKey = provider + ":author:" + normalizeText(authorName)
	}
	format := request.Format
	if format == "" {
		format = string(request.Result.Edition.Format)
	}
	format = normalizeFormat(format)
	qualityProfile := normalizeQualityProfile(request.QualityProfile)
	monitorNewItems := true
	if request.MonitorNewItems != nil {
		monitorNewItems = *request.MonitorNewItems
	}
	missingBookPolicy := normalizeAuthorMissingBookPolicy(request.MissingBookPolicy, monitorNewItems)
	monitorNewItems = missingBookPolicy != "none"
	return AuthorSubscription{
		Provider:          provider,
		ProviderKey:       providerKey,
		AuthorName:        authorName,
		Format:            format,
		QualityProfile:    qualityProfile,
		Status:            "monitored",
		MonitorNewItems:   monitorNewItems,
		MissingBookPolicy: missingBookPolicy,
		Tags:              compactRestrictionTags(request.Tags),
	}
}

func authorResultMatchesSubscription(subscription AuthorSubscription, result metadata.SearchResult) bool {
	if len(result.Work.Authors) == 0 {
		return false
	}
	subscriptionKey := strings.TrimSpace(subscription.ProviderKey)
	subscriptionName := normalizeText(subscription.AuthorName)
	for _, author := range result.Work.Authors {
		if subscriptionKey != "" && strings.EqualFold(subscriptionKey, strings.TrimSpace(author.ID)) {
			return true
		}
		authorName := normalizeText(author.Name)
		if authorName == "" || subscriptionName == "" {
			continue
		}
		if authorName == subscriptionName || strings.Contains(authorName, subscriptionName) || strings.Contains(subscriptionName, authorName) {
			return true
		}
	}
	return false
}

func authorMetadataReviewFromSkipped(subscription AuthorSubscription, result metadata.SearchResult, reason string) AuthorMetadataReview {
	return AuthorMetadataReview{
		AuthorSubscriptionID: subscription.ID,
		Provider:             result.Provider,
		CandidateKey:         authorMetadataReviewCandidateKey(result),
		Title:                firstNonEmpty(result.Work.Title, result.Edition.Title),
		AuthorName:           firstResultAuthorName(result),
		Format:               subscription.Format,
		QualityProfile:       subscription.QualityProfile,
		Tags:                 subscription.Tags,
		Policy:               normalizeAuthorMissingBookPolicy(subscription.MissingBookPolicy, subscription.MonitorNewItems),
		Reason:               reason,
		Status:               "pending",
		Result:               result,
	}
}

func authorResultAllowedByMissingPolicy(subscription AuthorSubscription, result metadata.SearchResult, now time.Time) (bool, string) {
	switch normalizeAuthorMissingBookPolicy(subscription.MissingBookPolicy, subscription.MonitorNewItems) {
	case "none":
		return false, "author policy is set to none"
	case "future":
		published, ok := resultPublicationDate(result)
		if !ok {
			return false, "future policy requires a publication date"
		}
		cutoff := subscription.CreatedAt
		if cutoff.IsZero() {
			cutoff = now
		}
		switch published.Precision {
		case "year":
			if published.Time.Year() >= cutoff.Year() {
				return true, ""
			}
		case "month":
			publishedMonth := time.Date(published.Time.Year(), published.Time.Month(), 1, 0, 0, 0, 0, time.UTC)
			cutoffMonth := time.Date(cutoff.UTC().Year(), cutoff.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
			if !publishedMonth.Before(cutoffMonth) {
				return true, ""
			}
		default:
			publishedDay := time.Date(published.Time.Year(), published.Time.Month(), published.Time.Day(), 0, 0, 0, 0, time.UTC)
			cutoffDay := time.Date(cutoff.UTC().Year(), cutoff.UTC().Month(), cutoff.UTC().Day(), 0, 0, 0, 0, time.UTC)
			if !publishedDay.Before(cutoffDay) {
				return true, ""
			}
		}
		return false, "published before the author subscription cutoff"
	default:
		return true, ""
	}
}

type publicationDate struct {
	Time      time.Time
	Precision string
}

func resultPublicationDate(result metadata.SearchResult) (publicationDate, bool) {
	if published, ok := parsePublicationDate(result.Edition.PublishedDate); ok {
		return published, true
	}
	if result.Work.FirstPublishYear > 0 {
		return publicationDate{
			Time:      time.Date(result.Work.FirstPublishYear, 1, 1, 0, 0, 0, 0, time.UTC),
			Precision: "year",
		}, true
	}
	return publicationDate{}, false
}

func parsePublicationDate(value string) (publicationDate, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return publicationDate{}, false
	}
	for _, candidate := range []struct {
		layout    string
		precision string
	}{
		{"2006-01-02", "day"},
		{"2006-01", "month"},
		{"2006", "year"},
	} {
		parsed, err := time.Parse(candidate.layout, value)
		if err == nil {
			return publicationDate{Time: parsed.UTC(), Precision: candidate.precision}, true
		}
	}
	return publicationDate{}, false
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

func authorMonitorMessage(run AuthorMonitorRun) string {
	if run.AuthorsChecked == 0 {
		return strings.TrimSpace(run.Status) + ": no due author subscriptions"
	}
	return fmt.Sprintf(
		"%s: checked %d author subscriptions, found %d items, created or refreshed %d wanted items, errors %d",
		strings.TrimSpace(run.Status),
		run.AuthorsChecked,
		run.ItemsFound,
		run.WantedCreated,
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
