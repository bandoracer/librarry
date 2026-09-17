package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

var errCompatBookMissing = errors.New("book not found in the active library")
var errCompatBookAmbiguous = errors.New("book identity is ambiguous; use its librarryId")

// UUID identity wins first, then the numeric ID emitted by this API. Legacy
// work/edition/source aliases are accepted only when they identify one book.
// Titles and hashes of aliases are never mutation identities.
func resolveCompatBooks(items []wanted.WantedItem, ids []string) ([]wanted.WantedItem, error) {
	primary, numeric, aliases := map[string][]wanted.WantedItem{}, map[string][]wanted.WantedItem{}, map[string][]wanted.WantedItem{}
	for _, item := range items {
		if !compatWantedItemVisible(item) {
			continue
		}
		primary[strings.ToLower(item.ID)] = append(primary[strings.ToLower(item.ID)], item)
		n := strconv.Itoa(stableInt(item.ID))
		numeric[n] = append(numeric[n], item)
		seenAlias := map[string]bool{}
		for _, alias := range []string{item.WorkID, item.EditionID, item.SourceKey} {
			if alias != "" && !seenAlias[alias] {
				seenAlias[alias] = true
				aliases[alias] = append(aliases[alias], item)
			}
		}
	}
	result := []wanted.WantedItem{}
	seen := map[string]bool{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		matches := primary[strings.ToLower(id)]
		if len(matches) == 0 {
			matches = numeric[id]
		}
		if len(matches) == 0 {
			matches = aliases[id]
		}

		if len(matches) == 0 {
			return nil, fmt.Errorf("%w: %s", errCompatBookMissing, id)
		}
		if len(matches) != 1 {
			return nil, fmt.Errorf("%w: %s", errCompatBookAmbiguous, id)
		}
		if !seen[matches[0].ID] {
			seen[matches[0].ID] = true
			result = append(result, matches[0])
		}
	}
	return result, nil
}

func writeCompatBookError(w http.ResponseWriter, err error) {
	code, message := 503, "Book records could not be loaded or saved; refresh before retrying"
	switch {
	case errors.Is(err, errCompatBookMissing):
		code = 404
		message = err.Error()
	case errors.Is(err, errCompatBookAmbiguous), errors.Is(err, wanted.ErrCompatibilityBookSelection):
		code = 409
		message = err.Error()
	case errors.Is(err, wanted.ErrCompatibilityBookRequest):
		code = 400
		message = err.Error()
	}
	writeJSON(w, code, map[string]any{"error": message})
}

func (h *handler) compatSelectBooks(ctx context.Context, ids []string) ([]wanted.WantedItem, error) {
	if h.deps.Wanted == nil {
		return nil, errCompatServiceUnavailable
	}
	if len(ids) == 0 || len(ids) > 500 {
		return nil, wanted.ErrCompatibilityBookRequest
	}
	for _, id := range ids {
		if len(id) > 512 {
			return nil, wanted.ErrCompatibilityBookRequest
		}
	}
	items, err := h.deps.Wanted.CompatibilityBooks(ctx)
	if err != nil {
		return nil, err
	}
	return resolveCompatBooks(items, ids)
}

func (h *handler) compatMutateBooks(w http.ResponseWriter, r *http.Request, ids []string, payload map[string]any, remove bool) ([]wanted.WantedItem, bool) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	books, err := h.compatSelectBooks(ctx, ids)
	if err != nil {
		writeCompatBookError(w, err)
		return nil, false
	}
	request := wanted.CompatibilityBookMutation{Delete: remove}
	for _, book := range books {
		update := h.compatWantedUpdateRequest(ctx, payload)
		updateWantedTagsFromPayload(&update, book.Tags, payload)
		request.Books = append(request.Books, wanted.CompatibilityBookEdit{ID: book.ID, UpdatedAt: book.UpdatedAt, Update: update})
	}
	items, err := h.deps.Wanted.ApplyCompatibilityBooks(ctx, request)
	if err != nil {
		writeCompatBookError(w, err)
		return nil, false
	}
	return items, true
}
