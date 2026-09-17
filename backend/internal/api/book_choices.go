package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type bookChoicesService interface {
	BookChoices(context.Context, wanted.BookChoicesQuery) (wanted.BookChoices, error)
}

func (h *handler) bookChoices(w http.ResponseWriter, r *http.Request) {
	s, ok := h.deps.Wanted.(bookChoicesService)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "book choices are unavailable"})
		return
	}
	params := r.URL.Query()
	for k, v := range params {
		switch k {
		case "q", "format", "selectedId", "cursor", "limit":
		default:
			writeJSON(w, 400, map[string]any{"error": "unknown book choice filter"})
			return
		}
		if len(v) != 1 {
			writeJSON(w, 400, map[string]any{"error": "book choice filters must have one value"})
			return
		}
	}
	q := wanted.BookChoicesQuery{Search: params.Get("q"), Format: params.Get("format"), SelectedID: params.Get("selectedId"), Cursor: params.Get("cursor")}
	if params.Has("limit") {
		n, e := strconv.Atoi(params.Get("limit"))
		if e != nil || n < 1 || n > 100 {
			writeJSON(w, 400, map[string]any{"error": "limit must be 1–100"})
			return
		}
		q.Limit = n
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := s.BookChoices(ctx, q)
	if err != nil {
		code := 503
		message := "Book choices could not be loaded"
		if errors.Is(err, wanted.ErrBookChoices) {
			code = 400
			message = err.Error()
		}
		writeJSON(w, code, map[string]any{"error": message})
		return
	}
	if page.Books == nil {
		page.Books = []wanted.BookChoice{}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, page)
}
