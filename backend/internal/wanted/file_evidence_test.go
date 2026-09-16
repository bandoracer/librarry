package wanted

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func evidenceBook(t *testing.T, db *sql.DB, title, format string) WantedItem {
	t.Helper()
	item, err := NewStore(db).CreateWanted(context.Background(), CreateRequest{Result: detailBook("author:evidence", title), Format: format})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func evidenceFile(t *testing.T, db *sql.DB, book WantedItem, path, presence string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`insert into files(media_format,path,size_bytes,checksum,presence_state,import_status,metadata) values($1,$2,10,$3,$4,'imported',jsonb_build_object('wantedId',$5::text)) returning id`, book.Format, path, strings.Repeat("a", 64), presence, book.ID).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func evidenceManifest(t *testing.T, db *sql.DB, book WantedItem, files ...string) string {
	t.Helper()
	var op string
	err := db.QueryRow(`insert into import_operations(source_kind,request_key,wanted_item_id,source_root,destination_root,media_format,import_mode,state) values('manual',gen_random_uuid()::text,$1,'/source','/library',$2,'copy','committed') returning id`, book.ID, book.Format).Scan(&op)
	if err != nil {
		t.Fatal(err)
	}
	for i, file := range files {
		_, err = db.Exec(`insert into import_operation_files(operation_id,file_order,relative_path,source_path,destination_path,size_bytes,sha256,media_format,state,file_id,wanted_item_id) select $1,$2,id::text,'/source/'||id,path,size_bytes,checksum,media_format,'committed',id,$4 from files where id=$3`, op, i, file, book.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	return op
}

func TestFileEvidenceRequiresPositivePresenceAndCompleteAudio(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	ebook := evidenceBook(t, db, "Ebook", "ebook")
	audio := evidenceBook(t, db, "Audio", "audiobook")
	ef := evidenceFile(t, db, ebook, "/library/book.epub", "unknown")
	a := evidenceFile(t, db, audio, "/library/ch1.mp3", "present")
	b := evidenceFile(t, db, audio, "/library/ch2.mp3", "present")
	check := func(id, state string, present, required int) {
		t.Helper()
		all, err := store.WantedFileEvidence(ctx, []string{id})
		if err != nil {
			t.Fatal(err)
		}
		got := all[id]
		if got.State != state || got.PresentFiles != present || got.RequiredFiles != required {
			t.Fatalf("%s: %+v expected %s,%d/%d", id, got, state, present, required)
		}
	}
	check(ebook.ID, "unknown", 0, 0)
	check(audio.ID, "unknown", 2, 0)
	if _, err := db.Exec(`update files set presence_state='present' where id=$1`, ef); err != nil {
		t.Fatal(err)
	}
	check(ebook.ID, "present", 1, 0)
	evidenceManifest(t, db, audio, a, b)
	check(audio.ID, "present", 2, 2)
	// A proven rename preserves IDs/content; historical manifest paths are immutable.
	if _, err := db.Exec(`update files set path='/library/renamed.mp3' where id=$1`, a); err != nil {
		t.Fatal(err)
	}
	check(audio.ID, "present", 2, 2)
	if _, err := db.Exec(`update files set presence_state='missing' where id=$1`, b); err != nil {
		t.Fatal(err)
	}
	check(audio.ID, "incomplete", 1, 2)
	if _, err := db.Exec(`delete from files where id=$1`, b); err != nil {
		t.Fatal(err)
	}
	check(audio.ID, "incomplete", 1, 2)
	if _, err := db.Exec(`update files set checksum=$2 where id=$1`, a, strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	// A surviving row with changed bytes is not a verified chapter.
	check(audio.ID, "unknown", 1, 0)
	// A new complete import can supersede an incomplete historical set.
	c := evidenceFile(t, db, audio, "/library/full.m4b", "present")
	evidenceManifest(t, db, audio, c)
	check(audio.ID, "present", 1, 1)
	if _, err := db.Exec(`update files set presence_state='missing'`); err != nil {
		t.Fatal(err)
	}
	check(ebook.ID, "missing", 0, 0)
	check(audio.ID, "missing", 0, 2)
}

func TestFileEvidenceIsScopedAndRejectsPendingPublicationAndWrongFormat(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	book := evidenceBook(t, db, "Pending", "ebook")
	other := evidenceBook(t, db, "Unrelated", "ebook")
	file := evidenceFile(t, db, book, "/library/pending.epub", "present")
	evidenceFile(t, db, other, "/library/other.epub", "present")
	op := evidenceManifest(t, db, book, file)
	if _, err := db.Exec(`update import_operations set state='failed' where id=$1`, op); err != nil {
		t.Fatal(err)
	}
	result, err := store.WantedFileEvidence(ctx, []string{book.ID})
	if err != nil || len(result) != 1 || result[book.ID].State != "unknown" {
		t.Fatal(result, err)
	}
	if _, err := db.Exec(`update files set media_format='audiobook' where id=$1`, file); err != nil {
		t.Fatal(err)
	}
	result, err = store.WantedFileEvidence(ctx, []string{book.ID})
	if err != nil || result[book.ID].State != "missing" {
		t.Fatal(result, err)
	}
}

type evidenceAcquisition struct {
	Acquisition
	result acquisition.DownloadEvidence
}

func (a evidenceAcquisition) LiveDownloadEvidence(context.Context, acquisition.DownloadListQuery) acquisition.DownloadEvidence {
	return a.result
}

func TestDerivedStateUsesEvidenceAndNeverTreatsOutageAsAbsence(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	book := evidenceBook(t, db, "Evidence", "ebook")
	cases := []struct{ status, downloadState, want string }{
		{"fresh", "", DerivedStateMissing}, {"unavailable", "", DerivedStateUnknown}, {"partial", "", DerivedStateUnknown},
		{"notConfigured", "", DerivedStateMissing}, {"partial", "downloading", DerivedStateDownloading},
		{"fresh", "error", DerivedStateMissing}, {"fresh", "failed", DerivedStateMissing},
	}
	for _, tc := range cases {
		evidence := acquisition.DownloadEvidence{Status: tc.status}
		if tc.downloadState != "" {
			evidence.Downloads = []acquisition.DownloadStatus{{State: tc.downloadState, Tags: []string{"wanted:" + book.ID}}}
		}
		service := NewService(store, evidenceAcquisition{result: evidence})
		got := service.AnnotateWantedStates(ctx, []WantedItem{book})[0]
		if got.DerivedState != tc.want || got.StateEvidence.Downloads != tc.status {
			t.Fatal(tc, got)
		}
	}
	file := evidenceFile(t, db, book, "/library/evidence.epub", "present")
	book.Monitored = false
	service := NewService(store, evidenceAcquisition{result: acquisition.DownloadEvidence{Status: "unavailable"}})
	got := service.AnnotateWantedStates(ctx, []WantedItem{book})[0]
	if got.DerivedState != DerivedStateDownloaded || got.StateEvidence.Message == "" {
		t.Fatal(got)
	}
	if _, err := db.Exec(`update files set presence_state='missing' where id=$1`, file); err != nil {
		t.Fatal(err)
	}
	got = service.AnnotateWantedStates(ctx, []WantedItem{book})[0]
	if got.DerivedState != DerivedStateUnknown {
		t.Fatal(got)
	}
	// Losing the evidence database must not reactivate the frontend's legacy fallback.
	db.Close()
	got = service.AnnotateWantedStates(ctx, []WantedItem{book})[0]
	if got.DerivedState != DerivedStateUnknown || got.StateEvidence.Files.State != "unavailable" {
		t.Fatal(got)
	}
}

func TestCutoffAnnotationBeyondGlobalCollectionCap(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	book := evidenceBook(t, db, "Older book", "ebook")
	evidenceFile(t, db, book, "/library/older.epub", "present")
	// The old implementation read the most recent 200 books globally to annotate a single page.
	for i := 0; i < 205; i++ {
		b := evidenceBook(t, db, fmt.Sprint("Newer ", i), "ebook")
		evidenceFile(t, db, b, fmt.Sprintf("/library/%d.epub", i), "present")
	}
	profile := QualityProfile{Name: book.QualityProfile, MediaFormat: "any", UpgradeAllowed: true, CutoffScore: 100}
	if _, err := store.UpsertQualityProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	got := NewService(store, nil).AnnotateWantedStates(ctx, []WantedItem{book})[0]
	if got.DerivedState != DerivedStateCutoffUnmet {
		t.Fatal(got)
	}
}

func TestImportedDownloadDoesNotHidePartialLibraryLoss(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	book := evidenceBook(t, db, "Imported audio", "audiobook")
	a := evidenceFile(t, db, book, "/library/imported1.mp3", "present")
	b := evidenceFile(t, db, book, "/library/imported2.mp3", "missing")
	evidenceManifest(t, db, book, a, b)
	source := evidenceAcquisition{result: acquisition.DownloadEvidence{Status: "fresh", Downloads: []acquisition.DownloadStatus{{State: "pausedUP", ImportStatus: "imported", Tags: []string{"wanted:" + book.ID}}}}}
	got := NewService(NewStore(db), source).AnnotateWantedStates(ctx, []WantedItem{book})[0]
	if got.DerivedState != DerivedStateIncomplete {
		t.Fatal(got)
	}
	cutoff, err := NewService(NewStore(db), nil).ListCutoffUnmet(ctx)
	if err != nil || len(cutoff) != 0 {
		t.Fatal(cutoff, err)
	}
}
