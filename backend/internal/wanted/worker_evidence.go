package wanted

import (
	"context"
	"database/sql"
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func (s *Store) markWorkerChecked(ctx context.Context, id, kind string) error {
	table, column, predicate := "wanted_items", "last_monitor_checked_at", "monitored and status in ('wanted','grabbed','imported')"
	switch kind {
	case "monitor":
	case "upgrade":
		column = "last_upgrade_checked_at"
	case "author":
		table = "author_subscriptions"
		column = "last_sync_attempt_at"
		predicate = "status='monitored' and monitor_new_items and missing_book_policy<>'none'"
	default:
		return errors.New("unknown worker check kind")
	}
	// Do not advance updated_at: this records scheduling, not an owner-setting
	// revision or successful search/sync. Column identifiers are closed above.
	result, err := s.db.ExecContext(ctx, `update `+table+` set `+column+`=now() where id=$1 and `+predicate, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type workerEvidence struct {
	files     map[string]FileEvidence
	downloads acquisition.DownloadEvidence
	inFlight  map[string]bool
}

func (s *Service) workerEvidence(ctx context.Context, items []WantedItem) (workerEvidence, error) {
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	files, err := s.store.WantedFileEvidence(ctx, ids)
	if err != nil {
		return workerEvidence{}, err
	}
	evidence := workerEvidence{files: files, downloads: s.liveBookDownloads(ctx), inFlight: map[string]bool{}}
	for id, downloads := range groupDownloadsByWantedID(evidence.downloads.Downloads) {
		for _, d := range downloads {
			if downloadSupportsInFlight(d) {
				evidence.inFlight[id] = true
				break
			}
		}
	}
	return evidence, nil
}
func (e workerEvidence) skipReason(item WantedItem, upgrade bool) string {
	if !item.Monitored || item.Status == "removed" || item.Status == "ignored" {
		return "book is no longer monitored"
	}
	if e.inFlight[item.ID] {
		return "book already has a live acquisition"
	}
	if e.downloads.Status != "fresh" && e.downloads.Status != "notConfigured" {
		return "download-client evidence is unavailable or incomplete"
	}
	file := e.files[item.ID]
	if file.State == "unknown" || file.State == "unavailable" || file.State == "" {
		return "library file evidence is unavailable or unverified"
	}
	if upgrade && file.State != "present" {
		return "upgrade requires a complete present library copy"
	}
	if !upgrade && file.State == "present" {
		return "library media is already present"
	}
	return ""
}

// Read the owner's settings again before provider IO. The selected candidate may
// have waited behind other books. Checks remain fair even when it is skipped.
func (s *Service) checkedWorkerItem(ctx context.Context, item WantedItem, kind string) (WantedItem, error) {
	if err := s.store.markWorkerChecked(ctx, item.ID, kind); err != nil {
		return item, err
	}
	return s.store.GetWanted(ctx, item.ID)
}

// Revalidate after provider IO before an automatic acquisition. This is a
// preflight; the acquisition store also fences stopped/removed books under its
// transaction and keeps the existing idempotent acquisition reservation.
func (s *Service) validateAutomaticGrab(ctx context.Context, item WantedItem, release ReleaseDecision, trigger string) error {
	current, err := s.store.GetWanted(ctx, item.ID)
	if err != nil {
		return err
	}
	if !current.Monitored || current.Status == "removed" || current.Status == "ignored" {
		return errors.New("automatic acquisition stopped: book is no longer monitored")
	}
	if !sameWantedAcquisitionSettings(current, item) {
		return errors.New("automatic acquisition stopped: book settings changed; search again")
	}
	evidence, err := s.workerEvidence(ctx, []WantedItem{current})
	if err != nil {
		return err
	}
	if reason := evidence.skipReason(current, trigger == "upgrade"); reason != "" {
		return errors.New("automatic acquisition stopped: " + reason)
	}
	if trigger == "upgrade" {
		profiles, err := s.store.ListQualityProfiles(ctx)
		if err != nil {
			return err
		}
		profile := profileFromList(profiles, current)
		score := s.currentReleaseScore(ctx, current)
		if !cutoffUnmet(profile, score) || release.Score <= score {
			return errors.New("automatic acquisition stopped: the current copy no longer needs this upgrade")
		}
	}
	return nil
}

func sameWantedAcquisitionSettings(a, b WantedItem) bool {
	return a.Title == b.Title && a.AuthorName == b.AuthorName && a.Format == b.Format && a.QualityProfile == b.QualityProfile && a.RootFolderID == b.RootFolderID && slices.Equal(a.Tags, b.Tags) && maps.Equal(wantedManualOverrideValues(a), wantedManualOverrideValues(b))
}

// Feed matching traverses every eligible book in stable ID batches. Changes made
// after a page may appear on the next run; no whole-run snapshot is claimed.
func (s *Store) listFeedWantedPage(ctx context.Context, after, format string) ([]WantedItem, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	rows, err := s.db.QueryContext(ctx, `
		select
			wi.id, wi.work_id, wi.edition_id, coalesce(nullif(wi.title, ''), w.title),
			coalesce(nullif(wi.author_name, ''), ''), coalesce(nullif(wi.cover_url, ''), w.cover_url),
			wi.wanted_format, wi.quality_profile, wi.status, wi.monitored, wi.metadata_provider,
			wi.source_key, coalesce(wi.current_release_id::text, ''), wi.current_release_score,
			coalesce(wi.root_folder_id::text, ''), wi.series, wi.series_position, wi.first_publish_year,
			wi.tags, wi.release_date, wi.last_search_at, wi.last_upgrade_search_at, wi.created_at, wi.updated_at
		from wanted_items wi
		left join works w on w.id = wi.work_id
 where wi.monitored and wi.status in ('wanted','grabbed','imported')
 and ($1='' or wi.id>nullif($1,'')::uuid)
 and ($2='' or $2='any' or wi.wanted_format=$2)
 order by wi.id limit 200`, after, format)
	if err != nil {
		return nil, err
	}
	items := []WantedItem{}
	for rows.Next() {
		item, err := scanWanted(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return s.attachWantedManualOverrides(ctx, items)
}

func appendFeedMatch(run *FeedSyncRun, match FeedSyncMatch) {
	if len(run.Matches) < 1000 {
		run.Matches = append(run.Matches, match)
	} else {
		run.MatchesTruncated = true
	}
}

// Failed/skipped checks get a short backoff; successful searches/syncs still
// obey their longer configured intervals. Force bypasses both waiting periods.
func workerCheckCutoff(interval time.Duration) time.Time {
	if interval > 15*time.Minute {
		interval = 15 * time.Minute
	}
	return time.Now().UTC().Add(-interval)
}
