package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type ResourceLister interface {
	ListResources(ctx context.Context, resourceType string) ([]Resource, error)
}

type ReleaseRestrictionProvider struct {
	store ResourceLister
}

func NewReleaseRestrictionProvider(store ResourceLister) *ReleaseRestrictionProvider {
	if store == nil {
		return nil
	}
	return &ReleaseRestrictionProvider{store: store}
}

func (p *ReleaseRestrictionProvider) ListReleaseRestrictions(ctx context.Context) ([]wanted.ReleaseRestriction, error) {
	if p == nil || p.store == nil {
		return nil, nil
	}
	resources, err := p.store.ListResources(ctx, "restriction")
	if err != nil {
		return nil, err
	}
	restrictions := make([]wanted.ReleaseRestriction, 0, len(resources))
	for _, resource := range resources {
		payload := resource.Payload
		restrictions = append(restrictions, wanted.NewReleaseRestriction(
			strconv.Itoa(resource.CompatID),
			firstPayloadString(payload, "required", "mustContain", "requiredTerms"),
			firstPayloadString(payload, "ignored", "mustNotContain", "ignoredTerms"),
			firstPayloadString(payload, "preferred", "preferredTerms"),
			payloadIntArray(payload, "tags"),
		))
	}
	return restrictions, nil
}

func firstPayloadString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := payloadString(payload, key); value != "" {
			return value
		}
	}
	return ""
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []string:
		return strings.Join(typed, ",")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func payloadIntArray(payload map[string]any, key string) []int {
	if payload == nil {
		return nil
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []int:
		return append([]int(nil), typed...)
	case []any:
		items := make([]int, 0, len(typed))
		for _, item := range typed {
			if parsed, ok := payloadInt(item); ok {
				items = append(items, parsed)
			}
		}
		return items
	case string:
		parts := strings.FieldsFunc(typed, func(r rune) bool {
			return r == ',' || r == ';' || r == '|'
		})
		items := make([]int, 0, len(parts))
		for _, part := range parts {
			if parsed, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				items = append(items, parsed)
			}
		}
		return items
	default:
		if parsed, ok := payloadInt(value); ok {
			return []int{parsed}
		}
		return nil
	}
}

func payloadInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := strconv.Atoi(typed.String())
		return parsed, err == nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		return 0, false
	}
}
