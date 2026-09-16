package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

// CompatibilityAdapter builds the existing Readarr payload from an immutable
// domain snapshot and current target settings. Configure before workers start.
type CompatibilityAdapter func(context.Context, json.RawMessage, Event, json.RawMessage) (*http.Request, error)

func (s *Service) WithCompatibilityAdapter(adapter CompatibilityAdapter) *Service {
	s.compatibilityAdapter = adapter
	return s
}

func loadDeliveryTarget(ctx context.Context, tx *sql.Tx, kind, id string, eventType EventType) (Target, json.RawMessage, error) {
	if kind == "native" {
		target, err := scanTarget(tx.QueryRowContext(ctx, `select `+targetColumns+` from notification_targets where id=$1 for share`, id))
		return target, nil, err
	}
	if kind != "compat" {
		return Target{}, nil, errors.New("unsupported notification target kind")
	}
	var target Target
	var payload []byte
	err := tx.QueryRowContext(ctx, `select id::text,name,payload,notification_compat_matches(payload,$2),created_at,updated_at from compat_resources where id=$1 and resource_type='notification' and deleted_at is null for share`, id, string(eventType)).Scan(&target.ID, &target.Name, &payload, &target.Enabled, &target.CreatedAt, &target.UpdatedAt)
	target.Type = "readarrWebhook"
	target.Triggers = Triggers{OnGrab: true, OnImport: true, OnUpgrade: true, OnDownloadFailure: true, OnHealthIssue: true}
	return target, payload, err
}
func (s *Service) buildDeliveryRequest(ctx context.Context, kind string, target Target, settings json.RawMessage, event Event, snapshot json.RawMessage) (*http.Request, error) {
	if kind == "native" {
		return s.buildRequest(ctx, target, event)
	}
	if s.compatibilityAdapter == nil {
		return nil, errors.New("compatibility notification adapter is unavailable")
	}
	req, err := s.compatibilityAdapter(ctx, settings, event, snapshot)
	// A user-selected GET/PUT method must not cause Go's transport to silently
	// replay a notification after a lost response, even with a reusable body.
	if req != nil {
		req.GetBody = nil
	}
	return req, err
}
