package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type reviewCollectionFixture struct {
	fakeMetadataWanted
	query wanted.MetadataReviewQuery
}

func (f *reviewCollectionFixture) MetadataReviewCollection(_ context.Context, q wanted.MetadataReviewQuery) (wanted.MetadataReviewQueue, error) {
	f.query = q
	return wanted.MetadataReviewQueue{Total: 501, Filtered: 201, ConflictCount: 800, NextCursor: "next"}, nil
}

func TestMetadataReviewCollectionAPI(t *testing.T) {
	fixture := &reviewCollectionFixture{}
	router := NewRouter(Dependencies{Wanted: fixture})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/wanted/metadata/review?q=old&format=ebook&limit=25&cursor=next", nil))
	if res.Code != 200 || fixture.query.Search != "old" || fixture.query.Format != "ebook" || fixture.query.Limit != 25 || fixture.query.Cursor != "next" || !strings.Contains(res.Body.String(), `"items":[]`) || !strings.Contains(res.Body.String(), `"total":501`) {
		t.Fatal(res.Code, res.Body.String(), fixture.query)
	}
}
func TestMetadataReviewAPIRejectsInvalidQueriesAndBodies(t *testing.T) {
	router := NewRouter(Dependencies{Wanted: wanted.NewService(nil, nil)})
	for _, query := range []string{"limit=0", "limit=101", "limit=no", "format=video", "format=ebook&format=audio", "cursor=bad", "unknown=true"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/wanted/metadata/review?"+query, nil))
		if res.Code != 400 {
			t.Fatal(query, res.Code, res.Body.String())
		}
	}
	for _, body := range []string{"", `null`, `{}`, `{"all":true,"wantedIds":["bad"]}`, `{"wantedIds":["bad"]}`, `{"all":true,"unknown":1}`, `{"all":true} {}`, `{"all":"true"}`} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/wanted/metadata/review/confirm-canonical", strings.NewReader(body)))
		if res.Code != 400 {
			t.Fatal(body, res.Code, res.Body.String())
		}
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/wanted/metadata/review", nil))
	if res.Code != 503 {
		t.Fatal(res.Code, res.Body.String())
	}
}
