package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/notify"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestNotificationDeliveryRoutesRequireCurrentConfirmedDecision(t *testing.T) {
	db := testdb.Open(t)
	service := notify.NewService(notify.NewStore(db), nil)
	_, err := service.CreateTarget(context.Background(), notify.Target{Name: "Fixture", Type: "webhook", Enabled: true, Triggers: notify.DefaultTriggers(), Settings: map[string]string{"url": "http://127.0.0.1:1/fixture-secret", "authorization": "Bearer fixture-token"}})
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{Notify: service})
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(method, path, strings.NewReader(body)))
		return res
	}
	empty := call("GET", "/api/v1/notification-deliveries", "")
	if empty.Code != 200 || !strings.Contains(empty.Body.String(), `"items":[]`) {
		t.Fatal(empty.Code, empty.Body.String())
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=bad", "offset=-1", "offset=bad"} {
		if r := call("GET", "/api/v1/notification-deliveries?"+query, ""); r.Code != 400 {
			t.Fatal(query, r.Code)
		}
	}
	if _, err = db.Exec(`insert into history_events(event_type,message) values('release_grabbed','Fixture'); update notification_deliveries set state='uncertain',attempts=1`); err != nil {
		t.Fatal(err)
	}
	res := call("GET", "/api/v1/notification-deliveries?limit=1&offset=0", "")
	if res.Code != 200 || strings.Contains(res.Body.String(), "fixture-secret") || strings.Contains(res.Body.String(), "fixture-token") {
		t.Fatal(res.Code, res.Body.String())
	}
	var page notify.DeliveryPage
	if err = json.Unmarshal(res.Body.Bytes(), &page); err != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatal(page, err)
	}
	d := page.Items[0]
	path := "/api/v1/notification-deliveries/" + d.ID + "/resolve"
	request := notify.DeliveryResolution{Action: "accepted", ExpectedUpdatedAt: d.UpdatedAt}
	raw, _ := json.Marshal(request)
	if res = call("POST", path, string(raw)); res.Code != 400 {
		t.Fatal(res.Code, res.Body.String())
	}
	request.Confirm = true
	raw, _ = json.Marshal(request)
	if res = call("POST", path, string(raw)); res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	if res = call("POST", path, string(raw)); res.Code != 409 {
		t.Fatal(res.Code, res.Body.String())
	}
	if res = call("POST", "/api/v1/notification-deliveries/unknown/resolve", string(raw)); res.Code != 404 {
		t.Fatal(res.Code, res.Body.String())
	}
	if res = call("POST", path, `{"action":"discard"}`); res.Code != 400 {
		t.Fatal(res.Code, res.Body.String())
	}
}

func TestNotificationDeliveryRoutesUnavailable(t *testing.T) {
	router := NewRouter(Dependencies{})
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		path := "/api/v1/notification-deliveries"
		if method == http.MethodPost {
			path += "/unknown/resolve"
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(method, path, strings.NewReader(`{}`)))
		if res.Code != 503 {
			t.Fatal(res.Code, res.Body.String())
		}
	}
}
