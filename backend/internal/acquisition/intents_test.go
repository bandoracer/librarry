package acquisition

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

const acquisitionFixtureHash = "0123456789abcdef0123456789abcdef01234567"

type intentFixture struct {
	mu      sync.Mutex
	adds    int
	failAdd bool
	offline bool
	tags    string
	present bool
}

func intentTestService(t *testing.T) (*Service, *sql.DB, *intentFixture, DownloadRequest) {
	t.Helper()
	db := testdb.Open(t)
	f := &intentFixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/api/v2/auth/login":
			fmt.Fprint(w, "Ok.")
		case "/api/v2/torrents/add":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				http.Error(w, "bad", 400)
				return
			}
			f.adds++
			f.tags = r.FormValue("tags")
			f.present = true
			if f.failAdd {
				http.Error(w, "accepted then connection failed", 502)
				return
			}
			fmt.Fprint(w, "Ok.")
		case "/api/v2/torrents/info":
			if f.offline {
				http.Error(w, "offline", 503)
				return
			}
			if !f.present {
				fmt.Fprint(w, "[]")
				return
			}
			json.NewEncoder(w).Encode([]map[string]any{{"hash": acquisitionFixtureHash, "name": "Fixture", "tags": f.tags, "state": "downloading", "category": "books-ebook"}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	var id string
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title,author_name,status) values('ebook','Fixture','Author','wanted') returning id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	service := NewService(IntegrationConfig{QBittorrentURL: server.URL, DownloadStore: NewSQLDownloadStore(db)})
	return service, db, f, DownloadRequest{ReleaseURL: "magnet:?xt=urn:btih:" + acquisitionFixtureHash, Title: "Fixture", Category: "books-ebook", Tags: []string{"librarry", "wanted:" + id}}
}
func readyIntentRecovery(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`update acquisition_intents set next_check_at=null,lease_expires_at=null`); err != nil {
		t.Fatal(err)
	}
}

