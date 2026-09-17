package wanted

import (
	"context"
	"database/sql"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

// acquisitionSummary counts the complete active acquisition ledger, independently
// of the bounded action preview. Import history is not current file presence.
func (s *Store) acquisitionSummary(ctx context.Context, status string, downloads map[string][]acquisition.DownloadStatus, evidence string) (AcquisitionQueueSummary, error) {
	var result AcquisitionQueueSummary
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `with release_counts as (
 select wanted_item_id,count(*) as total,count(*) filter(where approved) as approved from releases group by wanted_item_id
 ) select wi.id::text,wi.status,wi.last_search_at,coalesce(r.total,0),coalesce(r.approved,0)
 from wanted_items wi left join release_counts r on r.wanted_item_id=wi.id
 where wi.status not in ('removed','ignored') and ($1='' or wi.status=$1)`, strings.TrimSpace(status))
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item WantedItem
		var row AcquisitionQueueItem
		if err = rows.Scan(&item.ID, &item.Status, &item.LastSearchAt, &row.ReleaseCount, &row.ApprovedCount); err != nil {
			rows.Close()
			return result, err
		}
		row.Downloads = downloads[item.ID]
		state, _ := acquisitionQueueState(item, row)
		// Missing client evidence must not turn a possibly queued acquisition into
		// an invitation to regrab. Positive observations remain useful during a partial read.
		if (evidence == "partial" || evidence == "unavailable") && len(row.Downloads) == 0 && item.Status != "imported" {
			state = "unknown"
		}
		result.Total++
		switch state {
		case "needs_search":
			result.NeedsSearch++
		case "ready_to_grab":
			result.ReadyToGrab++
		case "queued", "downloading":
			result.Queued++
		case "import_ready":
			result.ImportReady++
		case "imported":
			result.Imported++
		case "blocked":
			result.Blocked++
		default:
			result.Unknown++
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func (s *Store) acquisitionPreview(ctx context.Context, status string, limit int) ([]WantedItem, error) {
	rows, err := s.db.QueryContext(ctx, `select `+wantedDetailColumns+` from wanted_items wi left join works w on w.id=wi.work_id
 where wi.status not in ('removed','ignored') and ($1='' or wi.status=$1) order by wi.created_at desc,wi.id limit $2`, status, limit)
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
