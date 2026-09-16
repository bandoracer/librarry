package acquisition

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTransmissionInventoryKeepsSelectionSeparateFromPriority(t *testing.T) {
	torrent := transmissionTorrentDetail{
		Files:     []transmissionFile{{Name: "Book/1.mp3", Length: 12, BytesCompleted: 12}, {Name: "Book/2.mp3", Length: 12, BytesCompleted: 0}, {Name: "Book/3.mp3", Length: 12, BytesCompleted: 12}},
		FileStats: []transmissionFileStat{{Wanted: true, Priority: 0, BytesCompleted: 12}, {Wanted: false, Priority: 0}},
	}
	files := torrent.DownloadFiles()
	if files[0].Selected == nil || !*files[0].Selected || files[1].Selected == nil || *files[1].Selected || files[2].Selected != nil {
		t.Fatalf("selection was guessed: %+v", files)
	}
}

func TestQBittorrentInventoryDoesNotGuessMissingSelection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/torrents/files" {
			t.Errorf("unexpected route %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"name":"Book/01.mp3","size":12,"progress":1,"priority":1},{"name":"Book/02.mp3","size":12,"progress":1,"priority":0},{"name":"Book/03.mp3","size":12,"progress":1}]`))
	}))
	defer server.Close()
	files, err := NewQBittorrentClient(server.URL, "", "", server.Client()).files(context.Background(), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || files[0].Selected == nil || !*files[0].Selected || files[1].Selected == nil || *files[1].Selected || files[2].Selected != nil {
		t.Fatalf("selection was guessed: %+v", files)
	}
}

func TestSABInventoryRequiresSuccessfulFinalizedHistoryDirectory(t *testing.T) {
	for _, tc := range []struct {
		status, storage, failure string
		accepted                 bool
	}{
		{"Completed", "/complete/Book", "", true},
		{"Extracting", "/complete/Book", "", false},
		{"Failed", "/complete/Book", "", false},
		{"Completed", "", "", false},
		{"Completed", "/complete/Book", "Unpack failed", false},
	} {
		t.Run(tc.status+tc.failure+tc.storage, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"history": map[string]any{"slots": []map[string]any{{"nzo_id": "fixture", "status": tc.status, "storage": tc.storage, "fail_message": tc.failure}}}})
			}))
			defer server.Close()
			client := NewSABnzbdClient(server.URL, "fixture", "", "", server.Client())
			details, found, err := client.historyDetails(context.Background(), "fixture")
			if err != nil || !found {
				t.Fatalf("history: %+v %v", details, err)
			}
			if (details.InventorySource == "completed-directory") != tc.accepted {
				t.Fatalf("incorrect final output evidence: %+v", details)
			}
		})
	}
}
