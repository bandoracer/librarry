package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func audiobookFixture(t *testing.T) (*Service, acquisition.DownloadStatus, string) {
	t.Helper()
	service, db, download, wantedID := operationFixture(t)
	if _, err := db.Exec(`update wanted_items set wanted_format='audiobook',title='Fixture Book' where id=$1`, wantedID); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(download.SavePath, download.Name)); err != nil {
		t.Fatal(err)
	}
	download.Name, download.Category = "Fixture Book", "books-audiobook"
	for _, name := range []string{"Disc 1/1.mp3", "Disc 1/2.mp3", "Disc 1/10.mp3", "Disc 2/1.mp3", "cover.jpg", "metadata.opf"} {
		path := filepath.Join(download.SavePath, download.Name, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		content := []byte("controlled fixture " + name)
		if name == "metadata.opf" {
			content = []byte(`<package><metadata><title>Fixture Book</title><creator>Author</creator></metadata></package>`)
		}
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	service.Reconfigure(Config{EbookRoot: service.Config().EbookRoot, AudiobookRoot: filepath.Join(filepath.Dir(service.Config().EbookRoot), "audio")})
	service.WithDownloadInspector(newFixtureInspector(t, download))
	return service, download, wantedID
}

func TestMultiDiscImportPreservesCompleteSetAndOrder(t *testing.T) {
	service, download, wantedID := audiobookFixture(t)
	ctx := context.Background()
	outcome, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || outcome.Imported != 1 {
		t.Fatalf("import: %+v %v", outcome, err)
	}
	imported := outcome.Results[0].Import
	if len(imported.Files) != 4 {
		t.Fatalf("chapter count: %+v", imported)
	}
	op, err := service.store.getOperation(ctx, imported.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(op.Files) != 6 {
		t.Fatalf("manifest omitted sidecars: %+v", op.Files)
	}
	expected := []string{"Disc 1/1.mp3", "Disc 1/2.mp3", "Disc 1/10.mp3", "Disc 2/1.mp3"}
	for i, name := range expected {
		if !strings.HasSuffix(filepath.ToSlash(op.Files[i].DestinationPath), name) || op.Files[i].WantedID != wantedID {
			t.Fatalf("chapter order/owner %d: %+v", i, op.Files[i])
		}
	}
	retry, err := service.runImportOperation(ctx, op)
	if err != nil || len(retry.Files) != len(imported.Files) {
		t.Fatalf("retry: %+v %v", retry, err)
	}
	for i, file := range imported.Files {
		if retry.Files[i].ID != file.ID {
			t.Fatal("retry changed chapter order")
		}
	}
	download.ImportedFileID = imported.File.ID
	if err := service.VerifyCompletedDownload(ctx, download, nil); err != nil {
		t.Fatal("whole-set cleanup failed", err)
	}
	// Removal of any one chapter blocks cleanup even though three tracked chapters
	// and the compatibility imported_file_id still exist.
	if err := os.Remove(op.Files[2].DestinationPath); err != nil {
		t.Fatal(err)
	}
	if err := service.VerifyCompletedDownload(ctx, download, nil); err == nil {
		t.Fatal("partial library authorized source removal")
	}
	for _, f := range op.Files {
		if _, err := os.Stat(f.SourcePath); err != nil {
			t.Fatal("source lost", err)
		}
	}
}

func TestPayloadReviewRejectsIncompleteOrConflictingEvidence(t *testing.T) {
	for _, testCase := range []string{"unselected", "partial", "size", "unknown-selection", "conflicting-opf", "unlisted-book", "multi-book-pack", "mixed-format", "symlink"} {
		t.Run(testCase, func(t *testing.T) {
			service, download, _ := audiobookFixture(t)
			inspector := service.inspector.(*fixtureInspector)
			switch testCase {
			case "unselected":
				inspector.files[0].Selected = new(false)
			case "partial":
				inspector.files[0].Progress = .5
			case "size":
				inspector.files[0].SizeBytes++
			case "unknown-selection":
				inspector.files[0].Selected = nil
			case "conflicting-opf":
				path := filepath.Join(download.SavePath, download.Name, "Disc 1", "metadata.opf")
				if err := os.WriteFile(path, []byte(`<package><metadata><title>Different Book</title><creator>Different Author</creator><identifier>9781234567897</identifier></metadata></package>`), 0644); err != nil {
					t.Fatal(err)
				}
				service.WithDownloadInspector(newFixtureInspector(t, download))
			case "unlisted-book":
				if err := os.WriteFile(filepath.Join(download.SavePath, download.Name, "other.epub"), []byte("foreign book"), 0644); err != nil {
					t.Fatal(err)
				}
			case "multi-book-pack":
				old := filepath.Join(download.SavePath, download.Name, "Disc 1", "1.mp3")
				if err := os.Rename(old, filepath.Join(filepath.Dir(old), "Unrelated Book.mp3")); err != nil {
					t.Fatal(err)
				}
				service.WithDownloadInspector(newFixtureInspector(t, download))
			case "mixed-format":
				if err := os.WriteFile(filepath.Join(download.SavePath, download.Name, "book.epub"), []byte("ebook edition"), 0644); err != nil {
					t.Fatal(err)
				}
				service.WithDownloadInspector(newFixtureInspector(t, download))
			case "symlink":
				path := filepath.Join(download.SavePath, download.Name, "Disc 1", "1.mp3")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(download.SavePath, download.Name, "Disc 1", "2.mp3"), path); err != nil {
					t.Fatal(err)
				}
			}
			outcome, err := service.ImportCompletedDownloads(context.Background(), []acquisition.DownloadStatus{download}, CompletedImportRequest{})
			if err != nil || outcome.Imported != 0 || outcome.ReviewQueued+outcome.Errored != 1 {
				t.Fatalf("unsafe set accepted: %+v %v", outcome, err)
			}
			var count int
			if err := service.store.db.QueryRow(`select count(*) from files`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("partial file visible: %d %v", count, err)
			}
			if outcome.ReviewQueued == 1 {
				if outcome.Results[0].Review.Metadata["payload"] == nil {
					t.Fatal("review lost manifest")
				}
			}
		})
	}
}

func TestMultiDiscFailureRecoversAllChaptersAsOneCommit(t *testing.T) {
	service, download, _ := audiobookFixture(t)
	ctx := context.Background()
	if _, err := service.store.db.Exec(`alter table downloads add constraint force_failure check(import_status<>'imported')`); err != nil {
		t.Fatal(err)
	}
	first, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || first.Errored != 1 {
		t.Fatalf("fault: %+v %v", first, err)
	}
	op, err := service.store.operationForDownload(ctx, download.Client, download.ID)
	if err != nil {
		t.Fatal(err)
	}
	scan, err := service.Scan(ctx, ScanRequest{Root: service.Config().AudiobookRoot})
	if err != nil || scan.Upserted != 0 || scan.Skipped < 4 {
		t.Fatalf("partial visibility: %+v %v", scan, err)
	}
	if _, err := service.store.db.Exec(`alter table downloads drop constraint force_failure`); err != nil {
		t.Fatal(err)
	}
	retry, err := service.RetryImportOperation(ctx, op.ID)
	if err != nil || !retry.Imported || len(retry.Files) != 4 {
		t.Fatalf("recovery: %+v %v", retry, err)
	}
	if _, err := os.Stat(op.Files[0].SourcePath); err != nil {
		t.Fatal(err)
	}
}

func TestClientInventoryNeverSearchesSharedSiblings(t *testing.T) {
	service, _, download, _ := operationFixture(t)
	if err := os.WriteFile(filepath.Join(download.SavePath, "foreign.epub"), []byte("unrelated"), 0644); err != nil {
		t.Fatal(err)
	}
	payload, err := service.inspectDownloadPayload(context.Background(), download)
	if err != nil || len(payload.Files) != 1 {
		t.Fatalf("foreign sibling entered manifest: %+v %v", payload, err)
	}
	inspector := service.inspector.(*fixtureInspector)
	for _, name := range []string{"../foreign.epub", "/absolute.epub", `C:\foreign.epub`, `Book\..\foreign.epub`} {
		inspector.files[0].Name = name
		if _, err := service.inspectDownloadPayload(context.Background(), download); err == nil {
			t.Fatalf("unsafe client path %s", name)
		}
	}
	inspector.err = errors.New("client unavailable")
	if _, err := service.inspectDownloadPayload(context.Background(), download); err == nil {
		t.Fatal("inventory outage fell back to filesystem scan")
	}
}

func TestReviewedPackMapsFilesToDifferentBooks(t *testing.T) {
	service, db, download, firstID := operationFixture(t)
	if err := os.Remove(filepath.Join(download.SavePath, download.Name)); err != nil {
		t.Fatal(err)
	}
	download.Name = "Book pack"
	var secondID string
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title,author_name) values('ebook','Second Book','Second Author') returning id::text`).Scan(&secondID); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first.epub", "second.epub"} {
		path := filepath.Join(download.SavePath, download.Name, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("controlled "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	service.WithDownloadInspector(newFixtureInspector(t, download))
	ctx := context.Background()
	outcome, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{})
	if err != nil || outcome.ReviewQueued != 1 {
		t.Fatalf("pack not reviewed: %+v %v", outcome, err)
	}
	review := outcome.Results[0].Review
	request := ReviewDecisionRequest{Action: "import", ConfirmIdentity: true, ImportMode: "copy", Mapping: []PayloadMapping{{RelativePath: "Book pack/first.epub", WantedID: firstID}, {RelativePath: "Book pack/second.epub", WantedID: secondID}}}
	preview, err := service.PreviewPayloadReview(ctx, review.ID, request)
	if err != nil || len(preview.Operation.Files) != 2 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	for _, f := range preview.Operation.Files {
		if _, err := os.Stat(f.DestinationPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("preview wrote a file")
		}
	}
	request.PreviewToken = preview.Fingerprint
	resolved, err := service.ResolveImportReview(ctx, review.ID, request)
	if err != nil || resolved.Review.Status != "imported" || len(resolved.Import.Files) != 2 {
		t.Fatalf("mapping: %+v %v", resolved, err)
	}
	for _, id := range []string{firstID, secondID} {
		files, err := service.ListFiles(ctx, FileListQuery{WantedID: id})
		if err != nil || len(files) != 1 || files[0].Metadata["wantedId"] != id {
			t.Fatalf("book link: %+v %v", files, err)
		}
	}
}

func TestSingleM4BImportAndEmbeddedChapterGrouping(t *testing.T) {
	for _, multipart := range []bool{false, true} {
		t.Run(map[bool]string{false: "m4b", true: "id3-chapters"}[multipart], func(t *testing.T) {
			service, db, download, wantedID := operationFixture(t)
			if _, err := db.Exec(`update wanted_items set wanted_format='audiobook',title='Fixture Book' where id=$1`, wantedID); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(download.SavePath, download.Name)); err != nil {
				t.Fatal(err)
			}
			download.Name, download.Category = "release-with-unhelpful-name", "books-audiobook"
			payload := filepath.Join(download.SavePath, download.Name)
			if err := os.Mkdir(payload, 0755); err != nil {
				t.Fatal(err)
			}
			expected := 1
			if multipart {
				expected = 2
				for _, name := range []string{"Introduction.mp3", "The journey.mp3"} {
					writeTestMP3WithID3(t, filepath.Join(payload, name), map[string]string{"TIT2": name, "TALB": "Fixture Book", "TPE1": "Author"})
				}
			} else {
				writeTestM4B(t, filepath.Join(payload, "book.m4b"), map[string]string{"\xa9nam": "Fixture Book", "\xa9ART": "Author"})
			}
			service.Reconfigure(Config{AudiobookRoot: filepath.Join(filepath.Dir(service.Config().EbookRoot), "audio")})
			service.WithDownloadInspector(newFixtureInspector(t, download))
			outcome, err := service.ImportCompletedDownloads(context.Background(), []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
			if err != nil || outcome.Imported != 1 || len(outcome.Results[0].Import.Files) != expected {
				t.Fatalf("audio import: %+v %v", outcome, err)
			}
		})
	}
}

func TestReviewPreviewRejectsChangedContent(t *testing.T) {
	service, download, _ := audiobookFixture(t)
	inspector := service.inspector.(*fixtureInspector)
	inspector.download.Name = "Uncertain pack"
	ctx := context.Background()
	outcome, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{})
	if err != nil || outcome.ReviewQueued != 1 {
		t.Fatalf("review: %+v %v", outcome, err)
	}
	review := outcome.Results[0].Review
	request := ReviewDecisionRequest{Action: "import", ConfirmIdentity: true, ImportMode: "copy"}
	preview, err := service.PreviewPayloadReview(ctx, review.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreviewToken = preview.Fingerprint
	source := preview.Operation.Files[0].SourcePath
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	content[0] ^= 1
	if err := os.WriteFile(source, content, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveImportReview(ctx, review.ID, request); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("stale preview accepted: %v", err)
	}
	var count int
	if err := service.store.db.QueryRow(`select count(*) from import_operations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("stale plan persisted: %d %v", count, err)
	}
}

