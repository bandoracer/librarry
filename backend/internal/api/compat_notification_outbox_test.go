package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/notify"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

type compatNoticeFixture struct {
	db                                *sql.DB
	service                           *notify.Service
	received                          chan map[string]any
	target, wanted, release, download string
}

func compatNoticeSetup(t *testing.T) compatNoticeFixture {
	t.Helper()
	f := compatNoticeFixture{db: testdb.Open(t), received: make(chan map[string]any, 10)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/fixture" {
			t.Error(r.Method, r.URL.Path)
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "fixture-user" || password != "fixture-password" {
			t.Error("Basic settings not applied")
		}
		if r.Header.Get("X-Librarry-Delivery-ID") == "" || r.Header.Get("X-Readarr-EventType") == "" {
			t.Error("missing stable/compat headers")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		f.received <- payload
		w.WriteHeader(204)
	}))
	t.Cleanup(server.Close)
	f.service = notify.NewService(notify.NewStore(f.db), nil)
	NewRouter(Dependencies{Notify: f.service})
	payload, _ := json.Marshal(map[string]any{"name": "Compatibility fixture", "implementation": "Webhook", "enable": true, "fields": []map[string]any{{"name": "url", "value": server.URL + "/fixture"}, {"name": "method", "value": "PUT"}, {"name": "username", "value": "fixture-user"}, {"name": "password", "value": "fixture-password"}}})
	if err := f.db.QueryRow(`insert into compat_resources(resource_type,compat_id,name,payload) values('notification',42,'Compatibility fixture',$1) returning id::text`, string(payload)).Scan(&f.target); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`insert into wanted_items(wanted_format,title,author_name,status,monitored,current_release_score) values('ebook','Walden','Henry David Thoreau','imported',true,77) returning id::text`).Scan(&f.wanted); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`insert into releases(wanted_item_id,indexer,title,protocol,download_url,info_url,score) values($1,'Public domain fixture','Walden EPUB','torrent','https://private.invalid/release-secret','https://private.invalid/info-secret',99) returning id::text`, f.wanted).Scan(&f.release); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`insert into downloads(release_id,client,external_id,name,category,save_path,state,size_bytes) values($1,'qBittorrent','download-fixture','Walden EPUB','books','/fixture/downloads','paused',1234) returning id::text`, f.release).Scan(&f.download); err != nil {
		t.Fatal(err)
	}
	return f
}
func (f compatNoticeFixture) history(t *testing.T, kind string, extra map[string]any) {
	t.Helper()
	data := map[string]any{"downloadRecordId": f.download, "downloadId": "download-fixture", "client": "qBittorrent", "releaseId": f.release, "trigger": "upgrade", "title": "Walden EPUB", "score": 88.5, "currentScore": 77, "cutoffScore": 95}
	for k, v := range extra {
		data[k] = v
	}
	raw, _ := json.Marshal(data)
	if _, err := f.db.Exec(`insert into history_events(event_type,entity_type,entity_id,message,data) values($1,'wanted_item',$2,'Fixture committed',$3)`, kind, f.wanted, string(raw)); err != nil {
		t.Fatal(err)
	}
}
func (f compatNoticeFixture) payload(t *testing.T) map[string]any {
	t.Helper()
	select {
	case payload := <-f.received:
		return payload
	case <-time.After(3 * time.Second):
		t.Fatal("fixture receiver did not receive a webhook")
		return nil
	}
}
func (f compatNoticeFixture) run(t *testing.T) {
	t.Helper()
	if _, err := f.service.RunPending(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func (f compatNoticeFixture) delivery(t *testing.T) notify.Delivery {
	t.Helper()
	page, err := f.service.Deliveries(context.Background(), 25, 0)
	if err != nil || len(page.Items) != 1 {
		t.Fatal(page, err)
	}
	return page.Items[0]
}

func TestCompatOutboxPreservesCommittedGrabPayloadAcrossRestartAndEdits(t *testing.T) {
	f := compatNoticeSetup(t)
	tx, err := f.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`insert into history_events(event_type,message) values('release_grabbed','rolled back')`); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	page, err := f.service.Deliveries(context.Background(), 25, 0)
	if err != nil || page.Total != 0 {
		t.Fatal(page, err)
	}
	f.history(t, "release_grabbed", nil)
	d := f.delivery(t)
	if d.TargetKind != "compat" || d.State != "pending" {
		t.Fatal(d)
	}
	if _, err = f.db.Exec(`update wanted_items set title='Changed later'; update downloads set name='Changed later'; update releases set score=999`); err != nil {
		t.Fatal(err)
	}
	f.service = notify.NewService(notify.NewStore(f.db), nil)
	NewRouter(Dependencies{Notify: f.service})
	f.run(t)
	f.run(t)
	select {
	case payload := <-f.received:
		if payload["eventType"] != "Upgrade" || payload["releaseTitle"] != "Walden EPUB" || payload["downloadId"] != "download-fixture" || payload["downloadClient"] != "qBittorrent" || payload["isUpgrade"] != true || payload["currentScore"] != float64(77) || payload["cutoffScore"] != float64(95) {
			t.Fatal(payload)
		}
		book := payload["book"].(map[string]any)
		if book["librarryId"] != f.wanted || book["title"] != "Walden" {
			t.Fatal(book)
		}
		release := payload["releaseDecision"].(map[string]any)
		if release["score"] != 88.5 {
			t.Fatal(release)
		}
		if payload["timestamp"] != d.CreatedAt.UTC().Format(time.RFC3339) {
			t.Fatal(payload["timestamp"], d.CreatedAt)
		}
		raw, _ := json.Marshal(payload)
		if strings.Contains(string(raw), "release-secret") || strings.Contains(string(raw), "info-secret") {
			t.Fatal("credential-bearing release URL escaped")
		}
	default:
		t.Fatal("committed notification lost")
	}
	if len(f.received) != 0 || f.delivery(t).Attempts != 1 {
		t.Fatal("replayed delivery")
	}
}

func TestCompatOutboxImportSnapshotsCompleteFileSet(t *testing.T) {
	f := compatNoticeSetup(t)
	ids := []string{}
	for _, path := range []string{"/library/Chapter 01.m4b", "/library/Chapter 02.m4b"} {
		var id string
		if err := f.db.QueryRow(`insert into files(media_format,path,title,author_name,import_status,metadata) values('audiobook',$1,'Walden','Henry David Thoreau','imported','{"private":"metadata-secret"}') returning id::text`, path).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	f.history(t, "book_imported", map[string]any{"fileIds": ids, "paths": []string{"/library/Chapter 01.m4b", "/library/Chapter 02.m4b"}, "sourceKind": "completed", "operationId": "fixture-operation", "conflictAction": "replace"})
	if _, err := f.db.Exec(`delete from files`); err != nil {
		t.Fatal(err)
	}
	f.run(t)
	payload := f.payload(t)
	if payload["eventType"] != "ReleaseImport" || payload["destinationPath"] != "/library/Chapter 01.m4b" {
		t.Fatal(payload)
	}
	files := payload["bookFiles"].([]any)
	if len(files) != 2 || files[1].(map[string]any)["librarryId"] != ids[1] {
		t.Fatal(files)
	}
	imported := payload["import"].(map[string]any)
	if imported["imported"] != true || imported["replaced"] != true || len(imported["files"].([]any)) != 2 {
		t.Fatal(imported)
	}
	raw, _ := json.Marshal(payload)
	if strings.Contains(string(raw), "metadata-secret") {
		t.Fatal("raw file metadata escaped")
	}
}

func TestCompatOutboxFlagsDeletionAndCurrentSettings(t *testing.T) {
	f := compatNoticeSetup(t)
	if _, err := f.db.Exec(`update compat_resources set payload=payload||'{"onGrab":false,"onUpgrade":false,"onDownload":false}' where id=$1`, f.target); err != nil {
		t.Fatal(err)
	}
	f.history(t, "release_grabbed", nil)
	f.history(t, "book_imported", nil)
	page, err := f.service.Deliveries(context.Background(), 25, 0)
	if err != nil || page.Total != 0 {
		t.Fatal(page, err)
	}
	if _, err = f.db.Exec(`update compat_resources set payload=payload||'{"onUpgrade":true}',updated_at=clock_timestamp() where id=$1`, f.target); err != nil {
		t.Fatal(err)
	}
	f.history(t, "release_grabbed", nil)
	if _, err = f.db.Exec(`update compat_resources set name='Renamed',updated_at=clock_timestamp() where id=$1`, f.target); err != nil {
		t.Fatal(err)
	}
	f.run(t)
	d := f.delivery(t)
	if d.State != "cancelled" || len(f.received) != 0 {
		t.Fatal(d)
	}
	if err = f.service.ResolveDelivery(context.Background(), d.ID, notify.DeliveryResolution{Action: "retry", Confirm: true, ExpectedUpdatedAt: d.UpdatedAt, ExpectedTargetRevision: d.CurrentTargetRevision}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`update compat_resources set deleted_at=now() where id=$1`, f.target); err != nil {
		t.Fatal(err)
	}
	f.run(t)
	d = f.delivery(t)
	if d.State != "cancelled" || d.TargetAvailable || d.CurrentTargetRevision != nil || len(f.received) != 0 {
		t.Fatal(d)
	}
}

func TestCompatOutboxDownloadFailureUsesExactClientAndBook(t *testing.T) {
	f := compatNoticeSetup(t)
	if _, err := f.db.Exec(`update downloads set failed_at=now(),failure_reason='Fixture failure' where id=$1`, f.download); err != nil {
		t.Fatal(err)
	}
	f.run(t)
	payload := f.payload(t)
	if payload["eventType"] != "DownloadFailure" || payload["wantedId"] != f.wanted || payload["downloadClient"] != "qBittorrent" || payload["message"] != "Fixture failure" {
		t.Fatal(payload)
	}
	if _, err := f.db.Exec(`update downloads set failed_at=coalesce(failed_at,now()) where id=$1`, f.download); err != nil {
		t.Fatal(err)
	}
	f.run(t)
	if len(f.received) != 0 {
		t.Fatal("same failure notified twice")
	}
}

func TestCompatOutboxHealthIsOptInAndTargetIDsAreNamespaced(t *testing.T) {
	f := compatNoticeSetup(t)
	if err := f.service.ObserveHealth(context.Background(), "fixture", "warning", "Fixture", "Initial issue"); err != nil {
		t.Fatal(err)
	}
	page, err := f.service.Deliveries(context.Background(), 25, 0)
	if err != nil || page.Total != 0 {
		t.Fatal("health was not opt in", page, err)
	}
	if _, err = f.db.Exec(`update compat_resources set payload=payload||'{"onHealthIssue":true}',updated_at=clock_timestamp() where id=$1`, f.target); err != nil {
		t.Fatal(err)
	}
	if err = f.service.ObserveHealth(context.Background(), "fixture", "ok", "Fixture", ""); err != nil {
		t.Fatal(err)
	}
	if err = f.service.ObserveHealth(context.Background(), "fixture", "error", "Fixture", "New issue"); err != nil {
		t.Fatal(err)
	}
	f.run(t)
	if payload := f.payload(t); payload["eventType"] != "HealthIssue" {
		t.Fatal(payload)
	}
	// UUID identity is scoped to the target namespace, including history joins.
	if _, err = f.db.Exec(`insert into notification_targets(id,name,type,settings) values($1,'Native identity','webhook','{"url":"http://127.0.0.1:1/native"}')`, f.target); err != nil {
		t.Fatal(err)
	}
	f.history(t, "release_grabbed", nil)
	page, err = f.service.Deliveries(context.Background(), 25, 0)
	if err != nil || page.Total != 3 || len(page.Items) != 3 {
		t.Fatal(page, err)
	}
	counts := map[string]int{}
	for _, d := range page.Items {
		counts[d.TargetKind]++
		if d.TargetKind == "native" && d.TargetName != "Native identity" {
			t.Fatal(d)
		}
	}
	if counts["native"] != 1 || counts["compat"] != 2 {
		t.Fatal(counts)
	}
}

func TestCompatImportDoesNotBorrowUnprovenReleaseIdentity(t *testing.T) {
	f := compatNoticeSetup(t)
	f.history(t, "book_imported", map[string]any{"releaseId": "", "score": 0})
	f.run(t)
	payload := f.payload(t)
	if _, ok := payload["releaseDecision"]; ok {
		t.Fatal("import borrowed the download's unproven release", payload)
	}
}

func TestCompatNotificationRecordRetainsLegacyImportTrigger(t *testing.T) {
	record := compatNotificationRecord(map[string]any{"onDownload": false}, 42)
	if record["onReleaseImport"] != false {
		t.Fatal("readback changed the legacy import trigger", record)
	}
	record = compatNotificationRecord(map[string]any{"onDownload": false, "onReleaseImport": true}, 42)
	if record["onReleaseImport"] != true {
		t.Fatal("explicit import trigger lost", record)
	}
}
