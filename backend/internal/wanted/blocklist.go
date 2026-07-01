package wanted

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

const (
	BlocklistSourceAutoFailed        = "auto-failed"
	BlocklistSourceQueueRemove       = "queue-remove"
	BlocklistSourceHistoryMarkFailed = "history-mark-failed"

	blocklistRejectionReason  = "blocklisted"
	defaultBlocklistListLimit = 500
	maxBlocklistMatchEntries  = 2000
)

// BlocklistEntry records a release identity that must never be grabbed again.
type BlocklistEntry struct {
	ID              string    `json:"id"`
	WantedItemID    string    `json:"wantedItemId,omitempty"`
	Title           string    `json:"title"`
	Indexer         string    `json:"indexer,omitempty"`
	Protocol        string    `json:"protocol,omitempty"`
	DownloadURLHash string    `json:"downloadUrlHash,omitempty"`
	InfoHash        string    `json:"infohash,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	Source          string    `json:"source"`
	CreatedAt       time.Time `json:"createdAt"`
}

type BlocklistDownloadRequest struct {
	DownloadID string `json:"downloadId"`
	Client     string `json:"client,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Source     string `json:"source,omitempty"`
}

type ManualFailRequest struct {
	ID        string `json:"id"`
	Client    string `json:"client,omitempty"`
	Blocklist bool   `json:"blocklist,omitempty"`
	Research  bool   `json:"research,omitempty"`
}

type ManualFailOutcome struct {
	Blocklisted     bool `json:"blocklisted"`
	SearchTriggered bool `json:"searchTriggered"`
}

// Store CRUD ----------------------------------------------------------------

