package library

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/calibre"
)

type handoffCalibre struct {
	fakeCalibreImporter
	adds, fields, starts, polls          int
	addErr, fieldsErr, startErr, pollErr error
	onAdd, onFields, onPoll              func()
	running                              bool
}

func (f *handoffCalibre) AddBook(ctx context.Context, r calibre.AddBookRequest) (calibre.AddBookResult, error) {
	f.adds++
	if f.onAdd != nil {
		f.onAdd()
	}
	return calibre.AddBookResult{ID: 42}, f.addErr
}
func (f *handoffCalibre) SetFields(ctx context.Context, r calibre.SetFieldsRequest) error {
	f.fields++
	f.setFieldsRequests = append(f.setFieldsRequests, r)
	if f.onFields != nil {
		f.onFields()
	}
	return f.fieldsErr
}
func (f *handoffCalibre) Convert(ctx context.Context, r calibre.ConvertRequest) (calibre.ConvertResult, error) {
	f.starts++
	if f.startErr != nil {
		return calibre.ConvertResult{}, f.startErr
	}
	return calibre.ConvertResult{Jobs: []calibre.ConvertJob{{OutputFormat: r.Settings.OutputFormat, JobID: 0}}}, nil
}
func (f *handoffCalibre) PollConversions(ctx context.Context, r calibre.PollConversionsRequest) ([]calibre.ConversionStatus, error) {
	f.polls++
	if f.onPoll != nil {
		f.onPoll()
	}
	if f.pollErr != nil {
		return nil, f.pollErr
	}
	return []calibre.ConversionStatus{{JobID: r.Jobs[0].JobID, OutputFormat: r.Jobs[0].OutputFormat, Running: f.running, OK: !f.running}}, nil
}
func (f *handoffCalibre) BookFormats(context.Context, calibre.Settings, int) ([]string, error) {
	return []string{"EPUB", "TXT"}, nil
}
func handoffFixture(t *testing.T, formats string) (*Service, *sql.DB, *handoffCalibre, ImportRequest, context.Context) {
	t.Helper()
	s, db, d, wantedID := operationFixture(t)
	root, err := s.store.CreateRootFolder(context.Background(), RootFolder{Name: "Calibre", Path: filepath.Join(t.TempDir(), "calibre"), MediaFormat: "ebook", IsDefault: true, Calibre: RootFolderCalibre{Enabled: true, Host: "calibre.test", Port: 8080, Username: "fixture", Password: "never-save-this", Library: "library", ConvertFormats: formats}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`update wanted_items set root_folder_id=$2 where id=$1`, wantedID, root.ID); err != nil {
		t.Fatal(err)
	}
	fake := &handoffCalibre{}
	s.calibre = fake
	source, err := filepath.EvalSymlinks(filepath.Join(d.SavePath, d.Name))
	if err != nil {
		t.Fatal(err)
	}
	return s, db, fake, ImportRequest{SourcePath: source, WantedID: wantedID, DownloadID: d.ID}, acquisition.WithDownloadClient(context.Background(), d.Client)
}
func handoffRow(t *testing.T, s *Service) calibreHandoffJournal {
	t.Helper()
	h, e := scanCalibreHandoff(s.store.db.QueryRow(`select ` + handoffColumns + ` from calibre_handoffs`))
	if e != nil {
		t.Fatal(e)
	}
	return h
}
func TestCalibreHandoffMetadataFailureRetainsAcknowledgement(t *testing.T) {
	s, db, f, r, ctx := handoffFixture(t, "")
	f.fieldsErr = errors.New("secret remote failure")
	out, err := s.Import(ctx, r)
	if err == nil || strings.Contains(err.Error(), "secret") || out.CalibreHandoffID == "" {
		t.Fatalf("%+v %v", out, err)
	}
	h := handoffRow(t, s)
	if h.Phase != "accepted" || h.BookID != 42 {
		t.Fatalf("%+v", h)
	}
	f.fieldsErr = nil
	out, err = s.Import(ctx, r)
	if err != nil || !out.Imported || f.adds != 1 {
		t.Fatalf("%+v %v adds=%d", out, err, f.adds)
	}
	if err = os.Remove(r.SourcePath); err != nil {
		t.Fatal(err)
	}
	out, err = s.Import(ctx, r)
	if err != nil || !out.Skipped || f.adds != 1 {
		t.Fatalf("replay %+v %v", out, err)
	}
	for q, n := range map[string]int{`select count(*) from files`: 1, `select count(*) from file_wanted_links`: 1, `select count(*) from file_download_links`: 1, `select count(*) from downloads where import_status='imported'`: 1, `select count(*) from history_events where event_type='book_imported'`: 1, `select count(*) from import_operations`: 0} {
		var count int
		if e := db.QueryRow(q).Scan(&count); e != nil || count != n {
			t.Fatalf("%s: %d %v", q, count, e)
		}
	}
	var raw string
	if err = db.QueryRow(`select plan::text||progress::text from calibre_handoffs`).Scan(&raw); err != nil || strings.Contains(raw, "never-save-this") {
		t.Fatal("journal contains credential", err)
	}
}
func TestCalibreHandoffUnknownUploadRequiresExplicitResolution(t *testing.T) {
	for _, lostSave := range []bool{false, true} {
		t.Run(map[bool]string{false: "lost response", true: "lost save"}[lostSave], func(t *testing.T) {
			s, db, f, r, ctx := handoffFixture(t, "")
			if lostSave {
				if _, e := db.Exec(`alter table calibre_handoffs add constraint inject_ack_failure check(phase<>'accepted')`); e != nil {
					t.Fatal(e)
				}
			} else {
				f.addErr = errors.New("ack lost")
			}
			if _, e := s.Import(ctx, r); e == nil {
				t.Fatal("expected failure")
			}
			h := handoffRow(t, s)
			if h.Phase != "uploading" {
				t.Fatal(h.Phase)
			}
			if lostSave {
				_, _ = db.Exec(`alter table calibre_handoffs drop constraint inject_ack_failure`)
			}
			f.addErr = nil
			for i := 0; i < 2; i++ {
				if _, e := s.Import(ctx, r); e == nil {
					t.Fatal("uncertain upload retried")
				}
			}
			if f.adds != 1 {
				t.Fatal(f.adds)
			}
			if _, e := s.ResolveCalibreHandoff(ctx, h.ID, CalibreHandoffResolution{Action: "attach-book", BookID: 42}); e == nil {
				t.Fatal("confirmation omitted")
			}
			if _, e := s.ResolveCalibreHandoff(ctx, h.ID, CalibreHandoffResolution{Action: "attach-book", BookID: 42, Confirm: true}); e != nil {
				t.Fatal(e)
			}
			if out, e := s.RetryCalibreHandoff(ctx, h.ID); e != nil || !out.Imported || f.adds != 1 {
				t.Fatalf("%+v %v adds=%d", out, e, f.adds)
			}
		})
	}
}
func TestCalibreHandoffConversionZeroAndConsumedStatus(t *testing.T) {
	s, db, f, r, ctx := handoffFixture(t, "TXT,PDF")
	f.running = true
	out, err := s.Import(ctx, r)
	if err != nil || !out.Skipped || f.starts != 2 {
		t.Fatalf("%+v %v starts=%d", out, err, f.starts)
	}
	h := handoffRow(t, s)
	if h.Conversions[0].JobID == nil || *h.Conversions[0].JobID != 0 {
		t.Fatal(h.Conversions)
	}
	f.running = false
	// Fail only after the first destructive terminal status response.
	f.onPoll = func() {
		_, e := db.Exec(`alter table calibre_handoffs add constraint inject_status_failure check(not(progress @> '[{"state":"done"}]'::jsonb))`)
		if e != nil {
			t.Fatal(e)
		}
		f.onPoll = nil
	}
	if _, err = s.RetryCalibreHandoff(ctx, h.ID); err == nil {
		t.Fatal("expected persistence failure")
	}
	h = handoffRow(t, s)
	if h.Conversions[0].State != "polling" {
		t.Fatal(h.Conversions)
	}
	polls := f.polls
	if _, err = s.RetryCalibreHandoff(ctx, h.ID); err == nil || f.polls != polls {
		t.Fatal("consumed status was polled again")
	}
	if _, err = db.Exec(`alter table calibre_handoffs drop constraint inject_status_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ResolveCalibreHandoff(ctx, h.ID, CalibreHandoffResolution{Action: "use-format", Format: "TXT", Confirm: true}); err != nil {
		t.Fatal(err)
	}
	out, err = s.RetryCalibreHandoff(ctx, h.ID)
	if err != nil || !out.Imported || f.starts != 2 || f.adds != 1 {
		t.Fatalf("%+v %v starts=%d adds=%d", out, err, f.starts, f.adds)
	}
}
func TestCalibreHandoffCommitFailureIsAtomicAndResumable(t *testing.T) {
	s, db, f, r, ctx := handoffFixture(t, "")
	if _, e := db.Exec(`alter table downloads add constraint inject_commit_failure check(import_status<>'imported')`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Import(ctx, r); e == nil {
		t.Fatal("expected commit failure")
	}
	for _, q := range []string{`select count(*) from files`, `select count(*) from history_events where event_type='book_imported'`} {
		var n int
		if e := db.QueryRow(q).Scan(&n); e != nil || n != 0 {
			t.Fatalf("%s %d %v", q, n, e)
		}
	}
	if _, e := db.Exec(`alter table downloads drop constraint inject_commit_failure`); e != nil {
		t.Fatal(e)
	}
	out, e := s.Import(ctx, r)
	if e != nil || !out.Imported || f.adds != 1 {
		t.Fatalf("%+v %v %d", out, e, f.adds)
	}
}
func TestCalibreHandoffTargetAndOwnerChanges(t *testing.T) {
	s, db, f, r, ctx := handoffFixture(t, "")
	f.fieldsErr = errors.New("retry")
	if _, e := s.Import(ctx, r); e == nil {
		t.Fatal("expected failure")
	}
	h := handoffRow(t, s)
	f.fieldsErr = nil
	if _, e := db.Exec(`update root_folders set calibre_host='another-server' where id=$1`, h.RootFolderID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Import(ctx, r); e == nil {
		t.Fatal("changed target accepted")
	}
	if f.adds != 1 {
		t.Fatal(f.adds)
	}
	if _, e := db.Exec(`update root_folders set calibre_host='calibre.test',calibre_password='rotated' where id=$1`, h.RootFolderID); e != nil {
		t.Fatal(e)
	}
	f.onFields = func() {
		_, e := db.Exec(`update wanted_items set title='Owner edit',updated_at=now() where id=$1`, r.WantedID)
		if e != nil {
			t.Fatal(e)
		}
		f.onFields = nil
	}
	if _, e := s.Import(ctx, r); e == nil {
		t.Fatal("owner race was not fenced")
	}
	out, e := s.Import(ctx, r)
	if e != nil || !out.Imported || out.File.Title != "Owner edit" || f.adds != 1 {
		t.Fatalf("%+v %v", out, e)
	}
}
func TestCalibreHandoffConcurrentRetryDoesNotSend(t *testing.T) {
	s, _, f, r, ctx := handoffFixture(t, "")
	f.onAdd = func() {
		h := handoffRow(t, s)
		if _, e := s.RetryCalibreHandoff(ctx, h.ID); e == nil {
			t.Error("concurrent retry accepted")
		}
	}
	out, e := s.Import(ctx, r)
	if e != nil || !out.Imported || f.adds != 1 {
		t.Fatalf("%+v %v adds=%d", out, e, f.adds)
	}
}

func TestCalibreHandoffExistingFilePreservesOwnerDataAndRejectsForeignLinks(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(map[bool]string{false: "owner metadata", true: "foreign book"}[foreign], func(t *testing.T) {
			s, db, f, r, ctx := handoffFixture(t, "")
			id := r.WantedID
			if foreign {
				if e := db.QueryRow(`insert into wanted_items(wanted_format,title) values('ebook','Other') returning id::text`).Scan(&id); e != nil {
					t.Fatal(e)
				}
			}
			record, e := s.store.UpsertFile(ctx, FileRecord{Path: r.SourcePath, Title: "Owner file title", AuthorName: "Owner author", MediaFormat: "ebook", Metadata: map[string]any{"wantedId": id, "notes": "Keep my note"}})
			if e != nil {
				t.Fatal(e)
			}
			out, e := s.Import(ctx, r)
			if foreign {
				if e == nil || f.adds != 0 {
					t.Fatalf("foreign upload %+v %v adds=%d", out, e, f.adds)
				}
				return
			}
			if e != nil || !out.Imported || out.File.ID != record.ID || out.File.Title != "Owner file title" || out.File.Metadata["notes"] != "Keep my note" {
				t.Fatalf("%+v %v", out, e)
			}
		})
	}
}
func TestCalibreHandoffScopedDownloadAndExplicitRetryUpload(t *testing.T) {
	s, db, f, r, ctx := handoffFixture(t, "")
	if _, e := db.Exec(`insert into downloads(client,external_id,category,save_path,state) values('Transmission','fixture','books','/fixture','pausedUP')`); e != nil {
		t.Fatal(e)
	}
	f.addErr = errors.New("ack lost")
	if _, e := s.Import(ctx, r); e == nil {
		t.Fatal("expected failure")
	}
	h := handoffRow(t, s)
	if _, e := s.Import(acquisition.WithDownloadClient(ctx, "Transmission"), r); e == nil {
		t.Fatal("other client adopted handoff")
	}
	if _, e := s.Import(context.Background(), r); e == nil {
		t.Fatal("ambiguous external ID adopted handoff")
	}
	if _, e := s.ResolveCalibreHandoff(ctx, h.ID, CalibreHandoffResolution{Action: "retry-upload", Confirm: true}); e != nil {
		t.Fatal(e)
	}
	f.addErr = nil
	out, e := s.RetryCalibreHandoff(ctx, h.ID)
	if e != nil || !out.Imported || f.adds != 2 {
		t.Fatalf("%+v %v %d", out, e, f.adds)
	}
	var changed int
	if e = db.QueryRow(`select count(*) from downloads where client='Transmission' and import_status='imported'`).Scan(&changed); e != nil || changed != 0 {
		t.Fatalf("other client changed %d %v", changed, e)
	}
}
func TestCalibreHandoffRechecksChangedBytesAndRetainsSources(t *testing.T) {
	s, _, f, r, ctx := handoffFixture(t, "")
	f.addErr = errors.New("ack lost")
	if _, e := s.Import(ctx, r); e == nil {
		t.Fatal("expected failure")
	}
	h := handoffRow(t, s)
	if _, e := s.ResolveCalibreHandoff(ctx, h.ID, CalibreHandoffResolution{Action: "retry-upload", Confirm: true}); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(r.SourcePath, []byte("different book bytes"), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := s.RetryCalibreHandoff(ctx, h.ID); e == nil || f.adds != 1 {
		t.Fatal("changed source uploaded")
	}
	if _, e := os.Stat(r.SourcePath); e != nil {
		t.Fatal(e)
	}
}

func TestCalibreHandoffBackgroundRefreshResumesAndNeverRepollsTerminal(t *testing.T) {
	s, _, f, r, ctx := handoffFixture(t, "TXT")
	f.running = true
	if out, e := s.Import(ctx, r); e != nil || !out.Skipped {
		t.Fatalf("%+v %v", out, e)
	}
	f.running = false
	result, e := s.RefreshCalibreConversions(ctx, CalibreConversionRefreshRequest{Limit: 1})
	if e != nil || result.Refreshed != 1 || f.polls != 2 {
		t.Fatalf("%+v %v polls=%d", result, e, f.polls)
	}
	h := handoffRow(t, s)
	if h.Phase != "committed" {
		t.Fatal(h.Phase)
	}
	result, e = s.RefreshCalibreConversions(ctx, CalibreConversionRefreshRequest{IDs: []string{h.FileID}, Force: true})
	if e != nil || result.Skipped != 1 || f.polls != 2 {
		t.Fatalf("%+v %v polls=%d", result, e, f.polls)
	}
}

func TestCalibreHandoffDeleteUsesBoundOriginalIdentity(t *testing.T) {
	s, _, f, r, ctx := handoffFixture(t, "")
	out, e := s.Import(ctx, r)
	if e != nil {
		t.Fatal(e)
	}
	foreign := out.File
	foreign.ID = "00000000-0000-0000-0000-000000000099"
	if e = s.applyCalibreDelete(ctx, foreign); e == nil || len(f.deleteRequests) != 0 {
		t.Fatal("unrelated file authorized a remote delete")
	}
	if e = s.applyCalibreDelete(ctx, out.File); e != nil || len(f.deleteRequests) != 1 || f.deleteRequests[0].IDs[0] != 42 || f.deleteRequests[0].Settings.Host != "calibre.test" {
		t.Fatalf("delete: %v %+v", e, f.deleteRequests)
	}
}
