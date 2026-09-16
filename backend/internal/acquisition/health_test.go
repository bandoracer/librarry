package acquisition

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func healthFixtureConfig(url string) IntegrationConfig {
	return IntegrationConfig{ProwlarrURL: url, ProwlarrAPIKey: "private-key", QBittorrentURL: url, QBittorrentUser: "fixture", QBittorrentPass: "private-password", TransmissionURL: url, TransmissionUser: "fixture", TransmissionPass: "private-password", SABnzbdURL: url, SABnzbdAPIKey: "private-key"}
}
func ageIntegrationCheck(service *Service, index int) {
	o := service.current.Load().health[index]
	o.mu.Lock()
	defer o.mu.Unlock()
	at := time.Now().Add(-time.Minute)
	o.latest.LastCheckedAt = &at
}

func TestIntegrationHealthChecksValidateProtocolsAndRetainEvidence(t *testing.T) {
	for index, name := range []string{"Prowlarr", "qBittorrent", "Transmission", "SABnzbd"} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			var malformed atomic.Bool
			var outage atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if outage.Load() {
					w.WriteHeader(503)
					fmt.Fprint(w, "private-key private-password")
					return
				}
				if malformed.Load() {
					fmt.Fprint(w, `{"error":"private-key private-password"}`)
					return
				}
				switch name {
				case "Prowlarr":
					if r.URL.Path != "/api/v1/system/status" || r.Header.Get("X-Api-Key") != "private-key" {
						t.Error("incorrect Prowlarr check")
					}
					fmt.Fprint(w, `{"appName":"Prowlarr","version":"2.3.4.5-private-build"}`)
				case "qBittorrent":
					if r.URL.Path == "/api/v2/auth/login" {
						http.SetCookie(w, &http.Cookie{Name: "SID", Value: "fixture-session", Path: "/"})
						fmt.Fprint(w, "Ok.")
						return
					}
					if cookie, err := r.Cookie("SID"); err != nil || cookie.Value != "fixture-session" {
						w.WriteHeader(403)
						return
					}
					if r.URL.Path != "/api/v2/app/version" {
						t.Error(r.URL.Path)
					}
					fmt.Fprint(w, "v5.0.4-private-build")
				case "Transmission":
					if r.Header.Get("X-Transmission-Session-Id") != "fixture-session" {
						w.Header().Set("X-Transmission-Session-Id", "fixture-session")
						w.WriteHeader(409)
						return
					}
					var request struct {
						Method string `json:"method"`
					}
					json.NewDecoder(r.Body).Decode(&request)
					if request.Method != "session-get" {
						t.Error(request)
					}
					fmt.Fprint(w, `{"result":"success","arguments":{"version":"4.0.6 (private-build)","rpc-version":17}}`)
				case "SABnzbd":
					if r.URL.Query().Get("mode") != "queue" || r.URL.Query().Get("limit") != "1" || r.URL.Query().Get("apikey") != "private-key" {
						t.Error("SAB check must verify privileged read")
					}
					fmt.Fprint(w, `{"queue":{"status":"Idle","slots":[],"version":"4.5.3-private-build"}}`)
				}
			}))
			defer server.Close()
			service := NewService(healthFixtureConfig(server.URL))
			before := service.Health(context.Background())[index]
			if before.Status != "configured" || before.LastCheckedAt != nil || calls.Load() != 0 {
				t.Fatal(before, calls.Load())
			}
			good, err := service.CheckIntegration(context.Background(), name)
			if err != nil || good.Status != "ready" || good.Authenticated == nil || !*good.Authenticated || good.LastCheckedAt == nil || good.LastSuccessAt == nil || good.Version == "" {
				t.Fatal(good, err)
			}
			originalCalls := calls.Load()
			for range 3 {
				snapshot := service.Health(context.Background())[index]
				if !snapshot.LastCheckedAt.Equal(*good.LastCheckedAt) {
					t.Fatal("snapshot refreshed evidence")
				}
			}
			service.CheckIntegration(context.Background(), name)
			if calls.Load() != originalCalls {
				t.Fatal("snapshot or immediate duplicate check spent requests")
			}
			ageIntegrationCheck(service, index)
			malformed.Store(true)
			bad, _ := service.CheckIntegration(context.Background(), name)
			if bad.Status == "ready" || !bad.LastSuccessAt.Equal(*good.LastSuccessAt) || bad.Version != good.Version || !bad.LastVersionAt.Equal(*good.LastVersionAt) {
				t.Fatal(bad)
			}
			raw, _ := json.Marshal(bad)
			for _, secret := range []string{"private-key", "private-password", "private-build", server.URL} {
				if strings.Contains(string(raw), secret) {
					t.Fatal("private input leaked", string(raw))
				}
			}
			ageIntegrationCheck(service, index)
			outage.Store(true)
			down, _ := service.CheckIntegration(context.Background(), name)
			if down.Status != "unavailable" || !down.LastSuccessAt.Equal(*good.LastSuccessAt) {
				t.Fatal(down)
			}
			ageIntegrationCheck(service, index)
			malformed.Store(false)
			outage.Store(false)
			recovered, _ := service.CheckIntegration(context.Background(), name)
			if recovered.Status != "ready" || !recovered.LastSuccessAt.After(*good.LastSuccessAt) {
				t.Fatal(recovered)
			}
			o := service.current.Load().health[index]
			o.mu.Lock()
			at := time.Now().Add(-11 * time.Minute)
			o.latest.LastCheckedAt = &at
			o.mu.Unlock()
			stale := service.Health(context.Background())[index]
			if stale.Status != "stale" || stale.ObservedStatus != "ready" || stale.Freshness != "stale" {
				t.Fatal(stale)
			}
			service.Reconfigure(healthFixtureConfig(server.URL))
			reset := service.Health(context.Background())[index]
			if reset.Status != "configured" || reset.LastCheckedAt != nil || reset.LastSuccessAt != nil || reset.Version != "" {
				t.Fatal("new configuration inherited old evidence", reset)
			}
		})
	}
}

