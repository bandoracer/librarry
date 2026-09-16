package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestCalibreRecoveryRejectsMalformedDecisions(t *testing.T) {
	db := testdb.Open(t)
	service := library.NewService(library.NewStore(db), library.Config{}, wanted.NewStore(db), nil)
	router := NewRouter(Dependencies{Logger: slog.Default(), Config: config.Config{WebOrigin: "*"}, Library: service})
	endpoint := "/api/v1/library/calibre-handoffs/00000000-0000-0000-0000-000000000046/resolve"
	for _, body := range []string{"", "null", "{}", `{"action":"attach-book","bookId":42}`, `{"action":"attach-book","confirm":true,"path":"/arbitrary"}`, `{"confirm":true} {"confirm":true}`} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(body)))
		if res.Code != 400 {
			t.Fatalf("%q: %d %s", body, res.Code, res.Body.String())
		}
	}
}
