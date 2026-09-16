// Package testdb creates a fresh, uniquely named database for integration tests.
// LIBRARRY_TEST_DATABASE_URL must point at a disposable Postgres instance whose
// user can create databases. Production DATABASE_URL is deliberately ignored.
package testdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/database"
)

func Open(t *testing.T) *sql.DB { return OpenThrough(t, "") }

// OpenThrough builds an old-schema fixture for append-only upgrade tests.
func OpenThrough(t *testing.T, lastMigration string) *sql.DB {
	t.Helper()
	raw := os.Getenv("LIBRARRY_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires disposable LIBRARRY_TEST_DATABASE_URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := database.Open(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	name := "librarry_test_" + hex.EncodeToString(random[:])
	if _, err := admin.ExecContext(context.Background(), "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})
	u.Path = "/" + name
	db, err := database.Open(context.Background(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, file, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(file), "../../migrations")
	if lastMigration != "" {
		staged := t.TempDir()
		entries, err := os.ReadDir(migrationsDir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && entry.Name() <= lastMigration {
				raw, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(staged, entry.Name()), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		migrationsDir = staged
	}
	if err := database.ApplyMigrations(context.Background(), db, migrationsDir); err != nil {
		t.Fatal(err)
	}
	return db
}

// SeedRange builds large fixtures in bounded transactions. File path guards take
// transaction-scoped advisory locks; seeding 10k paths in one statement can exhaust
// the shared lock pool while other test packages use the same disposable server.
// The query accepts inclusive integer range bounds as $1 and $2.
func SeedRange(t *testing.T, db *sql.DB, total int, query string) {
	t.Helper()
	for first := 1; first <= total; first += 500 {
		last := first + 499
		if last > total {
			last = total
		}
		if _, err := db.Exec(query, first, last); err != nil {
			t.Fatalf("seed rows %d–%d: %v", first, last, err)
		}
	}
}