func TestIntegrationChecksCoalesceAndFenceChangedConfiguration(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(started)
		<-release
		fmt.Fprint(w, `{"appName":"Prowlarr","version":"2.0.0"}`)
	}))
	defer server.Close()
	service := NewService(healthFixtureConfig(server.URL))
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h, err := service.CheckIntegration(context.Background(), "Prowlarr")
			if err != nil || h.Status != "ready" {
				t.Error(h, err)
			}
		}()
	}
	<-started
	if !service.Health(context.Background())[0].Checking {
		t.Fatal("active check invisible")
	}
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
	// A check that completes on an old generation cannot attach success to newly
	// configured credentials, even when its remote endpoint is unchanged.
	started2, release2 := make(chan struct{}), make(chan struct{})
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started2)
		<-release2
		fmt.Fprint(w, `{"appName":"Prowlarr","version":"2.0.0"}`)
	}))
	defer server2.Close()
	service.Reconfigure(healthFixtureConfig(server2.URL))
	result := make(chan error, 1)
	go func() { _, err := service.CheckIntegration(context.Background(), "Prowlarr"); result <- err }()
	<-started2
	service.Reconfigure(IntegrationConfig{})
	close(release2)
	if err := <-result; err != ErrIntegrationChanged {
		t.Fatal(err)
	}
	if h := service.Health(context.Background())[0]; h.Configured || h.LastCheckedAt != nil {
		t.Fatal(h)
	}
}

func TestIntegrationRateLimitCancellationAndRedirects(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(429)
		fmt.Fprint(w, "private server body")
	}))
	defer server.Close()
	service := NewService(healthFixtureConfig(server.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelled, _ := service.CheckIntegration(ctx, "Prowlarr")
	if cancelled.LastCheckedAt != nil || calls.Load() != 0 {
		t.Fatal(cancelled)
	}
	limited, _ := service.CheckIntegration(context.Background(), "Prowlarr")
	if limited.Status != "rate_limited" || limited.RetryAfter == nil {
		t.Fatal(limited)
	}
	ageIntegrationCheck(service, 0)
	service.CheckIntegration(context.Background(), "Prowlarr")
	if calls.Load() != 1 {
		t.Fatal("retry-after ignored")
	}
	var forwarded atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1) }))
	defer receiver.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, receiver.URL, 302) }))
	defer redirect.Close()
	service.Reconfigure(healthFixtureConfig(redirect.URL))
	result, _ := service.CheckIntegration(context.Background(), "Prowlarr")
	if result.Status == "ready" || forwarded.Load() != 0 {
		t.Fatal(result, forwarded.Load())
	}
}

