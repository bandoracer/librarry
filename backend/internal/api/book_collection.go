package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type bookCollectionService interface {
	BookCollection(context.Context, wanted.BookCollectionQuery) (wanted.BookCollection, error)
}

func (h *handler) libraryBookCollection(w http.ResponseWriter, r *http.Request) {
	s, ok := h.deps.Wanted.(bookCollectionService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "book collection is unavailable"})
		return
	}
	params := r.URL.Query()
	for name, values := range params {
		switch name {
		case "q", "format", "monitor", "state", "sort", "cursor", "limit":
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unknown book collection filter"})
			return
		}
		if len(values) != 1 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "book collection filters must have one value"})
			return
		}
	}
	q := wanted.BookCollectionQuery{Search: params.Get("q"), Format: params.Get("format"), Monitor: params.Get("monitor"), State: params.Get("state"), Sort: params.Get("sort"), Cursor: params.Get("cursor"), Limit: 100}
	if params.Has("limit") {
		n, err := strconv.Atoi(params.Get("limit"))
		if err != nil || n < 1 || n > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "limit must be between 1 and 100"})
			return
		}
		q.Limit = n
	}
	page, err := s.BookCollection(r.Context(), q)
	if errors.Is(err, wanted.ErrBookPage) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid book filters or cursor; refresh the collection"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "book collection could not be loaded"})
		return
	}
	if page.Books == nil {
		page.Books = []wanted.WantedItem{}
	}
	writeJSON(w, http.StatusOK, page)
}
