package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const notificationRetentionDays = 90
const notificationRetentionBatch = 100

type RetentionReport struct {
	Events     int `json:"events"`
	Deliveries int `json:"deliveries"`
	Accepted   int `json:"accepted"`
	Cancelled  int `json:"cancelled"`
	Attempts   int `json:"attempts"`
	Actions    int `json:"actions"`
	Skipped    int `json:"skipped"`
}

// PruneHistory compacts only fully resolved events. The event UUID/source key
// remain a permanent replay barrier. No receivers, domain journals, credentials,
// or health-episode state participate in maintenance.
func (s *Service) PruneHistory(ctx context.Context) (RetentionReport, error) {
	report := RetentionReport{}
	if !s.Available() {
		return report, errors.New("notification service is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	rows, err := s.store.db.QueryContext(ctx, `select e.id::text from notification_events e where e.archived_at is null and e.created_at<now()-($1*interval '1 day') and not exists(select 1 from notification_deliveries d where d.event_id=e.id and (d.state not in ('accepted','cancelled') or d.resolved_at is null or d.resolved_at>=now()-($1*interval '1 day'))) order by e.created_at,e.id limit $2`, notificationRetentionDays, notificationRetentionBatch)
	if err != nil {
		return report, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return report, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, err
	}
	for _, id := range ids {
		item, err := s.pruneEvent(ctx, id)
		if err != nil {
			return report, err
		}
		report.Events += item.Events
		report.Deliveries += item.Deliveries
		report.Accepted += item.Accepted
		report.Cancelled += item.Cancelled
		report.Attempts += item.Attempts
		report.Actions += item.Actions
		report.Skipped += item.Skipped
	}
	return report, nil
}

func (s *Service) pruneEvent(ctx context.Context, id string) (RetentionReport, error) {
	skipped := RetentionReport{Skipped: 1}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return RetentionReport{}, err
	}
	defer tx.Rollback()
	var exists string
	err = tx.QueryRowContext(ctx, `select id::text from notification_events where id=$1 and archived_at is null and created_at<now()-($2*interval '1 day') for update skip locked`, id, notificationRetentionDays).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return skipped, nil
	}
	if err != nil {
		return RetentionReport{}, err
	}
	// Lock the event against new FK references, then use the same advisory keys
	// as delivery/review. A concurrent retry wins without blocking or deleting its
	// payload; it is rechecked after all locks are acquired.
	rows, err := tx.QueryContext(ctx, `select id::text from notification_deliveries where event_id=$1 order by id`, id)
	if err != nil {
		return RetentionReport{}, err
	}
	ids := []string{}
	for rows.Next() {
		var deliveryID string
		if err = rows.Scan(&deliveryID); err != nil {
			rows.Close()
			return RetentionReport{}, err
		}
		ids = append(ids, deliveryID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return RetentionReport{}, err
	}
	for _, deliveryID := range ids {
		var locked bool
		if err = tx.QueryRowContext(ctx, `select pg_try_advisory_xact_lock(`+deliveryLock+`)`, deliveryID).Scan(&locked); err != nil {
			return RetentionReport{}, err
		}
		if !locked {
			return skipped, nil
		}
	}
	var unresolved bool
	if err = tx.QueryRowContext(ctx, `select exists(select 1 from notification_deliveries where event_id=$1 and (state not in ('accepted','cancelled') or resolved_at is null or resolved_at>=now()-($2*interval '1 day')))`, id, notificationRetentionDays).Scan(&unresolved); err != nil {
		return RetentionReport{}, err
	}
	if unresolved {
		return skipped, nil
	}
	report := RetentionReport{Events: 1}
	if err = tx.QueryRowContext(ctx, `select count(*),count(*) filter(where state='accepted'),count(*) filter(where state='cancelled') from notification_deliveries where event_id=$1`, id).Scan(&report.Deliveries, &report.Accepted, &report.Cancelled); err != nil {
		return RetentionReport{}, err
	}
	result, err := tx.ExecContext(ctx, `delete from notification_delivery_attempts where delivery_id in(select id from notification_deliveries where event_id=$1)`, id)
	if err != nil {
		return RetentionReport{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return RetentionReport{}, err
	}
	report.Attempts = int(n)
	result, err = tx.ExecContext(ctx, `delete from notification_delivery_actions where delivery_id in(select id from notification_deliveries where event_id=$1)`, id)
	if err != nil {
		return RetentionReport{}, err
	}
	n, err = result.RowsAffected()
	if err != nil {
		return RetentionReport{}, err
	}
	report.Actions = int(n)
	if _, err = tx.ExecContext(ctx, `delete from notification_deliveries where event_id=$1`, id); err != nil {
		return RetentionReport{}, err
	}
	summary, err := json.Marshal(report)
	if err != nil {
		return RetentionReport{}, err
	}
	if _, err = tx.ExecContext(ctx, `update notification_events set event='{}',compat_context='{}',archived_at=clock_timestamp(),retention_summary=$2 where id=$1`, id, string(summary)); err != nil {
		return RetentionReport{}, err
	}
	if err = tx.Commit(); err != nil {
		return RetentionReport{}, err
	}
	return report, nil
}
