package wanted

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrInvalidUpgradeRequest = errors.New("invalid upgrade request")

// NormalizeUpgradeRequest keeps explicit selection independent of the scheduled
// batch size. Empty/invalid IDs must never broaden an action to the whole queue.
func NormalizeUpgradeRequest(request UpgradeRequest) (UpgradeRequest, error) {
	if request.Limit < 0 || request.Limit > 200 {
		return request, fmt.Errorf("%w: limit must be between 0 and 200", ErrInvalidUpgradeRequest)
	}
	if len(request.WantedIDs) > 200 {
		return request, fmt.Errorf("%w: select at most 200 books per request", ErrInvalidUpgradeRequest)
	}
	ids := make([]string, 0, len(request.WantedIDs))
	seen := map[string]bool{}
	for _, id := range request.WantedIDs {
		var parsed pgtype.UUID
		if err := parsed.Scan(strings.TrimSpace(id)); err != nil || !parsed.Valid {
			return request, fmt.Errorf("%w: wantedIds must contain valid book UUIDs", ErrInvalidUpgradeRequest)
		}
		value, _ := parsed.Value()
		id = value.(string)
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	request.WantedIDs = ids
	if len(ids) > 0 {
		request.Limit = len(ids)
	} else if request.Limit == 0 {
		request.Limit = defaultWantedMonitorLimit
	}
	return request, nil
}

// selectedUpgradeBooks preserves request order and reports ineligible selections
// instead of dropping them through the scheduled queue's filters. Validate the
// complete selection before starting a run or touching any scheduling clocks.
func (s *Store) selectedUpgradeBooks(ctx context.Context, ids []string, interval time.Duration, force bool) ([]WantedItem, map[string]string, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `select `+wantedDetailColumns+`
	 from wanted_items wi left join works w on w.id=wi.work_id where wi.id=any($1::uuid[])`, ids)
	if err != nil {
		return nil, nil, err
	}
	byID := map[string]WantedItem{}
	for rows.Next() {
		item, err := scanWanted(rows)
		if err != nil {
			rows.Close()
			return nil, nil, err
		}
		byID[item.ID] = item
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	items := make([]WantedItem, 0, len(ids))
	for _, id := range ids {
		item, ok := byID[id]
		if !ok {
			return nil, nil, fmt.Errorf("%w: selected book %s no longer exists; refresh the selection", ErrInvalidUpgradeRequest, id)
		}
		items = append(items, item)
	}
	items, err = attachWantedDetails(ctx, tx, items)
	if err != nil {
		return nil, nil, err
	}
	rows, err = tx.QueryContext(ctx, `select id::text, case
	 when not monitored or status not in ('wanted','grabbed','imported') then 'book is no longer monitored'
	 when not $2::boolean and (last_upgrade_search_at > $3 or last_upgrade_checked_at > $4) then 'upgrade check is not due; use Force to check now'
	 else '' end from wanted_items where id=any($1::uuid[])`, ids, force, time.Now().UTC().Add(-interval), workerCheckCutoff(interval))
	if err != nil {
		return nil, nil, err
	}
	skips := map[string]string{}
	for rows.Next() {
		var id, reason string
		if err := rows.Scan(&id, &reason); err != nil {
			rows.Close()
			return nil, nil, err
		}
		skips[id] = reason
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	return items, skips, tx.Commit()
}