func TestSABFinalizedOutputDirectoryImportsWholeBook(t *testing.T) {
	service, download, _ := audiobookFixture(t)
	if _, err := service.store.db.Exec(`update downloads set client='SABnzbd'`); err != nil {
		t.Fatal(err)
	}
	download.Client = "SABnzbd"
	inspector := newFixtureInspector(t, download)
	inspector.download.State = "completed"
	inspector.download.SavePath = filepath.Join(download.SavePath, download.Name)
	inspector.inventorySource, inspector.payloadRoot = "completed-directory", inspector.download.SavePath
	inspector.files = nil // get_files archive data is deliberately not used.
	service.WithDownloadInspector(inspector)
	outcome, err := service.ImportCompletedDownloads(context.Background(), []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || outcome.Imported != 1 || len(outcome.Results[0].Import.Files) != 4 {
		t.Fatalf("extracted set: %+v %v", outcome, err)
	}
	inspector.download.State = "extracting"
	if _, err := service.inspectDownloadPayload(context.Background(), download); err == nil {
		t.Fatal("unfinished extraction accepted")
	}
}

func TestRetainedRequiredFileBlocksCleanup(t *testing.T) {
	service, download, _ := audiobookFixture(t)
	inspector := service.inspector.(*fixtureInspector)
	inspector.files[0].Selected = new(false)
	ctx := context.Background()
	outcome, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{})
	if err != nil || outcome.ReviewQueued != 1 {
		t.Fatalf("review: %+v %v", outcome, err)
	}
	review := outcome.Results[0].Review
	request := ReviewDecisionRequest{Action: "import", ConfirmIdentity: true, ImportMode: "copy", Mapping: []PayloadMapping{{RelativePath: inspector.files[0].Name, Exclude: true}}}
	preview, err := service.PreviewPayloadReview(ctx, review.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreviewToken = preview.Fingerprint
	resolved, err := service.ResolveImportReview(ctx, review.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	download.ImportedFileID = resolved.Import.File.ID
	if err := service.VerifyCompletedDownload(ctx, download, nil); err == nil {
		t.Fatal("operator-retained chapter was cleanup-eligible")
	}
}

func TestPayloadReviewDispositionSurvivesWorkerAndCanReopen(t *testing.T) {
	for _, action := range []string{"skip", "reject"} {
		t.Run(action, func(t *testing.T) {
			service, download, _ := audiobookFixture(t)
			service.inspector.(*fixtureInspector).files[0].Selected = new(false)
			ctx := context.Background()
			outcome, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{})
			if err != nil || outcome.ReviewQueued != 1 {
				t.Fatalf("review: %+v %v", outcome, err)
			}
			id := outcome.Results[0].Review.ID
			if _, err := service.ResolveImportReview(ctx, id, ReviewDecisionRequest{Action: action}); err != nil {
				t.Fatal(err)
			}
			service.inspector.(*fixtureInspector).files[0].Selected = new(true)
			outcome, err = service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{})
			if err != nil || outcome.Skipped != 1 || outcome.Imported != 0 {
				t.Fatalf("worker ignored decision: %+v %v", outcome, err)
			}
			if disposition, err := service.store.payloadReviewDisposition(ctx, "Transmission", download.ID); err != nil || disposition != "" {
				t.Fatalf("decision crossed client: %s %v", disposition, err)
			}
			if _, err := service.ResolveImportReview(ctx, id, ReviewDecisionRequest{Action: "reopen"}); err != nil {
				t.Fatal(err)
			}
			if disposition, err := service.store.payloadReviewDisposition(ctx, download.Client, download.ID); err != nil || disposition != "pending" {
				t.Fatalf("not reopened: %s %v", disposition, err)
			}
		})
	}
}

