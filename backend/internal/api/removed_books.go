package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type removedBooksService interface {
	RemovedBooks(context.Context, wanted.RemovedBooksQuery) (wanted.RemovedBooks, error)
	RestoreBook(context.Context, string, wanted.RestoreBookRequest) (wanted.WantedItem, error)
}

func (h *handler) removedBooks(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Wanted.(removedBooksService)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "removed books are unavailable"})
		return
	}
	params := r.URL.Query()
	for k, v := range params {
		switch k {
		case "q", "format", "status", "cursor", "limit":
		default:
			writeJSON(w, 400, map[string]any{"error": "unknown removed book filter"})
			return
		}
		if len(v) != 1 {
			writeJSON(w, 400, map[string]any{"error": "removed book filters must have one value"})
			return
		}
	}
	q := wanted.RemovedBooksQuery{Search: params.Get("q"), Format: params.Get("format"), Status: params.Get("status"), Cursor: params.Get("cursor")}
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
	page, err := service.RemovedBooks(ctx, q)
	if err != nil {
		code := 503
		message := "Removed books could not be loaded"
		if errors.Is(err, wanted.ErrRemovedBooks) {
			code = 400
			message = err.Error()
		}
		writeJSON(w, code, map[string]any{"error": message})
		return
	}
	if page.Books == nil {
		page.Books = []wanted.WantedItem{}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, page)
}
func (h *handler) restoreBook(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Wanted.(removedBooksService)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "book restore is unavailable"})
		return
	}
	var request wanted.RestoreBookRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid restore request"})
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		writeJSON(w, 400, map[string]any{"error": "restore request must contain one JSON object"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	item, err := service.RestoreBook(ctx, r.PathValue("id"), request)
	if err != nil {
		code := 503
		message := "Book could not be restored; refresh to check its current state"
		switch {
		case errors.Is(err, wanted.ErrRestoreBook):
			code = 400
			message = err.Error()
		case errors.Is(err, wanted.ErrRestoreBookChanged):
			code = 409
			message = err.Error()
		case errors.Is(err, sql.ErrNoRows):
			code = 404
			message = "Book not found"
		}
		writeJSON(w, code, map[string]any{"error": message})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, item)
}
