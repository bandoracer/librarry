package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type authorReviewCollectionService interface {
	AuthorReviewCollection(context.Context, wanted.AuthorMetadataReviewQuery) (wanted.AuthorReviewCollection, error)
}

func (h *handler) authorMetadataReviews(w http.ResponseWriter, r *http.Request) {
	s, ok := h.deps.Wanted.(authorReviewCollectionService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "author review collection is unavailable"})
		return
	}
	params := r.URL.Query()
	for name, values := range params {
		switch name {
		case "q", "format", "status", "cursor", "limit":
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unknown author review collection filter"})
			return
		}
		if len(values) != 1 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "author review collection filters must have one value"})
			return
		}
	}
	q := wanted.AuthorMetadataReviewQuery{Search: params.Get("q"), Format: params.Get("format"), Status: params.Get("status"), Cursor: params.Get("cursor"), Limit: 100}
	if params.Has("limit") {
		n, err := strconv.Atoi(params.Get("limit"))
		if err != nil || n < 1 || n > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "limit must be between 1 and 100"})
			return
		}
		q.Limit = n
	}
	page, err := s.AuthorReviewCollection(r.Context(), q)
	if errors.Is(err, wanted.ErrBookPage) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid author review filters or cursor; refresh the collection"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "author review collection could not be loaded"})
		return
	}
	if page.Reviews == nil {
		page.Reviews = []wanted.AuthorMetadataReview{}
	}
	writeJSON(w, http.StatusOK, page)
}
