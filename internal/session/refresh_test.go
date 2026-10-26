package session

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"capstone-be/internal/testutil"
)

const (
	sidA = "00000000-0000-4000-8000-0000000000a1"
	sidB = "00000000-0000-4000-8000-0000000000b1"
	sidC = "00000000-0000-4000-8000-0000000000c1"
	inst = "00000000-0000-4000-8000-00000000f001"
)

func TestNewRefresh(t *testing.T) {
	raw, hash, err := NewRefresh()
	if err != nil || len(raw) != 43 || !bytes.Equal(hash, HashRefresh(raw)) || len(hash) != 32 {
		t.Fatalf("raw=%q hash=%x err=%v", raw, hash, err)
	}
	other, _, _ := NewRefresh()
	if other == raw {
		t.Fatal("refresh tokens repeat")
	}
}

func seedUsers(t *testing.T) (*sql.DB, Store) {
	t.Helper()
	db := testutil.OpenTestDB(t)
	testutil.Exec(t, db, `INSERT INTO users(id,name,email,password) VALUES(1,'a','a@example.com','hash'),(2,'b','b@example.com','hash')`)
	return db, NewRepository(db)
}

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRefreshRotateAndRevoke(t *testing.T) {
	db, store := seedUsers(t)
	ctx := context.Background()
	raw, hash, _ := NewRefresh()
	mustOK(t, store.CreateRefresh(ctx, sidA, 1, hash))
	if err := store.CreateRefresh(ctx, sidB, 1, hash); err == nil {
		t.Fatal("duplicate refresh hash accepted")
	}
	if err := store.CreateRefresh(ctx, sidB, 99, nil); err == nil {
		t.Fatal("unknown user accepted")
	}
	_, next, _ := NewRefresh()
	sid, user, err := store.Rotate(ctx, HashRefresh(raw), next)
	if err != nil || sid != sidA || user != 1 {
		t.Fatalf("sid=%s user=%d err=%v", sid, user, err)
	}
	if _, _, err := store.Rotate(ctx, HashRefresh(raw), []byte("x")); !errors.Is(err, ErrInactive) {
		t.Fatalf("reused refresh token: %v", err)
	}
	mustOK(t, store.RevokeRefresh(ctx, []byte("unknown")))
	mustOK(t, store.RevokeRefresh(ctx, next))
	if !errors.Is(store.Active(ctx, sidA, 1), ErrInactive) {
		t.Fatal("revoked refresh session still active")
	}
	if _, _, err := store.Rotate(ctx, next, []byte("y")); !errors.Is(err, ErrInactive) {
		t.Fatalf("revoked session rotated: %v", err)
	}
	// Expired sessions cannot be rotated.
	_, expHash, _ := NewRefresh()
	testutil.Exec(t, db, `INSERT INTO sessions(id,user_id,expires_at,refresh_hash) VALUES($1,1,NOW()-INTERVAL '1 minute',$2)`, sidC, expHash)
	if _, _, err := store.Rotate(ctx, expHash, []byte("z")); !errors.Is(err, ErrInactive) {
		t.Fatalf("expired session rotated: %v", err)
	}
}

func TestBindInstallation(t *testing.T) {
	db, store := seedUsers(t)
	ctx := context.Background()
	if err := store.BindInstallation(ctx, sidA, 1, "tok", "web", inst); !errors.Is(err, ErrInactive) {
		t.Fatalf("missing session bound: %v", err)
	}
	if err := store.BindInstallation(ctx, sidA, 99, "tok", "web", inst); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown user: %v", err)
	}
	mustOK(t, store.CreateRefresh(ctx, sidA, 1, []byte("a")))
	if err := store.BindInstallation(ctx, sidA, 1, "tok", "web", "not-a-uuid"); err == nil {
		t.Fatal("invalid installation id accepted")
	}
	if err := store.BindInstallation(ctx, sidA, 1, "tok", "bogus", inst); err == nil {
		t.Fatal("invalid device type accepted")
	}
	// New installation.
	mustOK(t, store.BindInstallation(ctx, sidA, 1, "tok", "web", inst))
	if testutil.Count(t, db, `SELECT count(*) FROM device_tokens WHERE installation_id=$1 AND token='tok' AND user_id=1`, inst) != 1 {
		t.Fatal("installation not stored")
	}
	if testutil.Count(t, db, `SELECT count(*) FROM sessions s JOIN device_tokens d ON d.id=s.device_id WHERE s.id=$1`, sidA) != 1 {
		t.Fatal("session not linked to device")
	}
	// Same installation, new token; an older row already holding that token is retired.
	mustOK(t, store.Create(ctx, sidB, 1, "tok2", "android", time.Now().Add(time.Hour)))
	mustOK(t, store.BindInstallation(ctx, sidA, 1, "tok2", "ios", inst))
	if testutil.Count(t, db, `SELECT count(*) FROM device_tokens`) != 1 || testutil.Count(t, db, `SELECT count(*) FROM device_tokens WHERE token='tok2' AND device_type='ios' AND installation_id=$1`, inst) != 1 {
		t.Fatal("stale token row not replaced")
	}
	if !errors.Is(store.Active(ctx, sidB, 1), ErrInactive) {
		t.Fatal("session of retired registration still active")
	}
	// Account switch on the same installation revokes the previous user's sessions.
	mustOK(t, store.CreateRefresh(ctx, sidC, 2, []byte("c")))
	mustOK(t, store.BindInstallation(ctx, sidC, 2, "tok2", "ios", inst))
	if !errors.Is(store.Active(ctx, sidA, 1), ErrInactive) {
		t.Fatal("previous account session survived switch")
	}
	mustOK(t, store.Active(ctx, sidC, 2))
}