func (s *Store) ListBlocklist(ctx context.Context, limit int) ([]BlocklistEntry, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	if limit <= 0 || limit > maxBlocklistMatchEntries {
		limit = defaultBlocklistListLimit
	}
	rows, err := s.db.QueryContext(ctx, `
		select
			id, coalesce(wanted_item_id::text, ''), title, indexer, protocol,
			download_url_hash, infohash, reason, source, coalesce(created_at, now())
		from blocklist
		order by created_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []BlocklistEntry
	for rows.Next() {
		var entry BlocklistEntry
		if err := rows.Scan(
			&entry.ID, &entry.WantedItemID, &entry.Title, &entry.Indexer, &entry.Protocol,
			&entry.DownloadURLHash, &entry.InfoHash, &entry.Reason, &entry.Source, &entry.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *Store) CreateBlocklistEntry(ctx context.Context, entry BlocklistEntry) (BlocklistEntry, error) {
	if !s.Configured() {
		return BlocklistEntry{}, errors.New("wanted store is unavailable")
	}
	entry = normalizeBlocklistEntry(entry)
	if entry.Title == "" && entry.InfoHash == "" && entry.DownloadURLHash == "" {
		return BlocklistEntry{}, errors.New("blocklist entry requires a title, infohash, or download URL hash")
	}
	row := s.db.QueryRowContext(ctx, `
		insert into blocklist (
			wanted_item_id, title, indexer, protocol, download_url_hash, infohash, reason, source
		) values (
			nullif($1, '')::uuid, $2, $3, $4, $5, $6, $7, $8
		)
		returning
			id, coalesce(wanted_item_id::text, ''), title, indexer, protocol,
			download_url_hash, infohash, reason, source, coalesce(created_at, now())
	`, entry.WantedItemID, entry.Title, entry.Indexer, entry.Protocol,
		entry.DownloadURLHash, entry.InfoHash, entry.Reason, entry.Source)
	var stored BlocklistEntry
	if err := row.Scan(
		&stored.ID, &stored.WantedItemID, &stored.Title, &stored.Indexer, &stored.Protocol,
		&stored.DownloadURLHash, &stored.InfoHash, &stored.Reason, &stored.Source, &stored.CreatedAt,
	); err != nil {
		return BlocklistEntry{}, err
	}
	return stored, nil
}

func (s *Store) DeleteBlocklistEntry(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("wanted store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("blocklist id is required")
	}
	result, err := s.db.ExecContext(ctx, `delete from blocklist where id::text = $1`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteBlocklistEntries removes the given entries; an empty id list clears
// the whole blocklist. It returns the number of removed rows.
func (s *Store) DeleteBlocklistEntries(ctx context.Context, ids []string) (int, error) {
	if !s.Configured() {
		return 0, errors.New("wanted store is unavailable")
	}
	cleanIDs := compactStrings(ids)
	var result sql.Result
	var err error
	if len(cleanIDs) == 0 {
		result, err = s.db.ExecContext(ctx, `delete from blocklist`)
	} else {
		placeholders := make([]string, 0, len(cleanIDs))
		args := make([]any, 0, len(cleanIDs))
		for i, id := range cleanIDs {
			placeholders = append(placeholders, "$"+strconv.Itoa(i+1))
			args = append(args, id)
		}
		result, err = s.db.ExecContext(ctx, `delete from blocklist where id::text in (`+strings.Join(placeholders, ",")+`)`, args...)
	}
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

// MatchBlocklist looks a release identity up by infohash, download URL hash,
// or the title+indexer fallback.
func (s *Store) MatchBlocklist(ctx context.Context, infohash string, downloadURLHash string, title string, indexer string) (BlocklistEntry, bool, error) {
	if !s.Configured() {
		return BlocklistEntry{}, false, errors.New("wanted store is unavailable")
	}
	infohash = strings.ToLower(strings.TrimSpace(infohash))
	downloadURLHash = strings.ToLower(strings.TrimSpace(downloadURLHash))
	title = strings.TrimSpace(title)
	indexer = strings.TrimSpace(indexer)
	row := s.db.QueryRowContext(ctx, `
		select
			id, coalesce(wanted_item_id::text, ''), title, indexer, protocol,
			download_url_hash, infohash, reason, source, coalesce(created_at, now())
		from blocklist
		where ($1 <> '' and infohash = $1)
			or ($2 <> '' and download_url_hash = $2)
			or ($3 <> '' and $4 <> '' and lower(title) = lower($3) and lower(indexer) = lower($4))
		limit 1
	`, infohash, downloadURLHash, title, indexer)
	var entry BlocklistEntry
	if err := row.Scan(
		&entry.ID, &entry.WantedItemID, &entry.Title, &entry.Indexer, &entry.Protocol,
		&entry.DownloadURLHash, &entry.InfoHash, &entry.Reason, &entry.Source, &entry.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BlocklistEntry{}, false, nil
		}
		return BlocklistEntry{}, false, err
	}
	return entry, true, nil
}

// Service --------------------------------------------------------------------

func (s *Service) ListBlocklist(ctx context.Context, limit int) ([]BlocklistEntry, error) {
	if !s.Available() {
		return []BlocklistEntry{}, nil
	}
	entries, err := s.store.ListBlocklist(ctx, limit)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []BlocklistEntry{}
	}
	return entries, nil
}

func (s *Service) DeleteBlocklistEntry(ctx context.Context, id string) error {
	if !s.Available() {
		return errors.New("wanted service requires database persistence")
	}
	return s.store.DeleteBlocklistEntry(ctx, id)
}

func (s *Service) ClearBlocklist(ctx context.Context, ids []string) (int, error) {
	if !s.Available() {
		return 0, errors.New("wanted service requires database persistence")
	}
	return s.store.DeleteBlocklistEntries(ctx, ids)
}

// BlocklistDownload resolves a download-client job to its release identity and
// blocklists it (queue-remove flow by default).
func (s *Service) BlocklistDownload(ctx context.Context, request BlocklistDownloadRequest) (BlocklistEntry, error) {
	if !s.Available() {
		return BlocklistEntry{}, errors.New("wanted service requires database persistence")
	}
	downloadID := strings.TrimSpace(request.DownloadID)
	if downloadID == "" {
		return BlocklistEntry{}, errors.New("download id is required")
	}
	source := strings.TrimSpace(request.Source)
	if source == "" {
		source = BlocklistSourceQueueRemove
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		reason = "removed from queue"
	}
	download := s.lookupDownload(ctx, downloadID, request.Client)
	item, _ := s.wantedItemForDownload(ctx, download)
	return s.blocklistDownloadRelease(ctx, item, download, reason, source)
}

// MarkDownloadFailedManually marks a download failed on operator request,
// optionally blocklisting the release identity and re-searching the wanted
// item for a replacement.
func (s *Service) MarkDownloadFailedManually(ctx context.Context, request ManualFailRequest) (ManualFailOutcome, error) {
	if !s.Available() {
		return ManualFailOutcome{}, errors.New("wanted service requires database persistence")
	}
	if s.acquire == nil {
		return ManualFailOutcome{}, errors.New("acquisition service is unavailable")
	}
	downloadID := strings.TrimSpace(request.ID)
	if downloadID == "" {
		return ManualFailOutcome{}, errors.New("download id is required")
	}
	download := s.lookupDownload(ctx, downloadID, request.Client)
	reason := "marked as failed by operator"
	if err := s.acquire.MarkDownloadFailed(ctx, downloadID, reason); err != nil {
		return ManualFailOutcome{}, err
	}

	outcome := ManualFailOutcome{}
	item, found := s.wantedItemForDownload(ctx, download)
	if found {
		_ = s.store.MarkWantedStatus(ctx, item.ID, "wanted")
	}
	_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
		EventType:  "download_marked_failed",
		EntityType: "wanted_item",
		EntityID:   item.ID,
		Severity:   "warning",
		Message:    "Marked download failed" + historyTitleSuffix(item),
		Data: map[string]any{
			"downloadId": downloadID,
			"name":       download.Name,
			"reason":     reason,
			"blocklist":  request.Blocklist,
			"research":   request.Research,
		},
	})

	if request.Blocklist {
		if _, err := s.blocklistDownloadRelease(ctx, item, download, reason, BlocklistSourceHistoryMarkFailed); err == nil {
			outcome.Blocklisted = true
		}
	}
	if request.Research && found {
		if _, err := s.searchReleasesForItem(ctx, item, SearchReleasesRequest{}); err == nil {
			outcome.SearchTriggered = true
		} else {
			_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
				EventType:  "wanted_search_failed",
				EntityType: "wanted_item",
				EntityID:   item.ID,
				Severity:   "error",
				Message:    "Wanted release search failed for " + item.Title,
				Data:       map[string]any{"error": err.Error(), "title": item.Title},
			})
		}
	}
	return outcome, nil
}

// blocklistDownloadRelease records the release identity behind a failed or
// removed download so future evaluations reject it. Existing matches are
// reused instead of duplicated.
func (s *Service) blocklistDownloadRelease(ctx context.Context, item WantedItem, download acquisition.DownloadStatus, reason string, source string) (BlocklistEntry, error) {
	var releases []ReleaseDecision
	if strings.TrimSpace(item.ID) != "" {
		releases, _ = s.store.ListReleaseDecisions(ctx, item.ID)
	}
	entry := blocklistEntryForDownload(item, download, releases, reason, source)
	if existing, found, err := s.store.MatchBlocklist(ctx, entry.InfoHash, entry.DownloadURLHash, entry.Title, entry.Indexer); err != nil {
		return BlocklistEntry{}, err
	} else if found {
		return existing, nil
	}
	stored, err := s.store.CreateBlocklistEntry(ctx, entry)
	if err != nil {
		return BlocklistEntry{}, err
	}
	_, _ = s.store.InsertHistoryEvent(ctx, HistoryEvent{
		EventType:  "release_blocklisted",
		EntityType: "wanted_item",
		EntityID:   stored.WantedItemID,
		Severity:   "info",
		Message:    "Blocklisted release " + stored.Title,
		Data: map[string]any{
			"blocklistId": stored.ID,
			"downloadId":  download.ID,
			"indexer":     stored.Indexer,
			"protocol":    stored.Protocol,
			"infohash":    stored.InfoHash,
			"reason":      stored.Reason,
			"source":      stored.Source,
		},
	})
	return stored, nil
}

func (s *Service) lookupDownload(ctx context.Context, downloadID string, client string) acquisition.DownloadStatus {
	fallback := acquisition.DownloadStatus{ID: downloadID, Client: strings.TrimSpace(client)}
	if s.acquire == nil {
		return fallback
	}
	downloads, err := s.acquire.Downloads(ctx, acquisition.DownloadListQuery{
		IDs:    []string{downloadID},
		Client: strings.TrimSpace(client),
	})
	if err != nil {
		return fallback
	}
	for _, download := range downloads {
		if strings.EqualFold(strings.TrimSpace(download.ID), downloadID) {
			return download
		}
	}
	return fallback
}

func (s *Service) wantedItemForDownload(ctx context.Context, download acquisition.DownloadStatus) (WantedItem, bool) {
	wantedID := wantedIDFromTags(download.Tags)
	if wantedID == "" {
		wantedID = strings.TrimSpace(download.WantedID)
	}
	if wantedID == "" {
		return WantedItem{}, false
	}
	item, err := s.store.GetWanted(ctx, wantedID)
	if err != nil {
		return WantedItem{}, false
	}
	return item, true
}

// blocklistEntryForDownload derives the release identity of a download.
// Download-client IDs are infohashes for qBittorrent and Transmission, so the
// ID doubles as the infohash when it looks like one; the stored release
// decision (matched by infohash or current-release linkage) supplies indexer,
// protocol, and download URL identity.
func blocklistEntryForDownload(item WantedItem, download acquisition.DownloadStatus, releases []ReleaseDecision, reason string, source string) BlocklistEntry {
	entry := BlocklistEntry{
		WantedItemID: strings.TrimSpace(item.ID),
		Title:        firstNonEmpty(download.Name, item.Title),
		Reason:       strings.TrimSpace(reason),
		Source:       strings.TrimSpace(source),
	}
	downloadID := strings.TrimSpace(download.ID)
	if looksLikeInfoHash(downloadID) {
		entry.InfoHash = strings.ToLower(downloadID)
	}
	if release, ok := releaseForDownload(item, download, releases); ok {
		entry.Title = firstNonEmpty(release.Title, entry.Title)
		entry.Indexer = strings.TrimSpace(release.Indexer)
		entry.Protocol = strings.TrimSpace(release.Protocol)
		entry.DownloadURLHash = hashDownloadURL(release.DownloadURL)
		if entry.InfoHash == "" && strings.TrimSpace(release.InfoHash) != "" {
			entry.InfoHash = strings.ToLower(strings.TrimSpace(release.InfoHash))
		}
	}
	return normalizeBlocklistEntry(entry)
}

func releaseForDownload(item WantedItem, download acquisition.DownloadStatus, releases []ReleaseDecision) (ReleaseDecision, bool) {
	downloadID := strings.TrimSpace(download.ID)
	for _, release := range releases {
		if downloadID != "" && strings.EqualFold(strings.TrimSpace(release.InfoHash), downloadID) {
			return release, true
		}
	}
	if item.CurrentReleaseID != "" {
		for _, release := range releases {
			if release.ID == item.CurrentReleaseID {
				return release, true
			}
		}
	}
	return ReleaseDecision{}, false
}

// Matching -------------------------------------------------------------------

func blocklistMatchesRelease(entries []BlocklistEntry, release acquisition.Release) bool {
	if len(entries) == 0 {
		return false
	}
	infohash := strings.ToLower(strings.TrimSpace(release.InfoHash))
	urlHash := hashDownloadURL(release.DownloadURL)
	title := normalizeText(release.Title)
	indexer := normalizeText(release.Indexer)
	for _, entry := range entries {
		if entry.InfoHash != "" && infohash != "" && strings.EqualFold(entry.InfoHash, infohash) {
			return true
		}
		if entry.DownloadURLHash != "" && urlHash != "" && strings.EqualFold(entry.DownloadURLHash, urlHash) {
			return true
		}
		entryTitle := normalizeText(entry.Title)
		entryIndexer := normalizeText(entry.Indexer)
		if entryTitle != "" && entryIndexer != "" && entryTitle == title && entryIndexer == indexer {
			return true
		}
	}
	return false
}

func hashDownloadURL(url string) string {
	url = strings.TrimSpace(url)
	if url == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:])
}

func looksLikeInfoHash(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func normalizeBlocklistEntry(entry BlocklistEntry) BlocklistEntry {
	entry.WantedItemID = strings.TrimSpace(entry.WantedItemID)
	entry.Title = strings.TrimSpace(entry.Title)
	entry.Indexer = strings.TrimSpace(entry.Indexer)
	entry.Protocol = strings.TrimSpace(entry.Protocol)
	entry.DownloadURLHash = strings.ToLower(strings.TrimSpace(entry.DownloadURLHash))
	entry.InfoHash = strings.ToLower(strings.TrimSpace(entry.InfoHash))
	entry.Reason = strings.TrimSpace(entry.Reason)
	entry.Source = strings.TrimSpace(entry.Source)
	if entry.Source == "" {
		entry.Source = BlocklistSourceAutoFailed
	}
	return entry
}

func historyTitleSuffix(item WantedItem) string {
	if strings.TrimSpace(item.Title) == "" {
		return ""
	}
	return " for " + item.Title
}
