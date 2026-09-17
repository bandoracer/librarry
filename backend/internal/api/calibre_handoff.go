package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/jackc/pgx/v5/pgtype"
)

type calibreHandoffService interface {
	RetryCalibreHandoff(context.Context, string) (library.ImportOutcome, error)
	ResolveCalibreHandoff(context.Context, string, library.CalibreHandoffResolution) (library.ImportOutcome, error)
}

func (h *handler) retryCalibreHandoff(w http.ResponseWriter, r *http.Request) {
	h.calibreHandoff(w, r, false)
}
func (h *handler) resolveCalibreHandoff(w http.ResponseWriter, r *http.Request) {
	h.calibreHandoff(w, r, true)
}
func (h *handler) calibreHandoff(w http.ResponseWriter, r *http.Request, resolve bool) {
	service, ok := h.deps.Library.(calibreHandoffService)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "Calibre recovery is unavailable"})
		return
	}
	id := r.PathValue("id")
	var parsed pgtype.UUID
	if err := parsed.Scan(id); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid Calibre handoff ID"})
		return
	}
	var result library.ImportOutcome
	var err error
	if resolve {
		defer r.Body.Close()
		var request *library.CalibreHandoffResolution
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if e := decoder.Decode(&request); e != nil || request == nil {
			writeJSON(w, 400, map[string]any{"error": "invalid Calibre recovery decision"})
			return
		}
		if e := decoder.Decode(new(any)); e != io.EOF {
			writeJSON(w, 400, map[string]any{"error": "expected one Calibre recovery decision"})
			return
		}
		if !request.Confirm {
			writeJSON(w, 400, map[string]any{"error": "confirm inspection of the original Calibre library"})
			return
		}
		result, err = service.ResolveCalibreHandoff(r.Context(), id, *request)
	} else {
		result, err = service.RetryCalibreHandoff(r.Context(), id)
	}
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error(), "outcome": result})
		return
	}
	writeJSON(w, 200, result)
}
