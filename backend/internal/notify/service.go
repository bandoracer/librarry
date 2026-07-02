package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Service fans events out to enabled notification targets. Delivery uses a
// 10s timeout per request and logs failures instead of returning them to the
// automation that raised the event.
type Service struct {
	store  *Store
	logger *slog.Logger
	client *http.Client
	// telegramAPIBase is overridable in tests; production uses the public
	// Telegram Bot API.
	telegramAPIBase string
}

func NewService(store *Store, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		store:           store,
		logger:          logger,
		client:          &http.Client{Timeout: 10 * time.Second},
		telegramAPIBase: "https://api.telegram.org",
	}
}

// Available reports whether persisted targets can be loaded.
func (s *Service) Available() bool {
	return s != nil && s.store.Configured()
}

// Targets lists all persisted targets.
func (s *Service) Targets(ctx context.Context) ([]Target, error) {
	if !s.Available() {
		return nil, errors.New("notification service is unavailable")
	}
	return s.store.ListTargets(ctx)
}

func (s *Service) Target(ctx context.Context, id string) (Target, error) {
	if !s.Available() {
		return Target{}, errors.New("notification service is unavailable")
	}
	return s.store.GetTarget(ctx, id)
}

func (s *Service) CreateTarget(ctx context.Context, target Target) (Target, error) {
	if !s.Available() {
		return Target{}, errors.New("notification service is unavailable")
	}
	normalized, err := NormalizeTarget(target)
	if err != nil {
		return Target{}, err
	}
	return s.store.CreateTarget(ctx, normalized)
}

func (s *Service) UpdateTarget(ctx context.Context, id string, target Target) (Target, error) {
	if !s.Available() {
		return Target{}, errors.New("notification service is unavailable")
	}
	normalized, err := NormalizeTarget(target)
	if err != nil {
		return Target{}, err
	}
	return s.store.UpdateTarget(ctx, id, normalized)
}

func (s *Service) DeleteTarget(ctx context.Context, id string) error {
	if !s.Available() {
		return errors.New("notification service is unavailable")
	}
	return s.store.DeleteTarget(ctx, id)
}

// NormalizeTarget trims and validates a target before persistence.
func NormalizeTarget(target Target) (Target, error) {
	target.Name = strings.TrimSpace(target.Name)
	target.Type = strings.ToLower(strings.TrimSpace(target.Type))
	if target.Name == "" {
		return Target{}, errors.New("notification target name is required")
	}
	settings := map[string]string{}
	for key, value := range target.Settings {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		settings[key] = value
	}
	target.Settings = settings
	switch target.Type {
	case TargetTypeWebhook:
		if target.Setting("url") == "" {
			return Target{}, errors.New("webhook targets require settings.url")
		}
	case TargetTypeNtfy:
		if target.Setting("url") == "" && target.Setting("topic") == "" {
			return Target{}, errors.New("ntfy targets require settings.url or settings.topic")
		}
	case TargetTypeDiscord:
		if target.Setting("webhookUrl", "url") == "" {
			return Target{}, errors.New("discord targets require settings.webhookUrl")
		}
	case TargetTypeTelegram:
		if target.Setting("botToken") == "" {
			return Target{}, errors.New("telegram targets require settings.botToken")
		}
		if target.Setting("chatId") == "" {
			return Target{}, errors.New("telegram targets require settings.chatId")
		}
	default:
		return Target{}, fmt.Errorf("notification type %q is not supported (webhook, ntfy, discord, telegram)", target.Type)
	}
	return target, nil
}

// Dispatch fans the event out to every enabled target whose triggers match.
// Errors are logged, never returned: notifications must not fail automation.
func (s *Service) Dispatch(ctx context.Context, event Event) {
	if !s.Available() {
		return
	}
	targets, err := s.store.ListTargets(ctx)
	if err != nil {
		s.logger.Warn("notification targets unavailable", "event", event.Type, "error", err)
		return
	}
	for _, target := range targets {
		if !target.Enabled || !target.Triggers.Matches(event.Type) {
			continue
		}
		if err := s.Deliver(ctx, target, event); err != nil {
			s.logger.Warn("notification delivery failed",
				"target", target.Name, "type", target.Type, "event", event.Type, "error", err)
		}
	}
}

// DispatchAll dispatches a batch of events.
func (s *Service) DispatchAll(ctx context.Context, events []Event) {
	for _, event := range events {
		s.Dispatch(ctx, event)
	}
}

// Deliver sends one event to one target (also used by the test endpoint).
func (s *Service) Deliver(ctx context.Context, target Target, event Event) error {
	if s == nil {
		return errors.New("notification service is unavailable")
	}
	req, err := s.buildRequest(ctx, target, event)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s returned %s", target.Type, resp.Status)
	}
	return nil
}

func (s *Service) buildRequest(ctx context.Context, target Target, event Event) (*http.Request, error) {
	switch strings.ToLower(strings.TrimSpace(target.Type)) {
	case TargetTypeWebhook:
		return buildWebhookRequest(ctx, target, event)
	case TargetTypeNtfy:
		return buildNtfyRequest(ctx, target, event)
	case TargetTypeDiscord:
		return buildDiscordRequest(ctx, target, event)
	case TargetTypeTelegram:
		return buildTelegramRequest(ctx, s.telegramAPIBase, target, event)
	default:
		return nil, fmt.Errorf("notification type %q is not supported", target.Type)
	}
}
