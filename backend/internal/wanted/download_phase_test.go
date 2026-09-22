package wanted

import (
	"context"
	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"testing"
	"time"
)

func TestDownloadPhasesAndQueueActions(t *testing.T) {
	for _, tc := range []struct {
		state, phase string
		progress     float64
	}{
		{"stalledDL", "stalled", 0}, {"metaDL", "waiting_metadata", 0}, {"forcedMetaDL", "waiting_metadata", 0},
		{"downloading", "downloading", 0.5}, {"stoppedDL", "paused", 0}, {"queuedDL", "queued", 0},
		{"stalledUP", "import_ready", 1}, {"error", "failed", 0},
	} {
		t.Run(tc.state, func(t *testing.T) {
			d := acquisition.DownloadStatus{State: tc.state, Progress: tc.progress}
			if got := downloadPhase([]acquisition.DownloadStatus{d}); got != tc.phase {
				t.Fatal(got)
			}
			row := acquisitionQueueItem(WantedItem{}, nil, []acquisition.DownloadStatus{d})
			want := tc.phase
			if want == "failed" {
				want = "blocked"
			}
			if row.State != want {
				t.Fatal(row.State, want)
			}
		})
	}
	failed := acquisition.DownloadStatus{State: "stalledDL", FailureReason: "failed"}
	active := acquisition.DownloadStatus{State: "downloading"}
	for _, ds := range [][]acquisition.DownloadStatus{{failed, active}, {active, failed}} {
		if got := downloadPhase(ds); got != "downloading" {
			t.Fatal(got)
		}
	}
	row := acquisitionQueueItem(WantedItem{ImportReviewID: "review"}, nil, []acquisition.DownloadStatus{{Progress: 1, State: "stalledUP"}})
	if row.State != "import_review" {
		t.Fatal(row)
	}
}
func TestBookProgressSurvivesCollectionMatchAndClientOutage(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	result := bibliographyCandidate(1, "1990-01-01", "Author")
	book, err := store.CreateWanted(ctx, CreateRequest{Result: result, Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	var review string
	if err = db.QueryRow(`insert into import_reviews(source_path,wanted_item_id,title,reason,status,media_format) values('/fixture',$1,'Fixture','Conflicting identifier','pending','ebook') returning id::text`, book.ID).Scan(&review); err != nil {
		t.Fatal(err)
	}
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "fresh", Downloads: []acquisition.DownloadStatus{{State: "stalledUP", Progress: 1, Tags: []string{"wanted:" + book.ID}}}}}
	service := NewService(store, fixture)
	for _, offline := range []bool{false, true} {
		if offline {
			fixture.evidence = acquisition.DownloadEvidence{Status: "unavailable"}
		}
		page, err := service.BookCollection(ctx, BookCollectionQuery{})
		if err != nil {
			t.Fatal(err)
		}
		matches, err := service.MatchBooks(ctx, []BookMatchCandidate{candidateBookIdentity(result, "ebook")})
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range []WantedItem{page.Books[0], matches.Matches[0].Books[0]} {
			if b.ImportReviewID != review || b.ImportReviewReason != "Conflicting identifier" {
				t.Fatal(b)
			}
			if !offline && b.DownloadState != "import_ready" {
				t.Fatal(b.DownloadState)
			}
			if offline && b.DownloadState != "" {
				t.Fatal("stale progress", b.DownloadState)
			}
		}
		queue, err := service.AcquisitionQueue(ctx, AcquisitionQueueQuery{})
		if err != nil || queue.Items[0].State != "import_review" || queue.Summary.Blocked != 1 {
			t.Fatal(queue, err)
		}
	}
}
func TestMetadataTimeoutNeedsOldIdleEvidence(t *testing.T) {
	now := time.Now()
	old := now.Add(-25 * time.Hour)
	recent := now.Add(-time.Minute)
	base := acquisition.DownloadStatus{State: "metaDL", AddedAt: &old}
	if failedDownloadReason(base, 24*time.Hour, now) == "" {
		t.Fatal("metadata timeout missed")
	}
	for _, change := range []func(*acquisition.DownloadStatus){
		func(d *acquisition.DownloadStatus) { d.AddedAt = &recent }, func(d *acquisition.DownloadStatus) { d.AddedAt = nil },
		func(d *acquisition.DownloadStatus) { d.Seeders = 1 }, func(d *acquisition.DownloadStatus) { d.DownloadRate = 1 }, func(d *acquisition.DownloadStatus) { d.Progress = 1 },
	} {
		d := base
		change(&d)
		if got := failedDownloadReason(d, 24*time.Hour, now); got != "" {
			t.Fatal(got)
		}
	}
}

type recoveryFixture struct {
	workerFixture
	downloads []acquisition.DownloadStatus
}

func (f *recoveryFixture) Downloads(context.Context, acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error) {
	return f.downloads, nil
}
func (f *recoveryFixture) MarkDownloadFailed(context.Context, string, string) error { return nil }
func TestRecoverySearchUsesFreshBookRevision(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	book := evidenceBook(t, db, "Recovery fixture", "ebook")
	if _, err := db.Exec(`update wanted_items set status='grabbed',updated_at=now()-interval '1 day' where id=$1`, book.ID); err != nil {
		t.Fatal(err)
	}
	f := &recoveryFixture{downloads: []acquisition.DownloadStatus{{ID: "fixture", State: "stalledDL", Tags: []string{"wanted:" + book.ID}}}}
	s := NewService(store, f)
	run, err := s.RecoverFailedDownloads(ctx, FailedDownloadRequest{DownloadIDs: []string{"fixture"}, Force: true})
	if err != nil || run.ErrorCount != 0 || len(f.searches) == 0 {
		t.Fatal(run, err, f.searches)
	}
}

func TestDownloadReviewAnnotationIsClientScoped(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	book := evidenceBook(t, db, "Review", "ebook")
	_, err := db.Exec(`insert into import_reviews(source_path,download_id,wanted_item_id,title,reason,status,media_format,metadata) values('/fixture','shared-id',$1,'Review','Wrong title','pending','ebook','{"downloadClient":"qBittorrent"}')`, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	downloads := []acquisition.DownloadStatus{
		{ID: "shared-id", Client: "qBittorrent", Tags: []string{"wanted:" + book.ID}},
		{ID: "shared-id", Client: "Transmission", Tags: []string{"wanted:" + book.ID}},
	}
	got := NewService(NewStore(db), nil).AnnotateDownloads(ctx, downloads)
	if got[0].ImportReviewID == "" || got[0].ImportReviewReason != "Wrong title" || got[1].ImportReviewID != "" {
		t.Fatal(got)
	}
}
