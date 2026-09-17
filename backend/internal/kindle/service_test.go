package kindle

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func validSettings() Settings {
	return Settings{Enabled: true, Host: "smtp.example.org", Port: 465, TLSMode: "implicit", Username: "sender", Password: "fixture-secret", From: "books@example.org", FromName: "Library", Recipient: "reader@kindle.com"}
}
func epub() []byte {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, _ := z.Create("mimetype")
	w.Write([]byte("application/epub+zip"))
	w, _ = z.Create("META-INF/container.xml")
	w.Write([]byte("fixture"))
	z.Close()
	return b.Bytes()
}
func TestSettingsValidation(t *testing.T) {
	for _, change := range []func(*Settings){func(s *Settings) { s.Recipient = "someone@example.org" }, func(s *Settings) { s.Recipient = "reader@kindle.com.evil.org" }, func(s *Settings) { s.From = "sender@example.org\r\nBcc: other@example.org" }, func(s *Settings) { s.TLSMode = "none" }, func(s *Settings) { s.Password = "" }, func(s *Settings) { s.Port = 0 }} {
		s := validSettings()
		change(&s)
		if s.Validate() == nil {
			t.Fatal("accepted invalid settings")
		}
	}
	s := validSettings()
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	r := s.Redacted()
	if r.Password != "" || !r.PasswordConfigured {
		t.Fatal("password leaked")
	}
}
func TestReadDocumentGuards(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "book.epub")
	data := epub()
	os.WriteFile(p, data, 0600)
	hash := sha256.Sum256(data)
	if _, e := readDocument(root, "book.epub", ".epub", int64(len(data)), hex.EncodeToString(hash[:])); e != nil {
		t.Fatal(e)
	}
	for _, test := range []struct {
		name string
		size int64
		sum  string
	}{{"book.epub", 999, ""}, {"book.epub", 0, strings.Repeat("0", 64)}, {"../outside.epub", 0, ""}} {
		if _, e := readDocument(root, test.name, ".epub", test.size, test.sum); e == nil {
			t.Fatal("accepted unsafe/changed file", test)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside.epub")
	os.WriteFile(outside, data, 0600)
	os.Symlink(outside, filepath.Join(root, "escape.epub"))
	if _, e := readDocument(root, "escape.epub", ".epub", 0, ""); e == nil {
		t.Fatal("symlink escaped root")
	}
	os.WriteFile(p, []byte("not an epub"), 0600)
	if _, e := readDocument(root, "book.epub", ".epub", 0, ""); e == nil {
		t.Fatal("invalid EPUB accepted")
	}
	f, _ := os.Create(p)
	f.Truncate(MaxFileBytes + 1)
	f.Close()
	if _, e := readDocument(root, "book.epub", ".epub", 0, ""); e == nil {
		t.Fatal("oversized attachment accepted")
	}
}
func TestSettingsPersistRedactAndClear(t *testing.T) {
	s := New(testdb.Open(t), validSettings(), t.TempDir())
	ctx := context.Background()
	v := validSettings()
	if _, e := s.SaveSettings(ctx, v, false); e != nil {
		t.Fatal(e)
	}
	v.Password = ""
	v.FromName = "New name"
	saved, e := s.SaveSettings(ctx, v, false)
	if e != nil || saved.Password != "fixture-secret" {
		t.Fatal(saved.Redacted(), e)
	}
	v.Enabled = false
	saved, e = s.SaveSettings(ctx, v, true)
	if e != nil || saved.Password != "" {
		t.Fatal("clear failed", e)
	}
	got, e := s.Settings(ctx)
	if e != nil || got.Password != "" || got.Enabled {
		t.Fatal("env resurrected cleared password", e)
	}
}
func TestDurableSendIdempotencyAndCrash(t *testing.T) {
	db := testdb.Open(t)
	s := New(db, validSettings(), t.TempDir())
	ctx := context.Background()
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	s.send = func(context.Context, Settings, Message) (string, string) {
		calls.Add(1)
		close(entered)
		<-release
		return "accepted", "Accepted by email server"
	}
	done := make(chan error, 1)
	go func() { _, e := s.Send(ctx, "", "", "first-request-123456"); done <- e }()
	<-entered
	replay, e := s.Send(ctx, "", "", "first-request-123456")
	if e != nil || replay.State != "sending" {
		t.Fatal(replay, e)
	}
	if _, e = s.Send(ctx, "", "", "second-request-12345"); e != ErrConflict {
		t.Fatal("concurrent send not blocked", e)
	}
	close(release)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	replay, e = s.Send(ctx, "", "", "first-request-123456")
	if e != nil || replay.State != "accepted" || calls.Load() != 1 {
		t.Fatal(replay, e, calls.Load())
	}
	restarted := New(db, validSettings(), t.TempDir())
	restarted.send = func(context.Context, Settings, Message) (string, string) {
		t.Fatal("restart replay sent twice")
		return "failed", ""
	}
	if d, err := restarted.Send(ctx, "", "", "first-request-123456"); err != nil || d.State != "accepted" {
		t.Fatal("restart lost attempt", d, err)
	}
	if _, e = s.Send(ctx, "wrong", "wrong", "first-request-123456"); e != ErrConflict {
		t.Fatal("request collision accepted", e)
	}
	_, e = db.Exec(`insert into kindle_deliveries(request_id,wanted_id,file_id,recipient,sender,title,state,created_at) values('interrupted-12345678','','','reader@kindle.com','books@example.org','test','sending',now()-interval '3 minutes')`)
	if e != nil {
		t.Fatal(e)
	}
	history, e := s.History(ctx, "")
	if e != nil || history[1].State != "unknown" {
		t.Fatal(history, e)
	}
	replay, e = s.Send(ctx, "", "", "interrupted-12345678")
	if e != nil || replay.State != "unknown" || calls.Load() != 1 {
		t.Fatal("uncertain request resent", replay, e)
	}
}
func TestDocumentSelection(t *testing.T) {
	db := testdb.Open(t)
	root := t.TempDir()
	s := New(db, validSettings(), root)
	ctx := context.Background()
	data := epub()
	path := filepath.Join(root, "book.epub")
	os.WriteFile(path, data, 0600)
	var wanted, file string
	if e := db.QueryRow(`insert into wanted_items(wanted_format,title,author_name,status) values('ebook','Fixture','Author','imported') returning id::text`).Scan(&wanted); e != nil {
		t.Fatal(e)
	}
	if e := db.QueryRow(`insert into files(media_format,path,title,import_status,presence_state,size_bytes) values('ebook',$1,'Fixture','imported','present',$2) returning id::text`, path, len(data)).Scan(&file); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(`insert into file_wanted_links(file_id,wanted_item_id) values($1,$2)`, file, wanted); e != nil {
		t.Fatal(e)
	}
	m, e := s.document(ctx, wanted, file)
	if e != nil || !bytes.Equal(m.Data, data) {
		t.Fatal(e)
	}
	if _, e = s.document(ctx, "wrong-book", file); e == nil {
		t.Fatal("cross-book file accepted")
	}
	db.Exec(`update wanted_items set status='removed' where id=$1`, wanted)
	if _, e = s.document(ctx, wanted, file); e == nil {
		t.Fatal("removed book accepted")
	}
	db.Exec(`update wanted_items set status='imported' where id=$1`, wanted)
	db.Exec(`update files set presence_state='missing' where id=$1`, file)
	if _, e = s.document(ctx, wanted, file); e == nil {
		t.Fatal("missing file accepted")
	}
}

func TestManualRetryAndUnknownPreservation(t *testing.T) {
	db := testdb.Open(t)
	s := New(db, validSettings(), t.TempDir())
	ctx := context.Background()
	calls := 0
	s.send = func(context.Context, Settings, Message) (string, string) {
		calls++
		return "unknown", "SMTP acknowledgement lost"
	}
	first, e := s.Send(ctx, "", "", "uncertain-attempt-123")
	if e != nil || first.State != "unknown" {
		t.Fatal(first, e)
	}
	replay, e := s.Send(ctx, "", "", "uncertain-attempt-123")
	if e != nil || replay.ID != first.ID || calls != 1 {
		t.Fatal("unknown request retried", e)
	}
	s.send = func(context.Context, Settings, Message) (string, string) { calls++; return "accepted", "Accepted" }
	next, e := s.Send(ctx, "", "", "explicit-retry-123456")
	if e != nil || next.ID == first.ID || next.State != "accepted" || calls != 2 {
		t.Fatal(next, e)
	}
	history, e := s.History(ctx, "")
	if e != nil || len(history) != 2 || history[1].State != "unknown" {
		t.Fatal("history overwritten", history, e)
	}
}

func TestEnvironmentDefaults(t *testing.T) {
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("RESEND_API_KEY", "fixture-key")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_TLS_MODE", "starttls")
	t.Setenv("LIBRARRY_KINDLE_ENABLED", "false")
	s := FromEnv()
	if s.Enabled || s.Password != "fixture-key" || s.Port != 587 || s.TLSMode != "starttls" {
		t.Fatal("incorrect environment defaults")
	}
	t.Setenv("SMTP_PASSWORD", "override")
	if FromEnv().Password != "override" {
		t.Fatal("explicit SMTP password did not win")
	}
}
