package wanted

import (
	"context"
	"fmt"
	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCurrentReleaseScoreUsesInstalledSnapshotIncludingZero(t *testing.T) {
	s := NewService(nil, nil)
	for _, tc := range []struct {
		item WantedItem
		want float64
	}{
		{WantedItem{CurrentReleaseID: "saved", CurrentReleaseScore: 0}, 0},
		{WantedItem{CurrentReleaseID: "saved", CurrentReleaseScore: 75}, 75},
		{WantedItem{CurrentReleaseScore: 75}, 0},
	} {
		if got := s.currentReleaseScore(context.Background(), tc.item); got != tc.want {
			t.Fatal(got, tc.want)
		}
	}
}
func TestFailedUpgradeDoesNotDemoteImportedOrRemovedBook(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	for _, state := range []string{"imported", "removed", "ignored", "grabbed"} {
		var id string
		if err := db.QueryRow(`insert into wanted_items(wanted_format,title,status) values('ebook','Fixture',$1) returning id::text`, state).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkWantedStatus(context.Background(), id, "wanted"); err != nil {
			t.Fatal(err)
		}
		var actual string
		if err := db.QueryRow(`select status from wanted_items where id=$1`, id).Scan(&actual); err != nil {
			t.Fatal(err)
		}
		want := state
		if state == "grabbed" {
			want = "wanted"
		}
		if actual != want {
			t.Fatal(state, actual, want)
		}
	}
}

func TestWantedGrabPersistsSelectionWithoutCallingItInstalled(t *testing.T) {
	db := testdb.Open(t)
	var adds atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			fmt.Fprint(w, "Ok.")
		case "/api/v2/torrents/add":
			adds.Add(1)
			fmt.Fprint(w, "Ok.")
		default:
			t.Errorf("unexpected client call %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	var wantedID, releaseID string
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title,status) values('ebook','Fixture','wanted') returning id::text`).Scan(&wantedID); err != nil {
		t.Fatal(err)
	}
	magnet := "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"
	if err := db.QueryRow(`insert into releases(wanted_item_id,indexer,title,protocol,download_url,score) values($1,'Fixture','Selected EPUB','torrent',$2,80) returning id::text`, wantedID, magnet).Scan(&releaseID); err != nil {
		t.Fatal(err)
	}
	acquire := acquisition.NewService(acquisition.IntegrationConfig{QBittorrentURL: server.URL, DownloadStore: acquisition.NewSQLDownloadStore(db)})
	service := NewService(NewStore(db), acquire)
	for n := range 2 {
		result, err := service.Grab(context.Background(), wantedID, GrabRequest{ReleaseID: releaseID, Force: true})
		if err != nil || result.ReleaseID != releaseID || result.Deduplicated != (n == 1) {
			t.Fatal(result, err)
		}
	}
	var state, current string
	if err := db.QueryRow(`select status,coalesce(current_release_id::text,'') from wanted_items where id=$1`, wantedID).Scan(&state, &current); err != nil || state != "grabbed" || current != "" {
		t.Fatal(state, current, err)
	}
	var count int
	if err := db.QueryRow(`select count(*) from history_events where event_type='release_grabbed' and data->>'releaseId'=$1 and data->>'trigger'='manual'`, releaseID).Scan(&count); err != nil || count != 1 || adds.Load() != 1 {
		t.Fatal(count, adds.Load(), err)
	}
}
