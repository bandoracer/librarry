package wanted

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
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
	limit := request.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
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
	item, _ = s.store.GetWanted(ctx, wantedID)
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
	return s.acquire.Grab(ctx, acquisition.DownloadRequest{
		ReleaseURL: release.DownloadURL,
		InfoHash:   release.InfoHash,
		Title:      release.Title,
		Category:   s.acquire.CategoryForFormat(item.Format),
		SavePath:   s.acquire.TorrentRoot(),
		Paused:     request.Paused,
		Tags:       []string{"librarry", "wanted:" + item.ID},
	})
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