func TestAcquisitionConcurrentRequestsConvergeAcrossServices(t *testing.T) {
	service, db, fixture, request := intentTestService(t)
	var wg sync.WaitGroup
	for n := 0; n < 12; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			other := NewService(service.IntegrationConfig())
			_, err := other.Grab(context.Background(), request)
			if err != nil && !errors.Is(err, ErrAcquisitionBusy) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	fixture.mu.Lock()
	adds := fixture.adds
	fixture.mu.Unlock()
	if adds != 1 {
		t.Fatalf("submitted %d times", adds)
	}
	repeat, err := service.Grab(context.Background(), request)
	if err != nil || !repeat.Deduplicated || repeat.AcquisitionID == "" {
		t.Fatalf("repeat: %+v %v", repeat, err)
	}
	if _, err := db.Exec(`update downloads set import_status='imported',state='uploading',progress=1`); err != nil {
		t.Fatal(err)
	}
	repeat, err = service.Grab(context.Background(), request)
	if err != nil || repeat.ImportStatus != "imported" || repeat.State != "uploading" {
		t.Fatalf("replay regressed download: %+v %v", repeat, err)
	}
	var count int
	if err := db.QueryRow(`select count(*) from acquisition_intents`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
func TestAcquisitionAcceptedThenErrorReconcilesAfterRestart(t *testing.T) {
	service, db, fixture, request := intentTestService(t)
	fixture.failAdd = true
	if _, err := service.Grab(context.Background(), request); !errors.Is(err, ErrAcquisitionUncertain) {
		t.Fatal(err)
	}
	intents, err := service.AcquisitionRecovery(context.Background())
	if err != nil || len(intents) != 1 || intents[0].State != "uncertain" {
		t.Fatal(intents, err)
	}
	readyIntentRecovery(t, db)
	restarted := NewService(service.IntegrationConfig())
	result, err := restarted.Grab(context.Background(), request)
	if err != nil || result.ID != acquisitionFixtureHash || !result.Deduplicated {
		t.Fatal(result, err)
	}
	fixture.mu.Lock()
	adds := fixture.adds
	fixture.mu.Unlock()
	if adds != 1 {
		t.Fatal("retry added again", adds)
	}
	intents, err = restarted.AcquisitionRecovery(context.Background())
	if err != nil || len(intents) != 0 {
		t.Fatal(intents, err)
	}
}
func TestAcquisitionPersistsAcceptanceBeforeDownloadWrite(t *testing.T) {
	service, db, fixture, request := intentTestService(t)
	if _, err := db.Exec(`alter table downloads add constraint fail_download_write check(name<>'Fixture')`); err != nil {
		t.Fatal(err)
	}
	result, err := service.Grab(context.Background(), request)
	if err == nil || result.AcquisitionID == "" {
		t.Fatal(result, err)
	}
	var state string
	if err := db.QueryRow(`select state from acquisition_intents`).Scan(&state); err != nil || state != "accepted" {
		t.Fatal(state, err)
	}
	if _, err := db.Exec(`alter table downloads drop constraint fail_download_write`); err != nil {
		t.Fatal(err)
	}
	result, err = NewService(service.IntegrationConfig()).Grab(context.Background(), request)
	if err != nil || !result.Deduplicated {
		t.Fatal(result, err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.adds != 1 {
		t.Fatal("receipt replay sent add", fixture.adds)
	}
}
func TestAcquisitionUncertaintyNeverTreatsAbsenceOrOutageAsRejection(t *testing.T) {
	service, db, fixture, request := intentTestService(t)
	fixture.failAdd = true
	if _, err := service.Grab(context.Background(), request); !errors.Is(err, ErrAcquisitionUncertain) {
		t.Fatal(err)
	}
	for _, offline := range []bool{true, false} {
		fixture.mu.Lock()
		fixture.offline = offline
		fixture.present = false
		fixture.mu.Unlock()
		readyIntentRecovery(t, db)
		if _, err := service.Grab(context.Background(), request); !errors.Is(err, ErrAcquisitionUncertain) {
			t.Fatal(err)
		}
	}
	fixture.mu.Lock()
	adds := fixture.adds
	fixture.mu.Unlock()
	if adds != 1 {
		t.Fatal("uncertainty resubmitted", adds)
	}
	intents, _ := service.AcquisitionRecovery(context.Background())
	if err := service.ReleaseAcquisition(context.Background(), intents[0].ID); err != nil {
		t.Fatal(err)
	}
	fixture.mu.Lock()
	fixture.failAdd = false
	fixture.mu.Unlock()
	if _, err := service.Grab(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.adds != 2 {
		t.Fatal("explicit release did not permit new attempt", fixture.adds)
	}
}
func TestAcquisitionExpiredSubmissionOnlyReconcilesOriginalClient(t *testing.T) {
	service, db, fixture, request := intentTestService(t)
	state := service.current.Load()
	candidate, err := state.acquisitionCandidate(request, state.qbit)
	if err != nil {
		t.Fatal(err)
	}
	store := NewSQLDownloadStore(db)
	intent, _, err := store.ClaimAcquisition(context.Background(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update acquisition_intents set lease_expires_at=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	config := service.IntegrationConfig()
	config.QBittorrentURL = "http://127.0.0.1:1"
	service.Reconfigure(config)
	if _, err := service.ReconcileAcquisition(context.Background(), intent.ID, ""); !errors.Is(err, ErrAcquisitionUncertain) {
		t.Fatal(err)
	}
	persisted, err := store.GetAcquisition(context.Background(), intent.ID)
	if err != nil || !strings.Contains(persisted.LastError, "address changed") {
		t.Fatal(persisted, err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.adds != 0 {
		t.Fatal("expired pre-send claim was resent")
	}
}
func TestAcquisitionBookScopeBlocksDifferentClientAndLegacyDownloads(t *testing.T) {
	service, db, _, request := intentTestService(t)
	if _, err := service.Grab(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	config := service.IntegrationConfig()
	config.TransmissionURL = config.QBittorrentURL
	service.Reconfigure(config)
	request.Client = "Transmission"
	if _, err := service.Grab(context.Background(), request); !errors.Is(err, ErrAcquisitionActive) {
		t.Fatal(err)
	}
	if _, err := db.Exec(`delete from acquisition_intents`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Grab(context.Background(), request); !errors.Is(err, ErrAcquisitionActive) {
		t.Fatal("legacy active download bypassed", err)
	}
}

func TestAcquisitionTransmissionErrorRecoversByHashWithoutLabels(t *testing.T) {
	db := testdb.Open(t)
	var mu sync.Mutex
	adds := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var request struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		switch request.Method {
		case "torrent-add":
			adds++
			http.Error(w, "accepted but response lost", 502)
		case "torrent-get":
			json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{"torrents": []map[string]any{{"id": 2, "hashString": acquisitionFixtureHash, "name": "Fixture", "status": 4, "labels": []string{}}}}})
		default:
			http.Error(w, "unexpected", 400)
		}
	}))
	defer server.Close()
	service := NewService(IntegrationConfig{TransmissionURL: server.URL, DownloadStore: NewSQLDownloadStore(db)})
	request := DownloadRequest{Client: "Transmission", ReleaseURL: "magnet:?xt=urn:btih:" + acquisitionFixtureHash, Title: "Fixture"}
	if _, err := service.Grab(context.Background(), request); !errors.Is(err, ErrAcquisitionUncertain) {
		t.Fatal(err)
	}
	readyIntentRecovery(t, db)
	result, err := NewService(service.IntegrationConfig()).Grab(context.Background(), request)
	if err != nil || result.Client != "Transmission" || !result.Deduplicated {
		t.Fatal(result, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if adds != 1 {
		t.Fatal(adds)
	}
}
func TestAcquisitionSABNeedsExplicitIDAfterLostAcknowledgement(t *testing.T) {
	db := testdb.Open(t)
	var mu sync.Mutex
	adds := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Query().Get("mode") {
		case "addurl":
			adds++
			http.Error(w, "accepted but acknowledgement lost", 502)
		case "queue":
			json.NewEncoder(w).Encode(map[string]any{"queue": map[string]any{"slots": []map[string]any{{"nzo_id": "opaque-job-id", "filename": "Fixture", "status": "Downloading", "cat": "books-ebook"}}}})
		case "history":
			fmt.Fprint(w, `{"history":{"slots":[]}}`)
		default:
			http.Error(w, "unexpected", 400)
		}
	}))
	defer server.Close()
	service := NewService(IntegrationConfig{SABnzbdURL: server.URL, SABnzbdAPIKey: "fixture-only", DownloadStore: NewSQLDownloadStore(db)})
	request := DownloadRequest{Client: "SABnzbd", ReleaseURL: "https://fixture.invalid/book.nzb?apikey=not-to-be-stored", Title: "Fixture"}
	if _, err := service.Grab(context.Background(), request); !errors.Is(err, ErrAcquisitionUncertain) {
		t.Fatal(err)
	}
	intents, err := service.AcquisitionRecovery(context.Background())
	if err != nil || len(intents) != 1 {
		t.Fatal(intents, err)
	}
	readyIntentRecovery(t, db)
	if _, err := service.ReconcileAcquisition(context.Background(), intents[0].ID, ""); !errors.Is(err, ErrAcquisitionUncertain) {
		t.Fatal("same title was guessed", err)
	}
	readyIntentRecovery(t, db)
	result, err := service.ReconcileAcquisition(context.Background(), intents[0].ID, "opaque-job-id")
	if err != nil || result.ID != "opaque-job-id" {
		t.Fatal(result, err)
	}
	var raw string
	if err := db.QueryRow(`select row_to_json(acquisition_intents)::text from acquisition_intents`).Scan(&raw); err != nil || strings.Contains(raw, "not-to-be-stored") {
		t.Fatal("request secret persisted", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if adds != 1 {
		t.Fatal(adds)
	}
}
func TestAcquisitionReleaseKeepsActiveLeaseAndImportedBookCanUpgrade(t *testing.T) {
	service, db, fixture, request := intentTestService(t)
	state := service.current.Load()
	store := NewSQLDownloadStore(db)
	candidate, _ := state.acquisitionCandidate(request, state.qbit)
	intent, _, err := store.ClaimAcquisition(context.Background(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ReleaseAcquisition(context.Background(), intent.ID); !errors.Is(err, ErrAcquisitionBusy) {
		t.Fatal("released active sender", err)
	}
	readyIntentRecovery(t, db)
	if err := service.ReleaseAcquisition(context.Background(), intent.ID); err != nil {
		t.Fatal(err)
	}
	result, err := service.Grab(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update downloads set import_status='imported' where external_id=$1`, result.ID); err != nil {
		t.Fatal(err)
	}
	request.ReleaseURL = "magnet:?xt=urn:btih:1123456789abcdef0123456789abcdef01234567"
	upgraded, err := service.Grab(context.Background(), request)
	if err != nil || upgraded.AcquisitionID == result.AcquisitionID {
		t.Fatal(upgraded, err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.adds != 2 {
		t.Fatal(fixture.adds)
	}
}

func TestAcquisitionClientObservationPreservesBookAssociation(t *testing.T) {
	service, db, _, request := intentTestService(t)
	status, err := service.Grab(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	store := NewSQLDownloadStore(db)
	observed := []DownloadStatus{{Client: status.Client, ID: status.ID, Name: "Fixture", State: "downloading", Tags: []string{"client-only"}}}
	if err := store.UpsertDownloads(context.Background(), observed); err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{request.Tags[1], "librarry-intent:" + status.AcquisitionID, "client-only"} {
		if !strings.Contains(strings.Join(observed[0].Tags, ","), tag) {
			t.Fatalf("association lost: %+v", observed)
		}
	}
}

func TestAcquisitionRawManualAndBookWorkerShareReleaseReservation(t *testing.T) {
	service, db, fixture, request := intentTestService(t)
	raw := request
	raw.Tags = []string{"librarry"}
	if _, err := service.Grab(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Grab(context.Background(), request); !errors.Is(err, ErrAcquisitionActive) {
		t.Fatal("book worker bypassed manual release reservation", err)
	}
	if _, err := db.Exec(`update downloads set state='removed'`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Grab(context.Background(), request); err != nil {
		t.Fatal("explicitly removed raw acquisition remained reserved", err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.adds != 2 {
		t.Fatal(fixture.adds)
	}
}

func TestAcquisitionUncertainBookCannotMasqueradeAsDifferentRelease(t *testing.T) {
	service, db, fixture, request := intentTestService(t)
	fixture.failAdd = true
	if _, err := service.Grab(context.Background(), request); !errors.Is(err, ErrAcquisitionUncertain) {
		t.Fatal(err)
	}
	readyIntentRecovery(t, db)
	request.ReleaseURL = "magnet:?xt=urn:btih:1123456789abcdef0123456789abcdef01234567"
	if _, err := service.Grab(context.Background(), request); !errors.Is(err, ErrAcquisitionActive) {
		t.Fatal("older download was returned for a different requested release", err)
	}
	intents, err := service.AcquisitionRecovery(context.Background())
	if err != nil || len(intents) != 1 || intents[0].State != "uncertain" {
		t.Fatal(intents, err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.adds != 1 {
		t.Fatal(fixture.adds)
	}
}
