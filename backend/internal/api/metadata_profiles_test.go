package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type fakeMetadataProfileWanted struct {
	fakeWanted
	inUseID string
	deleted []string
}

func (f *fakeMetadataProfileWanted) ListMetadataProfiles(context.Context) ([]wanted.MetadataProfile, error) {
	return []wanted.MetadataProfile{{
		ID:               "profile-1",
		Name:             "Standard",
		AllowedLanguages: []string{"en"},
		MustNotContain:   []string{"boxed set"},
		SkipMissingISBN:  true,
		MinPages:         50,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}}, nil
}

func (f *fakeMetadataProfileWanted) CreateMetadataProfile(_ context.Context, profile wanted.MetadataProfile) (wanted.MetadataProfile, error) {
	profile.ID = "profile-2"
	return profile, nil
}

func (f *fakeMetadataProfileWanted) UpdateMetadataProfile(_ context.Context, id string, profile wanted.MetadataProfile) (wanted.MetadataProfile, error) {
	profile.ID = id
	return profile, nil
}

func (f *fakeMetadataProfileWanted) DeleteMetadataProfile(_ context.Context, id string) error {
	if id == f.inUseID {
		return wanted.ErrMetadataProfileInUse
	}
	f.deleted = append(f.deleted, id)
	return nil
}

func TestMetadataProfileEndpoints(t *testing.T) {
	wantedClient := &fakeMetadataProfileWanted{inUseID: "profile-1"}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   wantedClient,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metadata-profiles", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	for _, fragment := range []string{
		`"profiles":[`, `"id":"profile-1"`, `"name":"Standard"`, `"allowedLanguages":["en"]`,
		`"mustNotContain":["boxed set"]`, `"skipMissingIsbn":true`, `"minPages":50`,
	} {
		if !strings.Contains(res.Body.String(), fragment) {
			t.Fatalf("expected %s in metadata profiles payload, got %s", fragment, res.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/metadata-profiles", strings.NewReader(`{"name":"English only","allowedLanguages":["en"]}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"profile":{`) ||
		!strings.Contains(res.Body.String(), `"id":"profile-2"`) {
		t.Fatalf("expected created profile envelope, got %d: %s", res.Code, res.Body.String())
	}

	// Validation: a profile without a name is rejected before the service.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/metadata-profiles", strings.NewReader(`{"minPages":10}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "name is required") {
		t.Fatalf("expected 400 name-required, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/metadata-profiles", strings.NewReader(`{invalid`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid payload, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/metadata-profiles/profile-1", strings.NewReader(`{"name":"Renamed","minPages":100}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"name":"Renamed"`) {
		t.Fatalf("expected updated profile envelope, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/metadata-profiles/profile-1", strings.NewReader(`{"name":"  "}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for blank name on update, got %d: %s", res.Code, res.Body.String())
	}

	// Deleting a referenced profile conflicts (409) with the documented error.
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/metadata-profiles/profile-1", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusConflict || !strings.Contains(res.Body.String(), "metadata profile is in use") {
		t.Fatalf("expected 409 in-use conflict, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/metadata-profiles/profile-2", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != "{}" {
		t.Fatalf("expected empty 200 body, got %d: %s", res.Code, res.Body.String())
	}
	if len(wantedClient.deleted) != 1 || wantedClient.deleted[0] != "profile-2" {
		t.Fatalf("expected delete dispatched to service, got %+v", wantedClient.deleted)
	}
}

func TestMetadataProfileEndpointsUnavailableWithoutService(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metadata-profiles", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without metadata profile service, got %d: %s", res.Code, res.Body.String())
	}
}

type rootFolderRejectingWanted struct {
	fakeWanted
	request wanted.CreateRequest
}

func (f *rootFolderRejectingWanted) Create(_ context.Context, request wanted.CreateRequest) (wanted.WantedItem, error) {
	f.request = request
	if request.RootFolderID == "missing" {
		return wanted.WantedItem{}, errNotFoundRootFolder
	}
	return wanted.WantedItem{}, errRootFolderFormatMismatch
}

var (
	errNotFoundRootFolder       = errorString("root folder not found")
	errRootFolderFormatMismatch = errorString(`root folder media format "audiobook" is not compatible with wanted format "ebook"`)
)

type errorString string

func (e errorString) Error() string { return string(e) }

func TestCreateWantedRootFolderValidationMapsTo400(t *testing.T) {
	wantedClient := &rootFolderRejectingWanted{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   wantedClient,
	})

	body := `{"result":{"provider":"openlibrary","work":{"title":"Project Hail Mary"}},"format":"ebook","rootFolderId":"missing"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wanted", strings.NewReader(body))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "root folder not found") {
		t.Fatalf("expected 400 for unknown root folder, got %d: %s", res.Code, res.Body.String())
	}
	if wantedClient.request.RootFolderID != "missing" {
		t.Fatalf("expected rootFolderId decoded onto create request, got %+v", wantedClient.request)
	}

	body = `{"result":{"provider":"openlibrary","work":{"title":"Project Hail Mary"}},"format":"ebook","rootFolderId":"audio-root"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/wanted", strings.NewReader(body))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "not compatible") {
		t.Fatalf("expected 400 for format mismatch, got %d: %s", res.Code, res.Body.String())
	}
}
