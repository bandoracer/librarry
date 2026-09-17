package api

import (
	"context"
	"errors"
	"github.com/bandoracer/librarry/backend/internal/library"
	"net/http"
	"strconv"
)

type fileCollectionService interface {
	FileCollection(context.Context, library.FileCollectionQuery) (library.FileCollection, error)
}

func (h *handler) libraryFileCollection(w http.ResponseWriter, r *http.Request) {
	s, ok := h.deps.Library.(fileCollectionService)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "file collection is unavailable"})
		return
	}
	params := r.URL.Query()
	for key, values := range params {
		switch key {
		case "q", "wantedId", "format", "presence", "sort", "cursor", "limit":
		default:
			writeJSON(w, 400, map[string]any{"error": "unknown file collection filter"})
			return
		}
		if len(values) != 1 {
			writeJSON(w, 400, map[string]any{"error": "file collection filters must have one value"})
			return
		}
	}
	q := library.FileCollectionQuery{Search: params.Get("q"), WantedID: params.Get("wantedId"), Format: params.Get("format"), Presence: params.Get("presence"), Sort: params.Get("sort"), Cursor: params.Get("cursor"), Limit: 100}
	if params.Has("limit") {
		n, err := strconv.Atoi(params.Get("limit"))
		if err != nil || n < 1 || n > 100 {
			writeJSON(w, 400, map[string]any{"error": "limit must be between 1 and 100"})
			return
		}
		q.Limit = n
	}
	page, err := s.FileCollection(r.Context(), q)
	if errors.Is(err, library.ErrFilePage) {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, 503, map[string]any{"error": "file collection could not be loaded"})
		return
	}
	if page.Files == nil {
		page.Files = []library.FileCollectionItem{}
	}
	if page.Counts == nil {
		page.Counts = map[string]int{}
	}
	writeJSON(w, 200, page)
}
