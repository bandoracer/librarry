package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/metadata"
)

// Exercise the real adapter's missing-credential path through the public route.
// Series gets a bounded larger page; arbitrary client limits cannot expand it.
func TestMetadataSeriesRouteReportsMissingProviderAndBoundedPage(t *testing.T) {
	router := NewRouter(Dependencies{Logger: slog.Default(), Config: config.Config{WebOrigin: "*"}, Metadata: metadata.NewService([]metadata.Provider{metadata.NewHardcoverProvider(nil, "")})})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/search?query=Murderbot&type=series&limit=999999", nil))
	var out metadata.SearchOutcome
	if res.Code != http.StatusOK {
		t.Fatal(res.Code, res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Query.Limit != 50 || out.Results == nil || len(out.Results) != 0 || len(out.ProviderErrors) != 1 || out.ProviderErrors[0].Provider != "Hardcover" {
		t.Fatal(out)
	}
}
