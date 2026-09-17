package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/bandoracer/librarry/backend/internal/kindle"
)

func (h *handler) kindleReady(w http.ResponseWriter) bool {
	if h.deps.Kindle.Available() {
		return true
	}
	writeJSON(w, 503, map[string]string{"error": "Kindle delivery requires database persistence"})
	return false
}
func (h *handler) kindleSettings(w http.ResponseWriter, r *http.Request) {
	if !h.kindleReady(w) {
		return
	}
	s, e := h.deps.Kindle.Settings(r.Context())
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Unable to read Kindle settings"})
		return
	}
	writeJSON(w, 200, s.Redacted())
}
func (h *handler) saveKindleSettings(w http.ResponseWriter, r *http.Request) {
	if !h.kindleReady(w) {
		return
	}
	var body struct {
		kindle.Settings
		ClearPassword bool `json:"clearPassword"`
	}
	if !kindleBody(w, r, &body) {
		return
	}
	s, e := h.deps.Kindle.SaveSettings(r.Context(), body.Settings, body.ClearPassword)
	if e != nil {
		status, message := 500, "Unable to save Kindle settings"
		var validation *kindle.InvalidError
		if errors.As(e, &validation) {
			status = 400
			message = e.Error()
		}
		writeJSON(w, status, map[string]string{"error": message})
		return
	}
	writeJSON(w, 200, s.Redacted())
}
func (h *handler) kindleHistory(w http.ResponseWriter, r *http.Request) {
	if !h.kindleReady(w) {
		return
	}
	list, e := h.deps.Kindle.History(r.Context(), r.URL.Query().Get("wantedId"))
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Unable to read Kindle delivery history"})
		return
	}
	writeJSON(w, 200, list)
}
func (h *handler) sendKindleTest(w http.ResponseWriter, r *http.Request) { h.sendKindle(w, r, true) }
func (h *handler) sendKindleBook(w http.ResponseWriter, r *http.Request) { h.sendKindle(w, r, false) }
func (h *handler) sendKindle(w http.ResponseWriter, r *http.Request, test bool) {
	if !h.kindleReady(w) {
		return
	}
	var body struct {
		WantedID  string `json:"wantedId"`
		FileID    string `json:"fileId"`
		RequestID string `json:"requestId"`
	}
	if !kindleBody(w, r, &body) {
		return
	}
	if (test && (body.FileID != "" || body.WantedID != "")) || (!test && (body.FileID == "" || body.WantedID == "")) {
		writeJSON(w, 400, map[string]string{"error": "invalid book/file selection"})
		return
	}
	d, e := h.deps.Kindle.Send(r.Context(), body.WantedID, body.FileID, body.RequestID)
	if e != nil {
		code := 500
		if errors.Is(e, kindle.ErrConflict) {
			code = 409
		}
		// Database and filesystem/provider internals never escape this boundary.
		message := "Unable to prepare or record delivery; check settings and history before retrying"
		var validation *kindle.InvalidError
		if errors.As(e, &validation) {
			code = 400
			message = e.Error()
		}
		if errors.Is(e, kindle.ErrConflict) {
			message = e.Error()
		}

		writeJSON(w, code, map[string]string{"error": message})
		return
	}
	writeJSON(w, 200, d)
}
func kindleBody(w http.ResponseWriter, r *http.Request, v any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Content-Type must be application/json"})
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	dec.DisallowUnknownFields()
	if dec.Decode(v) != nil || dec.Decode(new(any)) != io.EOF {
		writeJSON(w, 400, map[string]string{"error": "invalid Kindle request"})
		return false
	}
	return true
}
