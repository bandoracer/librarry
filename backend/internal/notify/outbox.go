package notify

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var ErrDeliveryNotFound = errors.New("notification delivery not found")
var ErrDeliveryConflict = errors.New("notification delivery changed or is still active; refresh before resolving it")
var ErrDeliveryConfirmation = errors.New("confirm this notification delivery decision before applying it")

const deliveryLock = `hashtextextended('librarry-notification:'||$1,0)`

type Delivery struct {
	ID                    string     `json:"id"`
	EventID               string     `json:"eventId"`
	Event                 Event      `json:"event"`
	TargetID              string     `json:"targetId"`
	TargetName            string     `json:"targetName"`
	TargetType            string     `json:"targetType"`
	TargetRevision        time.Time  `json:"targetRevision"`
	CurrentTargetRevision *time.Time `json:"currentTargetRevision"`
	TargetAvailable       bool       `json:"targetAvailable"`
	State                 string     `json:"state"`
	Attempts              int        `json:"attempts"`
	StatusCode            *int       `json:"statusCode"`
	Message               string     `json:"message"`
	NextAttemptAt         time.Time  `json:"nextAttemptAt"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}
type DeliveryPage struct {
	Items  []Delivery `json:"items"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

func (s *Service) Deliveries(ctx context.Context, limit, offset int) (DeliveryPage, error) {
	page := DeliveryPage{Items: []Delivery{}, Limit: limit, Offset: offset}
	if !s.Available() {
		return page, errors.New("notification service is unavailable")
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return page, errors.New("invalid notification page")
	}
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, `select count(*) from notification_deliveries`).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := tx.QueryContext(ctx, `select d.id::text,d.event_id::text,e.event,d.target_id::text,d.target_name,d.target_type,d.target_revision,t.updated_at,coalesce(t.enabled,false),d.state,d.attempts,d.status_code,d.message,d.next_attempt_at,d.created_at,d.updated_at from notification_deliveries d join notification_events e on e.id=d.event_id left join notification_targets t on t.id=d.target_id order by d.created_at desc,d.id desc limit $1 offset $2`, limit, offset)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		var d Delivery
		var raw []byte
		err = rows.Scan(&d.ID, &d.EventID, &raw, &d.TargetID, &d.TargetName, &d.TargetType, &d.TargetRevision, &d.CurrentTargetRevision, &d.TargetAvailable, &d.State, &d.Attempts, &d.StatusCode, &d.Message, &d.NextAttemptAt, &d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			rows.Close()
			return page, err
		}
		if err = json.Unmarshal(raw, &d.Event); err != nil {
			rows.Close()
			return page, err
		}
		d.Event.ID, d.Event.OccurredAt = d.EventID, d.CreatedAt
		page.Items = append(page.Items, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	return page, tx.Commit()
}

func (s *Service) lockDelivery(ctx context.Context, id string) (*sql.Conn, error) {
	conn, err := s.store.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var locked bool
	if err = conn.QueryRowContext(ctx, `select pg_try_advisory_lock(`+deliveryLock+`)`, id).Scan(&locked); err != nil {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		conn.Close()
		return nil, err
	}
	if !locked {
		conn.Close()
		return nil, ErrDeliveryConflict
	}
	return conn, nil
}
func releaseDelivery(conn *sql.Conn, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var released bool
	if err := conn.QueryRowContext(ctx, `select pg_advisory_unlock(`+deliveryLock+`)`, id).Scan(&released); err != nil || !released {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}
	_ = conn.Close()
}

// RunPending is bounded and fair by due time. A disconnected sender is never
// silently replayed: reclaiming its lock changes sending to uncertain.
func (s *Service) RunPending(ctx context.Context) (int, error) {
	if !s.Available() {
		return 0, errors.New("notification service is unavailable")
	}
	rows, err := s.store.db.QueryContext(ctx, `select id::text from notification_deliveries where state in ('pending','retry','sending') and next_attempt_at<=now() order by next_attempt_at,id limit 25`)
	if err != nil {
		return 0, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, id := range ids {
		if err = s.processDelivery(ctx, id); errors.Is(err, ErrDeliveryConflict) {
			continue
		} else if err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (s *Service) processDelivery(ctx context.Context, id string) error {
	conn, err := s.lockDelivery(ctx, id)
	if err != nil {
		return err
	}
	defer releaseDelivery(conn, id)
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state, targetID, eventID string
	var revision, occurredAt time.Time
	var due bool
	var raw []byte
	err = tx.QueryRowContext(ctx, `select d.state,d.target_id::text,d.target_revision,d.next_attempt_at<=now(),e.event,e.id::text,e.created_at from notification_deliveries d join notification_events e on e.id=d.event_id where d.id::text=$1 for update of d`, id).Scan(&state, &targetID, &revision, &due, &raw, &eventID, &occurredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDeliveryNotFound
	}
	if err != nil {
		return err
	}
	if state == "sending" {
		if _, err = tx.ExecContext(ctx, `update notification_delivery_attempts set state='uncertain',message='Sender stopped before acceptance was recorded',finished_at=now() where delivery_id=$1 and state='sending'`, id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `update notification_deliveries set state='uncertain',message='Sender stopped before acceptance was recorded; inspect the receiver before retrying',updated_at=clock_timestamp() where id=$1`, id); err != nil {
			return err
		}
		return tx.Commit()
	}
	if (state != "pending" && state != "retry") || !due {
		return nil
	}
	target, err := scanTarget(tx.QueryRowContext(ctx, `select `+targetColumns+` from notification_targets where id=$1 for share`, targetID))
	var event Event
	if jsonErr := json.Unmarshal(raw, &event); jsonErr != nil {
		return jsonErr
	}
	event.ID, event.OccurredAt = eventID, occurredAt
	problem := ""
	switch {
	case errors.Is(err, sql.ErrNoRows):
		problem = "Connection was deleted; this delivery was not sent"
	case err != nil:
		return err
	case !target.Enabled || !target.Triggers.Matches(event.Type):
		problem = "Connection is disabled or no longer receives this event; this delivery was not sent"
	case !target.UpdatedAt.Equal(revision):
		problem = "Connection settings changed; review before sending with current settings"
	}
	if problem != "" {
		_, err = tx.ExecContext(ctx, `update notification_deliveries set state='cancelled',message=$2,updated_at=clock_timestamp() where id=$1`, id, problem)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	req, err := s.buildRequest(ctx, target, event)
	if err != nil {
		_, err = tx.ExecContext(ctx, `update notification_deliveries set state='failed',message='Notification request settings are invalid',updated_at=clock_timestamp() where id=$1`, id)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	var token string
	var attempts int
	err = tx.QueryRowContext(ctx, `update notification_deliveries set state='sending',attempts=attempts+1,run_token=gen_random_uuid(),status_code=null,message='',updated_at=clock_timestamp() where id=$1 returning run_token::text,attempts`, id).Scan(&token, &attempts)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `insert into notification_delivery_attempts(id,delivery_id,state) values($1,$2,'sending')`, token, id); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	req.Header.Set("X-Librarry-Delivery-ID", id)
	// Stable IDs allow receivers to deduplicate explicit replays. They do not
	// imply that third-party notification providers implement idempotency. Avoid
	// Idempotency-Key: Go's transport could otherwise silently replay the POST.
	req.Header.Set("X-Librarry-Event-ID", eventID)
	response, sendErr := s.client.Do(req)
	outcome, message, code, delay := "uncertain", "Notification acceptance could not be verified; inspect the receiver before retrying", 0, time.Duration(0)
	if sendErr == nil {
		code = response.StatusCode
		response.Body.Close()
		switch {
		case code >= 200 && code < 300:
			outcome, message = "accepted", "Receiver accepted the request"
		case code == http.StatusTooManyRequests:
			if attempts < 5 {
				outcome, message = "retry", "Receiver rate limited the request"
				delay = notificationRetryDelay(response.Header.Get("Retry-After"), attempts, time.Now())
				if delay > 24*time.Hour {
					outcome, message, delay = "failed", "Receiver requested more than 24 hours of backoff; review before retrying", 0
				}
			} else {
				outcome, message = "failed", "Receiver rate limited five attempts; review before retrying"
			}
		case code >= 300 && code < 500 && code != http.StatusRequestTimeout:
			outcome, message = "failed", fmt.Sprintf("Receiver returned HTTP %d", code)
		}
	}
	// Save through the original locked session. A stale sender cannot overwrite
	// a successor after its database connection disappears.
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err = conn.BeginTx(saveCtx, nil)
	if err != nil {
		return errors.New("notification attempt finished but its result could not be saved")
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(saveCtx, `update notification_deliveries set state=$3,message=$4,status_code=nullif($5,0),next_attempt_at=now()+($6*interval '1 millisecond'),updated_at=clock_timestamp() where id=$1 and run_token=$2 and state='sending'`, id, token, outcome, message, code, delay.Milliseconds())
	if err != nil {
		return errors.New("notification result could not be saved")
	}
	if n, e := result.RowsAffected(); e != nil || n != 1 {
		return ErrDeliveryConflict
	}
	if _, err = tx.ExecContext(saveCtx, `update notification_delivery_attempts set state=$2,message=$3,status_code=nullif($4,0),finished_at=now() where id=$1`, token, outcome, message, code); err != nil {
		return err
	}
	return tx.Commit()
}
func notificationRetryDelay(value string, attempt int, now time.Time) time.Duration {
	delay := time.Minute * time.Duration(1<<min(attempt-1, 4))
	if seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && seconds > 0 {
		if seconds > 86400 {
			seconds = 86401
		}
		delay = max(delay, time.Duration(seconds)*time.Second)
	} else if date, err := http.ParseTime(value); err == nil {
		delay = max(delay, date.Sub(now))
	}
	return delay
}

type DeliveryResolution struct {
	Action                 string     `json:"action"`
	Confirm                bool       `json:"confirm"`
	ExpectedUpdatedAt      time.Time  `json:"expectedUpdatedAt"`
	ExpectedTargetRevision *time.Time `json:"expectedTargetRevision"`
}

func (s *Service) ResolveDelivery(ctx context.Context, id string, request DeliveryResolution) error {
	if !s.Available() {
		return errors.New("notification service is unavailable")
	}
	if !request.Confirm {
		return ErrDeliveryConfirmation
	}
	if request.Action != "retry" && request.Action != "accepted" && request.Action != "cancel" {
		return errors.New("invalid notification resolution")
	}
	conn, err := s.lockDelivery(ctx, id)
	if err != nil {
		return err
	}
	defer releaseDelivery(conn, id)
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state, targetID string
	var updated, revision time.Time
	var raw []byte
	err = tx.QueryRowContext(ctx, `select d.state,d.target_id::text,d.updated_at,d.target_revision,e.event from notification_deliveries d join notification_events e on e.id=d.event_id where d.id::text=$1 for update of d`, id).Scan(&state, &targetID, &updated, &revision, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDeliveryNotFound
	}
	if err != nil {
		return err
	}
	if !updated.Equal(request.ExpectedUpdatedAt) || state == "sending" || state == "accepted" {
		return ErrDeliveryConflict
	}
	next, message := "cancelled", "Operator cancelled this delivery"
	if request.Action == "accepted" {
		if state != "uncertain" {
			return ErrDeliveryConflict
		}
		next, message = "accepted", "Operator confirmed receiver acceptance"
	}
	if request.Action == "retry" {
		if state != "failed" && state != "uncertain" && state != "cancelled" {
			return ErrDeliveryConflict
		}
		target, err := scanTarget(tx.QueryRowContext(ctx, `select `+targetColumns+` from notification_targets where id=$1 for share`, targetID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeliveryConflict
		}
		if err != nil {
			return err
		}
		var event Event
		if err = json.Unmarshal(raw, &event); err != nil {
			return err
		}
		if request.ExpectedTargetRevision == nil || !target.UpdatedAt.Equal(*request.ExpectedTargetRevision) || !target.Enabled || !target.Triggers.Matches(event.Type) {
			return ErrDeliveryConflict
		}
		revision = target.UpdatedAt
		next, message = "pending", "Operator confirmed retry using current connection settings"
	}
	if _, err = tx.ExecContext(ctx, `insert into notification_delivery_actions(delivery_id,action,previous_state,target_revision) values($1,$2,$3,$4)`, id, request.Action, state, revision); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `update notification_deliveries set state=$2,message=$3,target_revision=$4,next_attempt_at=now(),updated_at=clock_timestamp() where id=$1`, id, next, message, revision); err != nil {
		return err
	}
	return tx.Commit()
}

// ObserveHealth persists transitions and their target fan-out atomically. A
// restart or a second API process cannot announce the same unhealthy episode.
func (s *Service) ObserveHealth(ctx context.Context, id, severity, name, message string) error {
	if !s.Available() {
		return nil
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `insert into notification_health_states(check_id,severity) values($1,'ok') on conflict do nothing`, id); err != nil {
		return err
	}
	var previous string
	if err = tx.QueryRowContext(ctx, `select severity from notification_health_states where check_id=$1 for update`, id).Scan(&previous); err != nil {
		return err
	}
	if severity != "ok" && previous == "ok" {
		event := HealthIssueEvent(name, severity, message)
		event.Fields["checkId"] = id
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `select enqueue_native_notification('health:'||$1||':'||gen_random_uuid()::text,$2::jsonb)`, id, string(raw)); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `update notification_health_states set severity=$2,updated_at=now() where check_id=$1`, id, severity); err != nil {
		return err
	}
	return tx.Commit()
}
