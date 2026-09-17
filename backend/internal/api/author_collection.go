package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type authorCollectionService interface {
	AuthorCollection(context.Context, wanted.AuthorCollectionQuery) (wanted.AuthorCollection, error)
}

func (h *handler) libraryAuthorCollection(w http.ResponseWriter, r *http.Request) {
	s, ok := h.deps.Wanted.(authorCollectionService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "author collection is unavailable"})
		return
	}
	params := r.URL.Query()
	for name, values := range params {
		switch name {
		case "q", "format", "status", "cursor", "limit":
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unknown author collection filter"})
			return
		}
		if len(values) != 1 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "author collection filters must have one value"})
			return
		}
	}
	q := wanted.AuthorCollectionQuery{Search: params.Get("q"), Format: params.Get("format"), Status: params.Get("status"), Cursor: params.Get("cursor"), Limit: 100}
	if params.Has("limit") {
		n, err := strconv.Atoi(params.Get("limit"))
		if err != nil || n < 1 || n > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "limit must be between 1 and 100"})
			return
		}
		q.Limit = n
	}
	page, err := s.AuthorCollection(r.Context(), q)
	if errors.Is(err, wanted.ErrBookPage) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid author filters or cursor; refresh the collection"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "author collection could not be loaded"})
		return
	}
	if page.Authors == nil {
		page.Authors = []wanted.AuthorCollectionItem{}
	}
	writeJSON(w, http.StatusOK, page)
}
