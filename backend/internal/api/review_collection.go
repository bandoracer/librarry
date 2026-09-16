package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type metadataReviewCollectionService interface {
	MetadataReviewCollection(context.Context, wanted.MetadataReviewQuery) (wanted.MetadataReviewQueue, error)
}

func (h *handler) metadataReviewCollection(w http.ResponseWriter, r *http.Request, s metadataReviewCollectionService) {
	params := r.URL.Query()
	for name, values := range params {
		if (name != "q" && name != "format" && name != "cursor" && name != "limit") || len(values) != 1 {
			writeJSON(w, 400, map[string]any{"error": "invalid metadata review filter"})
			return
		}
	}
	q := wanted.MetadataReviewQuery{Search: params.Get("q"), Format: params.Get("format"), Cursor: params.Get("cursor"), Limit: 100}
	if params.Has("limit") {
		n, err := strconv.Atoi(params.Get("limit"))
		if err != nil || n < 1 || n > 100 {
			writeJSON(w, 400, map[string]any{"error": "limit must be between 1 and 100"})
			return
		}
		q.Limit = n
	}
	page, err := s.MetadataReviewCollection(r.Context(), q)
	if errors.Is(err, wanted.ErrBookPage) {
		writeJSON(w, 400, map[string]any{"error": "invalid metadata review filters or cursor; refresh the collection"})
		return
	}
	if err != nil {
		writeJSON(w, 503, map[string]any{"error": "metadata review could not be loaded"})
		return
	}
	if page.Items == nil {
		page.Items = []wanted.MetadataReviewItem{}
	}
	writeJSON(w, 200, page)
}
