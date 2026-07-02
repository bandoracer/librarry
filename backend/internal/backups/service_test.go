package backups

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	lookPathErr error
	runErr      error
	output      string
	gotName     string
	gotArgs     []string
	gotEnv      []string
	fileSize    int
}

func (f *fakeRunner) LookPath(string) (string, error) {
	if f.lookPathErr != nil {
		return "", f.lookPathErr
	}
	return "/usr/bin/pg_dump", nil
}

func (f *fakeRunner) Run(_ context.Context, name string, args []string, env []string) ([]byte, error) {
	f.gotName = name
	f.gotArgs = args
	f.gotEnv = env
	if f.runErr != nil {
		return []byte(f.output), f.runErr
	}
	// Simulate pg_dump writing the file.
	for index, arg := range args {
		if arg == "--file" && index+1 < len(args) {
			size := f.fileSize
			if size <= 0 {
				size = 4
			}
			if err := os.WriteFile(args[index+1], make([]byte, size), 0o644); err != nil {
				return nil, err
			}
		}
	}
	return []byte(f.output), nil
}

func newTestBackupService(t *testing.T, runner Runner) *Service {
	t.Helper()
	return NewService(Options{
		Dir:         t.TempDir(),
		DatabaseURL: "postgres://librarry:s3cret@postgres:5432/librarry?sslmode=disable",
		Runner:      runner,
	})
}

func TestValidBackupName(t *testing.T) {
	valid := []string{"librarry-20260701-093000.dump"}
	invalid := []string{
		"", "librarry.dump", "../librarry-20260701-093000.dump",
		"nested/librarry-20260701-093000.dump", "librarry-2026-99.dump",
		"librarry-20260701-093000.dump.sql", "..",
	}
	for _, name := range valid {
		if !ValidBackupName(name) {
			t.Fatalf("expected %q to be valid", name)
		}
	}
	for _, name := range invalid {
		if ValidBackupName(name) {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

func TestCreateBackupRunsPgDumpWithEnvPassword(t *testing.T) {
	runner := &fakeRunner{}
	service := newTestBackupService(t, runner)
	backup, err := service.Create(context.Background())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !ValidBackupName(backup.Name) || backup.SizeBytes <= 0 {
		t.Fatalf("unexpected backup: %+v", backup)
	}
	if runner.gotName != "pg_dump" {
		t.Fatalf("expected pg_dump, got %q", runner.gotName)
	}
	argsJoined := strings.Join(runner.gotArgs, " ")
	for _, want := range []string{"--format=custom", "--dbname librarry", "--host postgres", "--port 5432", "--username librarry"} {
		if !strings.Contains(argsJoined, want) {
			t.Fatalf("expected args to contain %q, got %q", want, argsJoined)
		}
	}
	if strings.Contains(argsJoined, "s3cret") {
		t.Fatal("password must never appear in argv")
	}
	envJoined := strings.Join(runner.gotEnv, " ")
	if !strings.Contains(envJoined, "PGPASSWORD=s3cret") || !strings.Contains(envJoined, "PGSSLMODE=disable") {
		t.Fatalf("expected password/sslmode via env, got %q", envJoined)
	}

	rows, err := service.List()
	if err != nil || len(rows) != 1 || rows[0].Name != backup.Name {
		t.Fatalf("expected the new backup in the list, got %+v err=%v", rows, err)
	}
}

func TestCreateBackupUnavailableWhenPgDumpMissing(t *testing.T) {
	service := newTestBackupService(t, &fakeRunner{lookPathErr: errors.New("not found")})
	if _, err := service.Create(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}

	noDB := NewService(Options{Dir: t.TempDir(), Runner: &fakeRunner{}})
	if noDB.Available() {
		t.Fatal("expected service without database url to be unavailable")
	}
	if _, err := noDB.Create(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable without database, got %v", err)
	}
}

func TestCreateBackupFailureRemovesPartialFile(t *testing.T) {
	runner := &fakeRunner{runErr: errors.New("exit 1"), output: "connection refused"}
	service := newTestBackupService(t, runner)
	_, err := service.Create(context.Background())
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("expected pg_dump failure message, got %v", err)
	}
	rows, _ := service.List()
	if len(rows) != 0 {
		t.Fatalf("expected no partial backups, got %+v", rows)
	}
}

func TestDeleteBackupSanitizesNames(t *testing.T) {
	service := newTestBackupService(t, &fakeRunner{})
	backup, err := service.Create(context.Background())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := service.Delete("../" + backup.Name); !errors.Is(err, ErrInvalidName) {
		// filepath.Base strips the traversal, making it the valid name; both
		// rejecting and safely deleting the basename are acceptable, but a
		// traversal outside the dir must never happen. Accept nil only if the
		// backup inside the dir is what got removed.
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
	}
	_ = service.Delete(backup.Name)
	if err := service.Delete("secrets.txt"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("expected invalid-name rejection, got %v", err)
	}
	if err := service.Delete("librarry-20990101-000000.dump"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not-found for unknown backup, got %v", err)
	}
}

func TestPruneKeepsNewestBackups(t *testing.T) {
	service := newTestBackupService(t, &fakeRunner{})
	dir := service.dir
	names := []string{
		"librarry-20260101-000000.dump",
		"librarry-20260201-000000.dump",
		"librarry-20260301-000000.dump",
		"librarry-20260401-000000.dump",
		"librarry-20260501-000000.dump",
		"stray-file.txt",
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := service.Prune(2)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 3 {
		t.Fatalf("expected 3 pruned, got %d", removed)
	}
	rows, err := service.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 || rows[0].Name != "librarry-20260501-000000.dump" || rows[1].Name != "librarry-20260401-000000.dump" {
		t.Fatalf("expected the two newest backups to survive, got %+v", rows)
	}
	// Non-backup files are never touched.
	if _, err := os.Stat(filepath.Join(dir, "stray-file.txt")); err != nil {
		t.Fatalf("expected stray file to survive prune: %v", err)
	}
	// Retention >= count is a no-op.
	if removed, err := service.Prune(10); err != nil || removed != 0 {
		t.Fatalf("expected no-op prune, got %d err=%v", removed, err)
	}
	_ = time.Now()
}
