// Package backups implements pg_dump-based database backups (M6.6): create
// custom-format dumps into LIBRARRY_BACKUP_DIR, list/delete them, and prune
// to a retention count from the scheduled backup task. Restore stays a
// documented-manual pg_restore.
package backups

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ErrUnavailable marks installs that cannot back up (no database URL or no
// pg_dump binary). HTTP handlers map it to 501.
var ErrUnavailable = errors.New("backups are unavailable")

// ErrNotFound marks unknown backup names.
var ErrNotFound = errors.New("backup not found")

// ErrInvalidName marks names that fail sanitization.
var ErrInvalidName = errors.New("invalid backup name")

var backupNamePattern = regexp.MustCompile(`^librarry-\d{8}-\d{6}\.dump$`)

// Backup is the API-facing shape of one dump file.
type Backup struct {
	Name      string    `json:"name"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
}

// Runner executes pg_dump; an interface so tests can fake the binary.
type Runner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args []string, env []string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (execRunner) Run(ctx context.Context, name string, args []string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

type Service struct {
	dir         string
	databaseURL string
	runner      Runner
	logger      *slog.Logger
	now         func() time.Time
}

type Options struct {
	Dir         string
	DatabaseURL string
	Runner      Runner
	Logger      *slog.Logger
}

func NewService(options Options) *Service {
	if options.Runner == nil {
		options.Runner = execRunner{}
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	return &Service{
		dir:         strings.TrimSpace(options.Dir),
		databaseURL: strings.TrimSpace(options.DatabaseURL),
		runner:      options.Runner,
		logger:      options.Logger,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

// Available reports whether backups can run at all (a database is configured).
func (s *Service) Available() bool {
	return s != nil && s.databaseURL != "" && s.dir != ""
}

// ValidBackupName sanitizes user-supplied names: basename-only and matching
// the librarry-YYYYMMDD-HHMMSS.dump pattern.
func ValidBackupName(name string) bool {
	if name != filepath.Base(name) || strings.Contains(name, "..") {
		return false
	}
	return backupNamePattern.MatchString(name)
}

// Create runs pg_dump (custom format) into the backup directory.
func (s *Service) Create(ctx context.Context) (Backup, error) {
	if !s.Available() {
		return Backup{}, fmt.Errorf("%w: no database configured", ErrUnavailable)
	}
	if _, err := s.runner.LookPath("pg_dump"); err != nil {
		return Backup{}, fmt.Errorf("%w: pg_dump is not installed", ErrUnavailable)
	}
	args, env, err := pgDumpConnectionArgs(s.databaseURL)
	if err != nil {
		return Backup{}, fmt.Errorf("%w: %s", ErrUnavailable, err.Error())
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return Backup{}, err
	}
	name := "librarry-" + s.now().Format("20060102-150405") + ".dump"
	path := filepath.Join(s.dir, name)
	args = append([]string{"--format=custom", "--file", path}, args...)
	// PGPASSWORD travels via the child env only; it is never logged.
	output, err := s.runner.Run(ctx, "pg_dump", args, env)
	if err != nil {
		_ = os.Remove(path)
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return Backup{}, errors.New("pg_dump failed: " + message)
	}
	info, err := os.Stat(path)
	if err != nil {
		return Backup{}, err
	}
	s.logger.Info("database backup created", "name", name, "size_bytes", info.Size())
	return Backup{Name: name, SizeBytes: info.Size(), CreatedAt: info.ModTime().UTC()}, nil
}

// List returns the stored backups, newest first.
func (s *Service) List() ([]Backup, error) {
	if s == nil || s.dir == "" {
		return []Backup{}, nil
	}
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Backup{}, nil
	}
	if err != nil {
		return nil, err
	}
	backups := make([]Backup, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !ValidBackupName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, Backup{Name: entry.Name(), SizeBytes: info.Size(), CreatedAt: info.ModTime().UTC()})
	}
	sort.Slice(backups, func(i, j int) bool {
		if backups[i].Name != backups[j].Name {
			return backups[i].Name > backups[j].Name
		}
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})
	return backups, nil
}

// Delete removes one backup by (sanitized) name.
func (s *Service) Delete(name string) error {
	if s == nil || s.dir == "" {
		return ErrNotFound
	}
	name = filepath.Base(strings.TrimSpace(name))
	if !ValidBackupName(name) {
		return ErrInvalidName
	}
	err := os.Remove(filepath.Join(s.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}

// Prune deletes the oldest backups beyond the retention count and returns how
// many were removed.
func (s *Service) Prune(retention int) (int, error) {
	if retention <= 0 {
		return 0, nil
	}
	backups, err := s.List()
	if err != nil {
		return 0, err
	}
	if len(backups) <= retention {
		return 0, nil
	}
	removed := 0
	var firstErr error
	// List is newest-first; everything past the retention window goes.
	for _, backup := range backups[retention:] {
		if err := s.Delete(backup.Name); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		removed++
	}
	return removed, firstErr
}

// pgDumpConnectionArgs parses a postgres URL into pg_dump flags plus child
// process env (PGPASSWORD/PGSSLMODE stay out of argv and logs).
func pgDumpConnectionArgs(databaseURL string) ([]string, []string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return nil, nil, errors.New("database url is not parseable")
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return nil, nil, errors.New("database url must be postgres://")
	}
	dbName := strings.TrimPrefix(parsed.Path, "/")
	if dbName == "" {
		return nil, nil, errors.New("database url is missing a database name")
	}
	args := []string{"--dbname", dbName}
	if host := parsed.Hostname(); host != "" {
		args = append(args, "--host", host)
	}
	if port := parsed.Port(); port != "" {
		args = append(args, "--port", port)
	}
	var env []string
	if parsed.User != nil {
		if username := parsed.User.Username(); username != "" {
			args = append(args, "--username", username)
		}
		if password, ok := parsed.User.Password(); ok && password != "" {
			env = append(env, "PGPASSWORD="+password)
		}
	}
	if sslmode := parsed.Query().Get("sslmode"); sslmode != "" {
		env = append(env, "PGSSLMODE="+sslmode)
	}
	return args, env, nil
}
