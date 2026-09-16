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

func Open(t *testing.T) *sql.DB {
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
	if err := database.ApplyMigrations(context.Background(), db, filepath.Join(filepath.Dir(file), "../../migrations")); err != nil {
		t.Fatal(err)
	}
	return db
}
