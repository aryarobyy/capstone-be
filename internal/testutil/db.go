// Package testutil provides a disposable PostgreSQL database for integration tests.
package testutil

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// OpenTestDB creates a fresh database with every migration applied and drops it
// when the test ends. TEST_DB_DSN must point at a throwaway PostgreSQL server
// (key=value DSN with CREATE DATABASE rights); the test is skipped when unset.
func OpenTestDB(t testing.TB) *sql.DB {
	t.Helper()
	admin := os.Getenv("TEST_DB_DSN")
	if admin == "" {
		t.Skip("TEST_DB_DSN not set")
	}
	adminDB, err := sql.Open("postgres", admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := adminDB.Ping(); err != nil {
		adminDB.Close()
		t.Fatalf("test database unreachable: %v", err)
	}
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	name := "test_" + hex.EncodeToString(buf[:])
	if _, err := adminDB.Exec(`CREATE DATABASE ` + name); err != nil {
		adminDB.Close()
		t.Fatalf("create test database: %v", err)
	}
	db, err := sql.Open("postgres", WithDBName(admin, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		_, _ = adminDB.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`)
		adminDB.Close()
	})
	files, _ := filepath.Glob(filepath.Join(MigrationsDir(), "*.up.sql"))
	if len(files) == 0 {
		t.Fatalf("no migrations in %s", MigrationsDir())
	}
	sort.Strings(files)
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(f), err)
		}
	}
	return db
}

// WithDBName returns a key=value DSN with dbname replaced (or added).
func WithDBName(dsn, name string) string {
	parts := strings.Fields(dsn)
	for i, p := range parts {
		if strings.HasPrefix(p, "dbname=") {
			parts[i] = "dbname=" + name
			return strings.Join(parts, " ")
		}
	}
	return dsn + " dbname=" + name
}

// MigrationsDir returns the absolute path of the repository's migrations folder.
func MigrationsDir() string {
	_, self, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(self), "..", "..", "migrations")
}

// Exec runs a statement and fails the test on error.
func Exec(t testing.TB, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// Count scans a single integer from query.
func Count(t testing.TB, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}
