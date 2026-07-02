package importlists

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

// WantedGateway is the slice of the wanted service the sync needs. Kept as an
// interface so mapping/dedupe/exclusion logic is testable without Postgres.
type WantedGateway interface {
	Create(ctx context.Context, request wanted.CreateRequest) (wanted.WantedItem, error)
	UpdateWanted(ctx context.Context, id string, request wanted.WantedUpdateRequest) (wanted.WantedItem, error)
	SearchReleases(ctx context.Context, wantedID string, request wanted.SearchReleasesRequest) (wanted.SearchOutcome, error)
	WantedSourceKeySet(ctx context.Context) (map[string]bool, error)
}

// Service syncs import lists into wanted items.
type Service struct {
	store    *Store
	wanted   WantedGateway
	fetchers map[string]ListFetcher
	logger   *slog.Logger
}

func NewService(store *Store, wantedGateway WantedGateway, hardcover ListFetcher, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	fetchers := map[string]ListFetcher{}
	if hardcover != nil {
		fetchers["hardcover"] = hardcover
	}
	return &Service{store: store, wanted: wantedGateway, fetchers: fetchers, logger: logger}
}

func (s *Service) Available() bool {
	return s != nil && s.store != nil && s.store.Configured() && s.wanted != nil
}

func (s *Service) Store() *Store {
	if s == nil {
		return nil
	}
	return s.store
}

// Lists proxies the store for API handlers.
func (s *Service) Lists(ctx context.Context) ([]List, error) {
	if !s.Available() {
		return nil, errors.New("import list service is unavailable")
	}
	return s.store.ListLists(ctx)
}

// Exclusions proxies the store for API handlers and the compat sync command.
func (s *Service) Exclusions(ctx context.Context) ([]Exclusion, error) {
	if !s.Available() {
		return nil, errors.New("import list service is unavailable")
	}
	return s.store.ListExclusions(ctx)
}

// Sync runs one pass over the given list ids (all enabled lists when empty).
// Explicitly-requested lists sync even when disabled.
func (s *Service) Sync(ctx context.Context, listIDs []string, trigger string) (SyncOutcome, error) {
	outcome := SyncOutcome{Status: "completed", Trigger: strings.TrimSpace(trigger), StartedAt: time.Now().UTC()}
	if !s.Available() {
		outcome.Status = "failed"
		outcome.Message = "import list service is unavailable"
		outcome.FinishedAt = time.Now().UTC()
		return outcome, errors.New(outcome.Message)
	}

	explicit := map[string]bool{}
	for _, id := range listIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			explicit[trimmed] = true
		}
	}

	lists, err := s.store.ListLists(ctx)
	if err != nil {
		outcome.Status = "failed"
		outcome.Message = err.Error()
		outcome.FinishedAt = time.Now().UTC()
		return outcome, err
	}
	exclusions, err := s.store.ListExclusions(ctx)
	if err != nil {
		outcome.Status = "failed"
		outcome.Message = err.Error()
		outcome.FinishedAt = time.Now().UTC()
		return outcome, err
	}
	existing, err := s.wanted.WantedSourceKeySet(ctx)
	if err != nil {
		outcome.Status = "failed"
		outcome.Message = err.Error()
		outcome.FinishedAt = time.Now().UTC()
		return outcome, err
	}
	if existing == nil {
		existing = map[string]bool{}
	}

	for _, list := range lists {
		if len(explicit) > 0 && !explicit[list.ID] {
			continue
		}
		if len(explicit) == 0 && !list.Enabled {
			continue
		}
		outcome.ListsChecked++
		s.syncList(ctx, list, exclusions, existing, &outcome)
	}

	if outcome.ErrorCount > 0 {
		outcome.Status = "completed_with_errors"
	}
	outcome.Message = syncMessage(outcome)
	outcome.FinishedAt = time.Now().UTC()
	return outcome, nil
}

