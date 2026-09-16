package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/bandoracer/librarry/backend/internal/library"
)

type bookRenameService interface {
	PreviewBookRename(context.Context, string) (library.BookRenamePreview, error)
	RenameBook(context.Context, string, string) (library.ImportOutcome, error)
}

func (h *handler) previewBookRename(w http.ResponseWriter, r *http.Request) {
	h.bookRename(w, r, false)
}
func (h *handler) applyBookRename(w http.ResponseWriter, r *http.Request) { h.bookRename(w, r, true) }
func (h *handler) bookRename(w http.ResponseWriter, r *http.Request, apply bool) {
	service, ok := h.deps.Library.(bookRenameService)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "book folder rename is unavailable"})
		return
	}
	defer r.Body.Close()
	var request struct {
		Revision string `json:"revision"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid book rename request"})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(w, 400, map[string]any{"error": "expected one book rename request"})
		return
	}
	if apply {
		if request.Revision == "" {
			writeJSON(w, 400, map[string]any{"error": "preview the complete book folder before applying"})
			return
		}
		result, err := service.RenameBook(r.Context(), r.PathValue("id"), request.Revision)
		if err != nil {
			writeJSON(w, 409, map[string]any{"error": err.Error(), "outcome": result})
			return
		}
		writeJSON(w, 200, result)
		return
	}
	preview, err := service.PreviewBookRename(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	if preview.Files == nil {
		preview.Files = []library.ImportOperationFile{}
	}
	writeJSON(w, 200, preview)
}
