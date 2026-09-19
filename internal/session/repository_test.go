package session

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Use a disposable database: this test creates and drops its own schema.
func TestSessionLifecyclePostgres(t *testing.T) {
	dsn := os.Getenv("SESSION_TEST_DSN")
	if dsn == "" {
		t.Skip("SESSION_TEST_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if _, err = db.Exec(`CREATE SCHEMA session_integration; SET search_path TO session_integration`); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(`DROP SCHEMA session_integration CASCADE`)
	for _, name := range []string{"000001_create_users_table.up.sql", "000008_create_notifications.up.sql", "000009_create_sessions.up.sql"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(content)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`INSERT INTO users(id,name,email,password) VALUES(1,'a','a@example.com','hash'),(2,'b','b@example.com','hash')`); err != nil {
		t.Fatal(err)
	}
	store := NewRepository(db)
	expiry := time.Now().Add(time.Hour)
	s1 := "00000000-0000-4000-8000-000000000001"
	s2 := "00000000-0000-4000-8000-000000000002"
	s3 := "00000000-0000-4000-8000-000000000003"
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(store.Create(ctx, s1, 1, "fcm", "web", expiry))
	must(store.Active(ctx, s1, 1))
	if !errors.Is(store.Active(ctx, s1, 2), ErrInactive) {
		t.Fatal("cross-user access accepted")
	}
	must(store.Create(ctx, s2, 1, "fcm", "web", expiry))
	must(store.Revoke(ctx, s1, 1))
	if !errors.Is(store.Active(ctx, s1, 1), ErrInactive) {
		t.Fatal("revoked session accepted")
	}
	must(store.Active(ctx, s2, 1))
	var count int
	must(db.QueryRow(`SELECT count(*) FROM device_tokens`).Scan(&count))
	if count != 1 {
		t.Fatal("logout removed device with another active session")
	}
	must(store.Create(ctx, s3, 2, "fcm", "web", expiry))
	if !errors.Is(store.Active(ctx, s2, 1), ErrInactive) {
		t.Fatal("account switch failed to revoke old session")
	}
	must(store.Active(ctx, s3, 2))
	must(store.BindDevice(ctx, s3, 2, "new-token", "web"))
	must(db.QueryRow(`SELECT count(*) FROM device_tokens`).Scan(&count))
	if count != 1 {
		t.Fatal("rotation left stale device")
	}
	must(store.Revoke(ctx, s3, 2))
	must(db.QueryRow(`SELECT count(*) FROM device_tokens`).Scan(&count))
	if count != 0 {
		t.Fatal("logout left token")
	}
	expired := "00000000-0000-4000-8000-000000000004"
	must(store.Create(ctx, expired, 1, "", "", time.Now().Add(-time.Minute)))
	if !errors.Is(store.Active(ctx, expired, 1), ErrInactive) {
		t.Fatal("expired session accepted")
	}
	// A failed session insert must also roll back device reassignment.
	if err := store.Create(ctx, expired, 1, "rollback-token", "web", expiry); err == nil {
		t.Fatal("expected duplicate session failure")
	}
	must(db.QueryRow(`SELECT count(*) FROM device_tokens WHERE token='rollback-token'`).Scan(&count))
	if count != 0 {
		t.Fatal("device persisted despite failed session transaction")
	}
	for _, name := range []string{"000009_create_sessions.down.sql", "000008_create_notifications.down.sql", "000001_create_users_table.down.sql"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		must(err)
		_, err = db.Exec(string(content))
		must(err)
	}
}