func (s *Service) syncList(ctx context.Context, list List, exclusions []Exclusion, existing map[string]bool, outcome *SyncOutcome) {
	fetcher := s.fetchers[strings.ToLower(strings.TrimSpace(list.Type))]
	if fetcher == nil {
		outcome.ErrorCount++
		outcome.Items = append(outcome.Items, SyncItem{
			ListID: list.ID, ListName: list.Name, Status: "error",
			Error: "no fetcher for list type " + list.Type,
		})
		return
	}
	entries, err := fetcher.FetchList(ctx, list.Settings, 200)
	if err != nil {
		outcome.ErrorCount++
		outcome.Items = append(outcome.Items, SyncItem{
			ListID: list.ID, ListName: list.Name, Status: "error", Error: err.Error(),
		})
		return
	}
	format := listFormat(list)
	for _, entry := range entries {
		outcome.EntriesFound++
		item := SyncItem{ListID: list.ID, ListName: list.Name, Title: entry.Title, AuthorName: entry.AuthorName}
		if excluded, reason := EntryExcluded(entry, exclusions); excluded {
			outcome.SkippedExcluded++
			item.Status = "skipped"
			item.Reason = reason
			outcome.Items = append(outcome.Items, item)
			continue
		}
		identity := wanted.SourceIdentity("Hardcover", entry.SourceKey, format)
		if existing[identity] {
			outcome.SkippedExisting++
			item.Status = "skipped"
			item.Reason = "already tracked"
			outcome.Items = append(outcome.Items, item)
			continue
		}
		created, err := s.wanted.Create(ctx, wanted.CreateRequest{
			Result:         EntryToSearchResult(entry, format),
			Format:         format,
			QualityProfile: list.QualityProfile,
		})
		if err != nil {
			outcome.ErrorCount++
			item.Status = "error"
			item.Error = err.Error()
			outcome.Items = append(outcome.Items, item)
			continue
		}
		existing[identity] = true
		item.Status = "wanted"
		item.WantedID = created.ID

		// Monitor mode and root folder land through the update path so the
		// create upsert semantics stay untouched.
		update := wanted.WantedUpdateRequest{}
		needsUpdate := false
		if list.Monitor == "none" {
			monitored := false
			update.Monitored = &monitored
			needsUpdate = true
		}
		if list.RootFolderID != "" {
			rootID := list.RootFolderID
			update.RootFolderID = &rootID
			needsUpdate = true
		}
		if needsUpdate {
			if _, err := s.wanted.UpdateWanted(ctx, created.ID, update); err != nil {
				outcome.ErrorCount++
				item.Error = err.Error()
			}
		}
		if list.SearchOnAdd && list.Monitor != "none" {
			// Review-first rule intact: search records release decisions, it
			// never grabs.
			if _, err := s.wanted.SearchReleases(ctx, created.ID, wanted.SearchReleasesRequest{}); err != nil {
				s.logger.Warn("import list search-on-add failed", "list", list.Name, "wanted", created.ID, "error", err)
			} else {
				outcome.SearchesStarted++
			}
		}
		outcome.WantedCreated++
		outcome.Items = append(outcome.Items, item)
	}
	if err := s.store.MarkListSynced(ctx, list.ID); err != nil {
		s.logger.Warn("import list sync timestamp update failed", "list", list.Name, "error", err)
	}
}

// EntryToSearchResult maps a list entry onto the metadata result shape the
// wanted store persists. The identity matches the Hardcover search provider
// (Work.ID "hardcover:<id>") so search-added and list-added books dedupe.
func EntryToSearchResult(entry Entry, format string) metadata.SearchResult {
	authorName := strings.TrimSpace(entry.AuthorName)
	return metadata.SearchResult{
		Provider:     "Hardcover",
		Kind:         metadata.SearchTypeBook,
		Score:        1,
		Confidence:   "import-list",
		MatchedOn:    []string{"import-list"},
		RawSourceKey: entry.SourceKey,
		Work: metadata.Work{
			ID:       entry.SourceKey,
			Title:    entry.Title,
			CoverURL: entry.CoverURL,
			Authors: []metadata.Author{{
				ID:   "hardcover-author:" + slugValue(authorName),
				Name: authorName,
			}},
			ProviderIDs: []string{entry.SourceKey},
		},
		Edition: metadata.Edition{
			Title:         entry.Title,
			Format:        metadata.MediaFormat(format),
			PublishedDate: strings.TrimSpace(entry.ReleaseDate),
		},
	}
}

// EntryExcluded reports whether an exclusion suppresses the entry: source-key
// match first, then title (+ optional author) match.
func EntryExcluded(entry Entry, exclusions []Exclusion) (bool, string) {
	for _, exclusion := range exclusions {
		if exclusion.SourceKey != "" && strings.EqualFold(exclusion.SourceKey, entry.SourceKey) {
			return true, "excluded by source key"
		}
		if exclusion.Title == "" || !strings.EqualFold(strings.TrimSpace(exclusion.Title), strings.TrimSpace(entry.Title)) {
			continue
		}
		if exclusion.AuthorName == "" || strings.EqualFold(strings.TrimSpace(exclusion.AuthorName), strings.TrimSpace(entry.AuthorName)) {
			return true, "excluded by title"
		}
	}
	return false, ""
}

func listFormat(list List) string {
	format := strings.ToLower(strings.TrimSpace(list.Settings["format"]))
	if format != "audiobook" {
		format = "ebook"
	}
	return format
}

func syncMessage(outcome SyncOutcome) string {
	return fmt.Sprintf(
		"%s: %d lists checked, %d entries, %d wanted created, %d already tracked, %d excluded, %d errors",
		outcome.Status, outcome.ListsChecked, outcome.EntriesFound, outcome.WantedCreated,
		outcome.SkippedExisting, outcome.SkippedExcluded, outcome.ErrorCount,
	)
}