func TestCreateBindRemoveErrors(t *testing.T) {
	db, store := seedUsers(t)
	ctx := context.Background()
	exp := time.Now().Add(time.Hour)
	if err := store.Create(ctx, sidA, 99, "", "", exp); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown user: %v", err)
	}
	if err := store.Create(ctx, sidA, 1, "tok", "bogus", exp); err == nil {
		t.Fatal("invalid device type accepted")
	}
	if !errors.Is(store.Revoke(ctx, sidA, 1), ErrInactive) {
		t.Fatal("revoking missing session succeeded")
	}
	if !errors.Is(store.BindDevice(ctx, sidA, 1, "tok", "web"), ErrInactive) {
		t.Fatal("bind on missing session succeeded")
	}
	mustOK(t, store.Create(ctx, sidA, 1, "", "", exp))
	if err := store.BindDevice(ctx, sidA, 1, "tok", "bogus"); err == nil {
		t.Fatal("invalid device type accepted")
	}
	mustOK(t, store.Revoke(ctx, sidA, 1)) // no device attached
	mustOK(t, store.Create(ctx, sidB, 1, "tok", "web", exp))
	mustOK(t, store.BindDevice(ctx, sidB, 1, "tok", "web")) // same device id: nothing to delete
	mustOK(t, store.RemoveDevice(ctx, sidB, 1, "other"))
	if testutil.Count(t, db, `SELECT count(*) FROM device_tokens`) != 1 {
		t.Fatal("wrong token removed")
	}
	mustOK(t, store.RemoveDevice(ctx, sidB, 1, "tok"))
	if testutil.Count(t, db, `SELECT count(*) FROM device_tokens`) != 0 {
		t.Fatal("device not removed")
	}
	if err := store.RemoveDevice(ctx, sidB, 99, "tok"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown user: %v", err)
	}
}

func TestCleanup(t *testing.T) {
	db, store := seedUsers(t)
	ctx := context.Background()
	exp := time.Now().Add(time.Hour)
	mustOK(t, store.Create(ctx, sidA, 1, "live", "web", exp))
	testutil.Exec(t, db, `INSERT INTO device_tokens(user_id,token,device_type) VALUES(1,'orphan','web')`)
	testutil.Exec(t, db, `INSERT INTO device_tokens(user_id,token,device_type,updated_at) VALUES(2,'stale','web',NOW()-INTERVAL '31 days')`)
	testutil.Exec(t, db, `INSERT INTO sessions(id,user_id,expires_at) VALUES($1,1,NOW()-INTERVAL '8 days')`, sidB)
	testutil.Exec(t, db, `INSERT INTO sessions(id,user_id,expires_at,revoked_at) VALUES($1,1,NOW()+INTERVAL '1 day',NOW()-INTERVAL '8 days')`, sidC)
	mustOK(t, store.Cleanup(ctx))
	if n := testutil.Count(t, db, `SELECT count(*) FROM device_tokens`); n != 1 {
		t.Fatalf("device_tokens=%d, want only live", n)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM sessions`); n != 1 {
		t.Fatalf("sessions=%d, want only active", n)
	}
}

// A closed pool exercises the generic database-error branches.
func TestClosedDatabaseErrors(t *testing.T) {
	db, store := seedUsers(t)
	db.Close()
	ctx := context.Background()
	exp := time.Now().Add(time.Hour)
	for name, err := range map[string]error{
		"CreateRefresh":    store.CreateRefresh(ctx, sidA, 1, []byte("a")),
		"RevokeRefresh":    store.RevokeRefresh(ctx, []byte("a")),
		"BindInstallation": store.BindInstallation(ctx, sidA, 1, "t", "web", inst),
		"Cleanup":          store.Cleanup(ctx),
		"Create":           store.Create(ctx, sidA, 1, "", "", exp),
		"Active":           store.Active(ctx, sidA, 1),
		"Revoke":           store.Revoke(ctx, sidA, 1),
		"BindDevice":       store.BindDevice(ctx, sidA, 1, "t", "web"),
		"RemoveDevice":     store.RemoveDevice(ctx, sidA, 1, "t"),
	} {
		if err == nil || errors.Is(err, ErrInactive) {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	if _, _, err := store.Rotate(ctx, []byte("a"), []byte("b")); err == nil || errors.Is(err, ErrInactive) {
		t.Fatalf("Rotate: err=%v", err)
	}
}
