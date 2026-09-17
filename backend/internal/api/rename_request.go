package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/bandoracer/librarry/backend/internal/library"
)

func decodeRenameRequest(w http.ResponseWriter, r *http.Request, request *library.RenameFilesRequest) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid library rename payload"})
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "expected one rename request"})
		return false
	}
	if len(request.IDs)+len(request.Paths) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "select at least one file"})
		return false
	}
	return true
}
