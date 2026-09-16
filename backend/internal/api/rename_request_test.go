package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/library"
)

func TestRenameBodyRejectsAmbiguousRequests(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `[]`, `{"ids":["file"],"unknown":true}`, `{"ids":["file"]}{}`, `{"ids":["file"]} true`, `{"ids":["` + strings.Repeat("x", 1<<20) + `"]}`} {
		t.Run(body[:min(len(body), 70)], func(t *testing.T) {
			request := httptest.NewRequest("POST", "/api/v1/library/files/rename", strings.NewReader(body))
			response := httptest.NewRecorder()
			var parsed library.RenameFilesRequest
			if decodeRenameRequest(response, request, &parsed) || response.Code != 400 {
				t.Fatal(response.Code, response.Body.String())
			}
		})
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/v1/library/files/rename", strings.NewReader(`{"ids":["file"],"revisions":{"file":"proof"}}`))
	var parsed library.RenameFilesRequest
	if !decodeRenameRequest(response, request, &parsed) || parsed.Revisions["file"] != "proof" {
		t.Fatal(response.Body.String(), parsed)
	}
}