func TestInventoryRejectsAliasesAndExcludedSymlinks(t *testing.T) {
	for _, scenario := range []string{"aliases", "excluded-symlink"} {
		t.Run(scenario, func(t *testing.T) {
			service, _, download, _ := operationFixture(t)
			inspector := service.inspector.(*fixtureInspector)
			if scenario == "aliases" {
				alias := inspector.files[0]
				alias.Name = filepath.Base(download.SavePath) + "/" + alias.Name
				inspector.files = append(inspector.files, alias)
			} else {
				path := filepath.Join(download.SavePath, "unselected.txt")
				if err := os.Symlink(filepath.Join(download.SavePath, download.Name), path); err != nil {
					t.Fatal(err)
				}
				inspector.files = append(inspector.files, acquisition.DownloadFile{Name: "unselected.txt", SizeBytes: 10, Progress: 1, Selected: new(false)})
			}
			if _, err := service.inspectDownloadPayload(context.Background(), download); err == nil {
				t.Fatal("unsafe inventory accepted")
			}
		})
	}
}

func TestBookDirectoryReservationsKeepPackBooksSeparate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Same title")
	reserved := map[string]bool{}
	first, err := planBookDirectory(path, "rename", reserved)
	if err != nil {
		t.Fatal(err)
	}
	second, err := planBookDirectory(path, "rename", reserved)
	if err != nil || first == second {
		t.Fatalf("books share a planned directory: %s %s %v", first, second, err)
	}
	if _, err := os.Stat(first); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("reservation wrote to disk")
	}
}
