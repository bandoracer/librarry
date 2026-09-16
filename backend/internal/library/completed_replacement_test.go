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

func replacementFixture(t *testing.T, audio bool) (*Service, ImportOperation, []FileRecord, acquisition.DownloadStatus) {
	t.Helper()
	var s *Service
	var d acquisition.DownloadStatus
	if audio {
		s, d, _ = audiobookFixture(t)
	} else {
		s, _, d, _ = operationFixture(t)
	}
	ctx := context.Background()
	first, err := s.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{d}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || first.Imported != 1 {
		t.Fatal(first, err)
	}
	original := first.Results[0].Import.Files
	if _, err := s.store.db.Exec(`update files set title='Manual title',author_name='Manual author',metadata=metadata||'{"manualNote":"retain this"}'`); err != nil {
		t.Fatal(err)
	}
	d.ID = "replacement"
	d.SavePath = t.TempDir()
	source := filepath.Join(d.SavePath, d.Name)
	if audio {
		for _, name := range []string{"Disc 1/1.mp3", "Disc 1/2.mp3", "Disc 1/10.mp3", "Disc 2/1.mp3", "cover.jpg", "metadata.opf"} {
			path := filepath.Join(source, name)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			content := []byte("replacement bytes " + name)
			if name == "metadata.opf" {
				content = []byte(`<package><metadata><title>Fixture Book</title><creator>Author</creator></metadata></package>`)
			}
			if err := os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
		}
	} else if err := os.WriteFile(source, []byte("replacement ebook fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.Exec(`insert into downloads(client,external_id,category,save_path,state) values($1,$2,'books',$3,'pausedUP')`, d.Client, d.ID, d.SavePath); err != nil {
		t.Fatal(err)
	}
	s.WithDownloadInspector(newFixtureInspector(t, d))
	payload, err := s.inspectDownloadPayload(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	request := ImportRequest{WantedID: wantedIDFromTags(d.Tags), ImportMode: "copy", ConflictAction: "replace"}
	if _, _, err := s.planPayload(ctx, payload, request, nil, false); err == nil {
		t.Fatal("unconfirmed replacement planned")
	}
	op, _, err := s.planPayload(ctx, payload, request, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	again, _, err := s.planPayload(ctx, payload, request, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := payloadPlanFingerprint(op)
	b, _ := payloadPlanFingerprint(again)
	if a != b {
		t.Fatal("replacement preview is unstable")
	}
	for _, file := range op.Files {
		if file.PreviousPath == "" {
			t.Fatal("missing backup plan", file)
		}
		if _, err := os.Stat(file.PreviousPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("preview changed filesystem", err)
		}
	}
	op, err = s.store.planOperation(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	return s, op, original, d
}
func TestCompletedReplacementRestartsWithoutLosingOriginals(t *testing.T) {
	for _, audio := range []bool{false, true} {
		t.Run(map[bool]string{false: "ebook", true: "chapters"}[audio], func(t *testing.T) {
			s, op, original, d := replacementFixture(t, audio)
			ctx := context.Background()
			if _, err := s.store.db.Exec(`alter table import_operations add constraint inject_replacement_commit check(id<>'` + op.ID + `' or state<>'committed')`); err != nil {
				t.Fatal(err)
			}
			if _, err := s.runImportOperation(ctx, op); err == nil {
				t.Fatal("commit failure hidden")
			}
			for _, f := range op.Files {
				if err := verifyPreviousImportFile(f); err != nil {
					t.Fatal("original lost", err)
				}
				if err := verifyManifestPath(f.DestinationPath, f); err != nil {
					t.Fatal(err)
				}
				if err := verifyManifestPath(f.SourcePath, f); err != nil {
					t.Fatal("download source lost", err)
				}
			}
			visible, err := s.ListFiles(ctx, FileListQuery{Format: "any"})
			if err != nil || len(visible) != 0 {
				t.Fatal("unfinished replacement exposed", visible, err)
			}
			if _, err := s.store.db.Exec(`alter table import_operations drop constraint inject_replacement_commit`); err != nil {
				t.Fatal(err)
			}
			restarted := NewService(s.store, s.Config(), s.wanted, s.downloads).WithDownloadInspector(s.inspector)
			out, err := restarted.RetryImportOperation(ctx, op.ID)
			if err != nil || !out.Replaced || len(out.Files) != len(original) {
				t.Fatal(out, err)
			}
			ids := map[string]bool{}
			for _, f := range original {
				ids[f.ID] = true
			}
			for _, f := range out.Files {
				if !ids[f.ID] || f.Title != "Manual title" || f.AuthorName != "Manual author" || f.Metadata["manualNote"] != "retain this" {
					t.Fatal("identity or manual correction lost", f)
				}
			}
			saved, err := s.store.getOperation(ctx, op.ID)
			if err != nil || saved.ReplacementCleanupState != "cleaned" || saved.CleanupState != "blocked" {
				t.Fatal(saved, err)
			}
			for _, f := range saved.Files {
				if _, err := os.Stat(f.PreviousPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("backup cleanup incomplete", err)
				}
				if err := verifyManifestPath(f.SourcePath, f); err != nil {
					t.Fatal(err)
				}
			}
			repeat, err := restarted.RetryImportOperation(ctx, op.ID)
			if err != nil || !repeat.Skipped || !repeat.Replaced {
				t.Fatal(repeat, err)
			}
			d.ImportedFileID = out.File.ID
			if err := restarted.VerifyCompletedDownload(ctx, d, nil); err != nil {
				t.Fatal("new receipt cannot verify complete destinations", err)
			}
		})
	}
}
func TestCompletedReplacementCleanupFailureIsVisibleAndRetryable(t *testing.T) {
	s, op, _, _ := replacementFixture(t, false)
	ctx := context.Background()
	bin := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(bin, []byte("block recycle"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := s.Config()
	cfg.RecycleBin = bin
	s.Reconfigure(cfg)
	out, err := s.runImportOperation(ctx, op)
	if err == nil || !out.Imported {
		t.Fatal(out, err)
	}
	saved, err := s.store.getOperation(ctx, op.ID)
	if err != nil || saved.State != "committed" || saved.ReplacementCleanupState != "pending" || saved.ReplacementCleanupError == "" {
		t.Fatal(saved, err)
	}
	report, err := s.ImportRecovery(ctx)
	if err != nil || report.Unfinished != 1 {
		t.Fatal(report, err)
	}
	for _, f := range saved.Files {
		if err := verifyPreviousImportFile(f); err != nil {
			t.Fatal("unavailable bin lost backup", err)
		}
	}
	cfg.RecycleBin = ""
	s.Reconfigure(cfg)
	if _, err := s.RetryImportOperation(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	report, err = s.ImportRecovery(ctx)
	if err != nil || report.Unfinished != 0 {
		t.Fatal(report, err)
	}
}
func TestCompletedReplacementChecksChangedBookOwnership(t *testing.T) {
	s, op, original, _ := replacementFixture(t, false)
	ctx := context.Background()
	if _, err := s.store.db.Exec(`with w as(insert into wanted_items(wanted_format,title) values('ebook','Other book') returning id) insert into file_wanted_links(file_id,wanted_item_id) select $1,w.id from w`, original[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runImportOperation(ctx, op); err == nil || !strings.Contains(err.Error(), "different book") {
		t.Fatal(err)
	}
	previous := op.Files[0]
	previous.SHA256, previous.SizeBytes = previous.PreviousSHA256, previous.PreviousSizeBytes
	if err := verifyManifestPath(previous.DestinationPath, previous); err != nil {
		t.Fatal("conflicting destination changed", err)
	}
}

func TestCompletedReplacementPreviewRejectsChangedBytesAndExtraChapters(t *testing.T) {
	for _, audio := range []bool{false, true} {
		t.Run(map[bool]string{false: "bytes", true: "extra-chapter"}[audio], func(t *testing.T) {
			s, op, _, download := replacementFixture(t, audio)
			ctx := context.Background()
			// Use another external ID to exercise the real review route before planning.
			download.ID = "reviewed-replacement"
			if _, err := s.store.db.Exec(`insert into downloads(client,external_id,category,save_path,state) values($1,$2,'books',$3,'pausedUP')`, download.Client, download.ID, download.SavePath); err != nil {
				t.Fatal(err)
			}
			// Discard only this untouched test plan; no filesystem mutation has run.
			if _, err := s.store.db.Exec(`delete from import_operations where id=$1`, op.ID); err != nil {
				t.Fatal(err)
			}
			s.WithDownloadInspector(newFixtureInspector(t, download))
			outcome, err := s.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ConflictAction: "replace"})
			if err != nil || outcome.ReviewQueued != 1 {
				t.Fatal(outcome, err)
			}
			review := outcome.Results[0].Review
			request := ReviewDecisionRequest{Action: "import", WantedID: wantedIDFromTags(download.Tags), ImportMode: "copy", ConflictAction: "replace", ConfirmIdentity: true}
			preview, err := s.PreviewPayloadReview(ctx, review.ID, request)
			if err != nil {
				t.Fatal(err)
			}
			if audio {
				path := filepath.Join(filepath.Dir(preview.Operation.Files[0].DestinationPath), "old-extra-chapter.mp3")
				if err := os.WriteFile(path, []byte("retain previous chapter"), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := s.PreviewPayloadReview(ctx, review.ID, request); err == nil || !strings.Contains(err.Error(), "outside the new manifest") {
					t.Fatal(err)
				}
				if _, err := os.Stat(path); err != nil {
					t.Fatal("unmatched chapter lost", err)
				}
			} else {
				if err := os.WriteFile(preview.Operation.Files[0].DestinationPath, []byte("changed after preview"), 0600); err != nil {
					t.Fatal(err)
				}
				request.PreviewToken = preview.Fingerprint
				if _, err := s.ResolveImportReview(ctx, review.ID, request); !errors.Is(err, ErrImportReviewConflict) {
					t.Fatal("stale replacement applied", err)
				}
				refreshed, err := s.PreviewPayloadReview(ctx, review.ID, request)
				if err != nil {
					t.Fatal(err)
				}
				request.PreviewToken = refreshed.Fingerprint
				result, err := s.ResolveImportReview(ctx, review.ID, request)
				if err != nil || result.Review.Status != "imported" || result.Import == nil || !result.Import.Replaced {
					t.Fatal(result, err)
				}
			}
		})
	}
}

func TestCompletedReplacementRechecksDirectoryAfterPlanning(t *testing.T) {
	s, op, _, _ := replacementFixture(t, true)
	extra := filepath.Join(filepath.Dir(op.Files[0].DestinationPath), "appeared-after-preview.mp3")
	if err := os.WriteFile(extra, []byte("retain this chapter"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runImportOperation(context.Background(), op); err == nil || !strings.Contains(err.Error(), "outside the new manifest") {
		t.Fatal(err)
	}
	for _, file := range op.Files {
		previous := file
		previous.SHA256, previous.SizeBytes = file.PreviousSHA256, file.PreviousSizeBytes
		if err := verifyManifestPath(file.DestinationPath, previous); err != nil {
			t.Fatal("old book changed before layout review", err)
		}
	}
}

func TestCompletedReplacementRestoresMissingTrackedDestination(t *testing.T) {
	s, planned, original, download := replacementFixture(t, false)
	ctx := context.Background()
	if _, err := s.store.db.Exec(`delete from import_operations where id=$1`, planned.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(planned.Files[0].DestinationPath); err != nil {
		t.Fatal(err)
	}
	payload, err := s.inspectDownloadPayload(ctx, download)
	if err != nil {
		t.Fatal(err)
	}
	op, _, err := s.planPayload(ctx, payload, ImportRequest{WantedID: wantedIDFromTags(download.Tags), ImportMode: "copy", ConflictAction: "replace"}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if op.Files[0].PreviousPath != "" {
		t.Fatal("invented previous bytes", op.Files[0])
	}
	op, err = s.store.planOperation(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.runImportOperation(ctx, op)
	if err != nil || out.File.ID != original[0].ID || out.File.Title != "Manual title" {
		t.Fatal(out, err)
	}
}