func TestIntegrationHealthRejectsEmptyMalformedAndWrongShapeBodies(t *testing.T) {
	for _, body := range []string{"", "null", "{}", "[]", "<html>login</html>", `{"appName":"Prowlarr","version":"2.0.0"} trailing`, `{"result":"failure","arguments":{"version":"4.0.6","rpc-version":17}}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			cfg := healthFixtureConfig(server.URL)
			cfg.QBittorrentUser = ""
			cfg.QBittorrentPass = ""
			service := NewService(cfg)
			for _, name := range []string{"Prowlarr", "qBittorrent", "Transmission", "SABnzbd"} {
				h, err := service.CheckIntegration(context.Background(), name)
				if err != nil || h.Status == "ready" || h.LastSuccessAt != nil {
					t.Fatal(name, h, err)
				}
			}
		})
	}
}

func TestQBittorrentInjectedClientRetainsSessionWithoutMutatingCaller(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "fixture", Path: "/"})
			fmt.Fprint(w, "Ok.")
			return
		}
		if _, err := r.Cookie("SID"); err != nil {
			w.WriteHeader(403)
			return
		}
		fmt.Fprint(w, `[]`)
	}))
	defer server.Close()
	original := server.Client()
	client := NewQBittorrentClient(server.URL, "user", "password", original)
	if original.Jar != nil || client.client.Jar == nil {
		t.Fatal("caller client was mutated or session jar missing")
	}
	if _, err := client.List(context.Background(), DownloadListQuery{}); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationCancelledInflightCheckDoesNotRecordOutage(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	service := NewService(healthFixtureConfig(server.URL))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan IntegrationHealth, 1)
	go func() { result, _ := service.CheckIntegration(ctx, "Prowlarr"); done <- result }()
	<-started
	cancel()
	result := <-done
	if result.LastCheckedAt != nil || result.Status != "configured" || result.Checking {
		t.Fatal(result)
	}
}

func TestIntegrationAccessFailuresAndOversizedResponses(t *testing.T) {
	for _, code := range []int{401, 403, 200} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				if code == 200 {
					fmt.Fprint(w, strings.Repeat("1", (1<<20)+2))
				} else {
					fmt.Fprint(w, "private-key")
				}
			}))
			defer server.Close()
			service := NewService(healthFixtureConfig(server.URL))
			for _, name := range []string{"Prowlarr", "qBittorrent", "Transmission", "SABnzbd"} {
				result, _ := service.CheckIntegration(context.Background(), name)
				if result.Status == "ready" || result.LastSuccessAt != nil || strings.Contains(result.Message, "private-key") {
					t.Fatal(result)
				}
				if code != 200 && (result.Status != "invalid_credentials" || result.Authenticated == nil || *result.Authenticated) {
					t.Fatal(result)
				}
			}
		})
	}
}

func TestQBittorrentRejectsLoginTextContainingOK(t *testing.T) {
	var otherRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/auth/login" {
			otherRequests.Add(1)
		}
		fmt.Fprint(w, "not ok private-password")
	}))
	defer server.Close()
	client := NewQBittorrentClient(server.URL, "user", "private-password", server.Client())
	if _, err := client.List(context.Background(), DownloadListQuery{}); err == nil || strings.Contains(err.Error(), "private-password") {
		t.Fatal(err)
	}
	if otherRequests.Load() != 0 {
		t.Fatal("login failure was accepted")
	}
}

func TestHealthVersionAcceptsReleaseFormsWithoutExportingBuildSuffixes(t *testing.T) {
	for value, want := range map[string]string{"v5.0.4": "5.0.4", "v5.2.0alpha1": "5.2.0", "4.5.0RC1": "4.5.0", "4.0.6 (private-build)": "4.0.6", "2.3.4.5-private-build": "2.3.4.5", "5.0.4 <html>login</html>": "", "5.0.4 (unfinished": "", "5.0.4 trailing content": "", "1234567890.1": ""} {
		if got := healthVersion(value); got != want {
			t.Errorf("%q: got %q want %q", value, got, want)
		}
	}
}
