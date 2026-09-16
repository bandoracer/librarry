package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type upgradeSelectionAPI struct {
	fakeWanted
	calls []wanted.UpgradeRequest
	err   error
}

func (f *upgradeSelectionAPI) SearchUpgrades(_ context.Context, request wanted.UpgradeRequest) (wanted.UpgradeRun, error) {
	f.calls = append(f.calls, request)
	return wanted.UpgradeRun{Status: "completed"}, f.err
}

func TestUpgradeRequestRejectsInvalidScopeBeforeStartingWork(t *testing.T) {
	for _, body := range []string{`null`, `[]`, `{`, `{"wantedIds":`, `{"wantedIds":[1]}`, `{"wantedIds":[" "]}`, `{"wantedIds":["not-an-id"]}`, `{"wantedId":"typo"}`, `{"limit":201}`, `{} {}`, `{} trailing`} {
		t.Run(body, func(t *testing.T) {
			fixture := &upgradeSelectionAPI{}
			h := handler{deps: Dependencies{Wanted: fixture}}
			res := httptest.NewRecorder()
			h.upgradeWanted(res, httptest.NewRequest(http.MethodPost, "/api/v1/wanted/upgrades", strings.NewReader(body)))
			if res.Code != http.StatusBadRequest || len(fixture.calls) != 0 {
				t.Fatal(res.Code, res.Body.String(), fixture.calls)
			}
		})
	}
}

func TestUpgradeRequestSelectionAndBatchContracts(t *testing.T) {
	var ids []string
	for i := 0; i < 200; i++ {
		ids = append(ids, fmt.Sprintf(`"00000000-0000-0000-0000-%012d"`, i))
	}
	for _, check := range []struct {
		body         string
		count, limit int
	}{
		{`{"wantedIds":[` + strings.Join(ids, ",") + `],"limit":50}`, 200, 200},
		{`{}`, 0, 50},
		{`{"wantedIds":[],"limit":10}`, 0, 10},
		{"", 0, 50},
	} {
		fixture := &upgradeSelectionAPI{}
		h := handler{deps: Dependencies{Wanted: fixture}}
		res := httptest.NewRecorder()
		h.upgradeWanted(res, httptest.NewRequest(http.MethodPost, "/api/v1/wanted/upgrades", strings.NewReader(check.body)))
		if res.Code != http.StatusOK || len(fixture.calls) != 1 || len(fixture.calls[0].WantedIDs) != check.count || fixture.calls[0].Limit != check.limit {
			t.Fatal(res.Code, res.Body.String(), fixture.calls)
		}
	}
}

func TestUpgradeMissingSelectionIsClientError(t *testing.T) {
	fixture := &upgradeSelectionAPI{err: fmt.Errorf("%w: selected book no longer exists", wanted.ErrInvalidUpgradeRequest)}
	h := handler{deps: Dependencies{Wanted: fixture}}
	res := httptest.NewRecorder()
	h.upgradeWanted(res, httptest.NewRequest(http.MethodPost, "/api/v1/wanted/upgrades", strings.NewReader(`{}`)))
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "no longer exists") {
		t.Fatal(res.Code, res.Body.String())
	}
}
