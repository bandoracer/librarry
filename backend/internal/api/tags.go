package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/tags"
)

// Native tags API (M6.4). Tag identity is the label; the numeric id is the
// compat-style stable hash the UI and Readarr-compatible clients use.

func (h *handler) tagsStore(w http.ResponseWriter) (*tags.Store, bool) {
	if h.deps.Tags == nil || !h.deps.Tags.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "tags require database persistence"})
		return nil, false
	}
	return h.deps.Tags, true
}

func (h *handler) listTags(w http.ResponseWriter, r *http.Request) {
	store, ok := h.tagsStore(w)
	if !ok {
		return
	}
	tagRows, err := store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if tagRows == nil {
		tagRows = []tags.Tag{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": tagRows})
}

type tagPayload struct {
	Label string `json:"label"`
}

func (h *handler) createTag(w http.ResponseWriter, r *http.Request) {
	store, ok := h.tagsStore(w)
	if !ok {
		return
	}
	defer r.Body.Close()
	var payload tagPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Label) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "tag label is required"})
		return
	}
	tag, err := store.Create(r.Context(), payload.Label)
	if err != nil {
		writeJSON(w, tagErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"tag": tag})
}

func (h *handler) updateTag(w http.ResponseWriter, r *http.Request) {
	store, ok := h.tagsStore(w)
	if !ok {
		return
	}
	defer r.Body.Close()
	var payload tagPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Label) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "tag label is required"})
		return
	}
	tag, err := store.Rename(r.Context(), r.PathValue("id"), payload.Label)
	if err != nil {
		writeJSON(w, tagErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": tag})
}

func (h *handler) deleteTag(w http.ResponseWriter, r *http.Request) {
	store, ok := h.tagsStore(w)
	if !ok {
		return
	}
	if err := store.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeJSON(w, tagErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func tagErrorStatus(err error) int {
	switch {
	case errors.Is(err, tags.ErrTagNotFound):
		return http.StatusNotFound
	case errors.Is(err, tags.ErrTagExists):
		return http.StatusConflict
	default:
		return http.StatusBadGateway
	}
}

/* ------------------- compat integer-tag <-> label bridge ------------------- */

// decodeTagLabels accepts a JSON tags array of labels, legacy integer ids, or
// a mix of both, returning string labels (integers become decimal strings for
// resolveTagLabels to map).
func decodeTagLabels(raw json.RawMessage) ([]string, error) {
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	labels := make([]string, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			labels = append(labels, typed)
		case float64:
			labels = append(labels, strconv.Itoa(int(typed)))
		}
	}
	return labels, nil
}

// resolveTagLabels maps purely-numeric labels (legacy compat integer ids)
// back onto known labels when possible; everything else passes through.
func (h *handler) resolveTagLabels(ctx context.Context, labels []string) []string {
	needsLookup := false
	for _, label := range labels {
		if _, err := strconv.Atoi(strings.TrimSpace(label)); err == nil {
			needsLookup = true
			break
		}
	}
	if !needsLookup {
		return labels
	}
	known := h.compatKnownTagLabels(ctx)
	resolved := make([]string, 0, len(labels))
	for _, label := range labels {
		if numeric, err := strconv.Atoi(strings.TrimSpace(label)); err == nil {
			if mapped, ok := known[numeric]; ok {
				resolved = append(resolved, mapped)
				continue
			}
		}
		resolved = append(resolved, label)
	}
	return resolved
}

// compatTagLabels maps Readarr-style integer tag ids onto native labels. Known
// ids resolve through the native tags table and persisted compat tag
// resources; unknown ids fall back to their decimal string so nothing is
// silently dropped.
func (h *handler) compatTagLabels(ctx context.Context, ids []int) []string {
	ids = compactCompatTags(ids)
	if len(ids) == 0 {
		return nil
	}
	known := h.compatKnownTagLabels(ctx)
	labels := make([]string, 0, len(ids))
	for _, id := range ids {
		if label, ok := known[id]; ok {
			labels = append(labels, label)
			continue
		}
		labels = append(labels, strconv.Itoa(id))
	}
	return labels
}

// compatKnownTagLabels indexes every known tag label by the integer ids a
// compat client could reference it with.
func (h *handler) compatKnownTagLabels(ctx context.Context) map[int]string {
	known := map[int]string{}
	register := func(label string, extraIDs ...int) {
		label = tags.NormalizeLabel(label)
		if label == "" {
			return
		}
		known[tags.StableID(label)] = label
		for _, id := range extraIDs {
			if id > 0 {
				known[id] = label
			}
		}
	}
	register("librarry")
	if h.deps.Tags != nil && h.deps.Tags.Configured() {
		if tagRows, err := h.deps.Tags.List(ctx); err == nil {
			for _, tag := range tagRows {
				register(tag.Label, tag.ID)
			}
		}
	}
	if h.deps.Compat != nil {
		if resources, err := h.deps.Compat.ListResources(ctx, "tag"); err == nil {
			for _, resource := range resources {
				label := firstNonEmptyString(payloadString(resource.Payload, "label"), payloadString(resource.Payload, "name"), resource.Name)
				register(label, resource.CompatID)
			}
		}
	}
	return known
}

// compatTagIDs renders label tags as Readarr-style integer arrays. Legacy
// numeric labels pass through as their own value.
func compatTagIDs(labels []string) []int {
	if len(labels) == 0 {
		return []int{}
	}
	ids := make([]int, 0, len(labels))
	for _, label := range labels {
		label = tags.NormalizeLabel(label)
		if label == "" {
			continue
		}
		if numeric, err := strconv.Atoi(label); err == nil && numeric > 0 {
			ids = append(ids, numeric)
			continue
		}
		ids = append(ids, tags.StableID(label))
	}
	return compactCompatTags(ids)
}

// applyCompatTagLabelMode mirrors Readarr's applyTags editor modes on label
// tags: add, remove, none/noop, or replace (default).
func applyCompatTagLabelMode(current []string, requested []string, mode string) []string {
	current = normalizeTagLabels(current)
	requested = normalizeTagLabels(requested)
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "add":
		return normalizeTagLabels(append(append([]string{}, current...), requested...))
	case "remove":
		remove := make(map[string]bool, len(requested))
		for _, label := range requested {
			remove[label] = true
		}
		out := make([]string, 0, len(current))
		for _, label := range current {
			if !remove[label] {
				out = append(out, label)
			}
		}
		return out
	case "none", "noop":
		return current
	default:
		return requested
	}
}

func normalizeTagLabels(labels []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		label = tags.NormalizeLabel(label)
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	return out
}
