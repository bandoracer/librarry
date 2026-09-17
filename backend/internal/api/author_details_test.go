package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/testdb"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestAuthorDetailRoutesAndFailureBoundaries(t *testing.T) {
	db := testdb.Open(t)
	store := wanted.NewStore(db)
	ctx := context.Background()
	sub, err := store.UpsertAuthorSubscription(ctx, wanted.AuthorSubscription{AuthorName: "Empty Author", Provider: "Hardcover", ProviderKey: "hardcover-author:7", Format: "ebook", MonitorNewItems: true})
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{Wanted: wanted.NewService(store, nil)})
	path := "/api/v1/library/authors/" + sub.ID
	for _, tc := range []struct {
		suffix string
		status int
	}{{"", 200}, {"?limit=101", 400}, {"?limit=0", 400}, {"?limit=no", 400}, {"?cursor=bad", 400}} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest("GET", path+tc.suffix, nil))
		if res.Code != tc.status {
			t.Fatal(tc, res.Code, res.Body.String())
		}
		if tc.status == 200 {
			var payload map[string]json.RawMessage
			if json.Unmarshal(res.Body.Bytes(), &payload) != nil {
				t.Fatal(res.Body.String())
			}
			for _, key := range []string{"books", "choices"} {
				if string(payload[key]) != "[]" {
					t.Fatal(key, string(payload[key]))
				}
			}
		}
	}
	if err := store.DeleteAuthorSubscription(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest("GET", path, nil))
	if res.Code != 404 {
		t.Fatal(res.Code, res.Body.String())
	}
	res = httptest.NewRecorder()
	NewRouter(Dependencies{}).ServeHTTP(res, httptest.NewRequest("GET", path, nil))
	if res.Code != 503 {
		t.Fatal(res.Code, res.Body.String())
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	res = httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest("GET", path, nil))
	if res.Code != 503 {
		t.Fatal(res.Code, res.Body.String())
	}
}
