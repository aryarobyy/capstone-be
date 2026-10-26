package area

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"capstone-be/internal/testutil"

	_ "github.com/lib/pq"
)

// closedDB returns a *sql.DB whose every call fails, without needing a server.
func closedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	return db
}

func TestAreaRepository_ClosedDB(t *testing.T) {
	repo := NewAreaRepository(closedDB(t))
	if _, err := repo.Exists(context.Background(), 1); err == nil {
		t.Fatal("Exists: expected error")
	}
	if _, err := repo.FindByID(context.Background(), 1); err == nil || errors.Is(err, ErrAreaNotFound) {
		t.Fatalf("FindByID: expected db error, got %v", err)
	}
}

func TestAreaRepository_DB(t *testing.T) {
	db := testutil.OpenTestDB(t)
	repo := NewAreaRepository(db)
	ctx := context.Background()

	testutil.Exec(t, db, `INSERT INTO users(id,name,email,password) VALUES(7,'o','o@x.io','x')`)
	testutil.Exec(t, db, `INSERT INTO areas(id,farm_id,owner_id,name,icon) VALUES(1,3,7,'North','leaf')`)
	testutil.Exec(t, db, `INSERT INTO areas(id,name) VALUES(2,'Bare')`)

	if ok, err := repo.Exists(ctx, 1); err != nil || !ok {
		t.Fatalf("Exists(1) = %v, %v", ok, err)
	}
	if ok, err := repo.Exists(ctx, 99); err != nil || ok {
		t.Fatalf("Exists(99) = %v, %v", ok, err)
	}

	a, err := repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != 1 || a.FarmID != 3 || a.OwnerID != 7 || a.Name != "North" || a.Icon != "leaf" || a.CreatedAt.IsZero() {
		t.Fatalf("unexpected area %+v", a)
	}

	// NULL owner_id / icon are coalesced.
	b, err := repo.FindByID(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if b.OwnerID != 0 || b.Icon != "" || b.FarmID != 0 {
		t.Fatalf("unexpected area %+v", b)
	}

	if _, err := repo.FindByID(ctx, 99); !errors.Is(err, ErrAreaNotFound) {
		t.Fatalf("expected ErrAreaNotFound, got %v", err)
	}
}
