package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type bookMatchesService interface {
	MatchBooks(context.Context, []wanted.BookMatchCandidate) (wanted.BookMatches, error)
}

func (h *handler) bookMatches(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Wanted.(bookMatchesService)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "Library identity check is unavailable"})
		return
	}
	var request struct {
		Candidates []wanted.BookMatchCandidate `json:"candidates"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.Candidates == nil {
		writeJSON(w, 400, map[string]any{"error": "Invalid library identity request"})
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		writeJSON(w, 400, map[string]any{"error": "Expected one library identity request"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := service.MatchBooks(ctx, request.Candidates)
	if err != nil {
		code, message := 503, "Library identity check failed; retry before adding a book"
		if errors.Is(err, wanted.ErrBookMatches) {
			code = 400
			message = err.Error()
		}
		writeJSON(w, code, map[string]any{"error": message})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, result)
}
