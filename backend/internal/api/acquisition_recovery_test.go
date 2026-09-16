package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

type fakeAcquisitionRecovery struct {
	fakeAcquire
	releases   int
	checks     int
	downloadID string
}

func (f *fakeAcquisitionRecovery) AcquisitionRecovery(context.Context) ([]acquisition.AcquisitionIntent, error) {
	return nil, nil
}
func (f *fakeAcquisitionRecovery) ReconcileAcquisition(_ context.Context, _, id string) (acquisition.DownloadStatus, error) {
	f.checks++
	f.downloadID = id
	return acquisition.DownloadStatus{}, acquisition.ErrAcquisitionUncertain
}
func (f *fakeAcquisitionRecovery) ReleaseAcquisition(context.Context, string) error {
	f.releases++
	return nil
}
func TestAcquisitionRecoveryRequiresExplicitOperatorDecisions(t *testing.T) {
	fake := &fakeAcquisitionRecovery{}
	router := NewRouter(Dependencies{Acquire: fake})
	for _, body := range []string{`{"action":"release"}`, `{"action":"attach","downloadId":"client-id"}`, `{"action":"attach","confirmed":true}`, `{"action":"unknown"}`} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/acquisition-recovery/fixture", strings.NewReader(body)))
		if res.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", body, res.Code, res.Body.String())
		}
	}
	if fake.releases != 0 || fake.checks != 0 {
		t.Fatal("unconfirmed decision executed")
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/acquisition-recovery/fixture", strings.NewReader(`{"action":"release","confirmed":true}`)))
	if res.Code != http.StatusOK || fake.releases != 1 {
		t.Fatal(res.Code, res.Body.String(), fake.releases)
	}
	res = httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/acquisition-recovery/fixture", strings.NewReader(`{"action":"attach","downloadId":"exact-id","confirmed":true}`)))
	if res.Code != http.StatusConflict || fake.downloadID != "exact-id" {
		t.Fatal(res.Code, res.Body.String(), fake.downloadID)
	}
	res = httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/acquisition-recovery", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"intents":[]`) {
		t.Fatal(res.Code, res.Body.String())
	}
}
