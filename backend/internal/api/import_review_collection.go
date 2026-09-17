package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/bandoracer/librarry/backend/internal/library"
)

type importReviewCollectionService interface {
	ImportReviewCollection(context.Context, library.ImportReviewQuery) (library.ImportReviewCollection, error)
}

func (h *handler) importReviewCollection(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Library.(importReviewCollectionService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "import review collection is unavailable"})
		return
	}
	values := r.URL.Query()
	for key, values := range values {
		switch key {
		case "view", "status", "format", "kind", "q", "cursor", "limit":
		default:
			writeJSON(w, 400, map[string]any{"error": "unknown import review filter"})
			return
		}
		if len(values) != 1 {
			writeJSON(w, 400, map[string]any{"error": "import review filters must have one value"})
			return
		}
	}
	q := library.ImportReviewQuery{Status: values.Get("status"), Format: values.Get("format"), Kind: values.Get("kind"), Search: values.Get("q"), Cursor: values.Get("cursor")}
	if values.Has("limit") {
		n, err := strconv.Atoi(values.Get("limit"))
		if err != nil || n < 1 || n > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "limit must be 1–100"})
			return
		}
		q.Limit = n
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := service.ImportReviewCollection(ctx, q)
	if err != nil {
		status := http.StatusServiceUnavailable
		message := "Import review collection could not be loaded"
		if errors.Is(err, library.ErrImportReviewPage) {
			status = http.StatusBadRequest
			message = err.Error()
		}
		writeJSON(w, status, map[string]any{"error": message})
		return
	}
	if page.Reviews == nil {
		page.Reviews = []library.ImportReview{}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, page)
}
