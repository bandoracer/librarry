package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type authorDetailService interface {
	AuthorDetail(context.Context, string, string, int) (wanted.AuthorDetail, error)
}

func (h *handler) libraryAuthorDetail(w http.ResponseWriter, r *http.Request) {
	s, ok := h.deps.Wanted.(authorDetailService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "author lookup is unavailable"})
		return
	}
	limit := 100
	if value := r.URL.Query().Get("limit"); value != "" {
		var err error
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "limit must be between 1 and 100"})
			return
		}
	}
	detail, err := s.AuthorDetail(r.Context(), r.PathValue("key"), r.URL.Query().Get("cursor"), limit)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "author not found"})
		return
	}
	if errors.Is(err, wanted.ErrAuthorPage) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid author page"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "author could not be loaded"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}
