package notification

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"capstone-be/internal/testutil"
	"github.com/google/uuid"
)

// closedDB returns a handle whose every call fails with "sql: database is closed".
func closedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	return db
}

func seedUser(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO users(name,email,password) VALUES('u',$1,'x') RETURNING id`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// seedDevice registers a device token with an active session and returns its id.
func seedDevice(t *testing.T, db *sql.DB, user int64, token string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO device_tokens(user_id,token,device_type) VALUES($1,$2,'android') RETURNING id`, user, token).Scan(&id); err != nil {
		t.Fatal(err)
	}
	testutil.Exec(t, db, `INSERT INTO sessions(id,user_id,device_id,expires_at) VALUES($1,$2,$3,NOW()+INTERVAL '1 day')`, uuid.NewString(), user, id)
	return id
}

func TestRepositoryDevices(t *testing.T) {
	db := testutil.OpenTestDB(t)
	r := NewRepository(db)
	ctx := context.Background()
	u1, u2 := seedUser(t, db, "a@x.io"), seedUser(t, db, "b@x.io")

	if err := r.Register(ctx, u1, "tok-a", "android"); err != nil {
		t.Fatal(err)
	}
	// Re-registering moves the token to the new owner and device type.
	if err := r.Register(ctx, u2, "tok-a", "ios"); err != nil {
		t.Fatal(err)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM device_tokens WHERE token='tok-a' AND user_id=$1 AND device_type='ios'`, u2); n != 1 {
		t.Fatalf("upsert rows = %d", n)
	}
	if err := r.Register(ctx, u1, "tok-b", "desktop"); err == nil {
		t.Fatal("invalid device_type must violate check constraint")
	}

	// Tokens only returns devices bound to an active session.
	seedDevice(t, db, u1, "tok-active")
	revoked := seedDevice(t, db, u1, "tok-revoked")
	testutil.Exec(t, db, `UPDATE sessions SET revoked_at=NOW() WHERE device_id=$1`, revoked)
	tokens, err := r.Tokens(ctx, u1)
	if err != nil || len(tokens) != 1 || tokens[0] != "tok-active" {
		t.Fatalf("Tokens = %v, %v", tokens, err)
	}

	if err := r.Remove(ctx, u1, "tok-a"); err != nil { // owned by u2, untouched
		t.Fatal(err)
	}
	if err := r.Remove(ctx, u2, "tok-a"); err != nil {
		t.Fatal(err)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM device_tokens WHERE token='tok-a'`); n != 0 {
		t.Fatalf("token not removed: %d", n)
	}
}

func TestRepositoryInbox(t *testing.T) {
	db := testutil.OpenTestDB(t)
	r := NewRepository(db)
	ctx := context.Background()
	user, other := seedUser(t, db, "a@x.io"), seedUser(t, db, "b@x.io")

	var ids []int64
	for i := 0; i < 3; i++ {
		n := &Notification{UserID: user, Title: "t", Body: "b", Data: map[string]string{"k": "v"}}
		if err := r.Create(ctx, n); err != nil || n.ID == 0 || n.CreatedAt.IsZero() {
			t.Fatalf("Create = %+v, %v", n, err)
		}
		ids = append(ids, n.ID)
	}
	if err := r.Create(ctx, &Notification{UserID: other, Title: "t", Body: "b"}); err != nil {
		t.Fatal(err)
	}

	list, err := r.List(ctx, user, 0)
	if err != nil || len(list) != 3 || list[0].ID != ids[2] || list[0].Data["k"] != "v" {
		t.Fatalf("List = %+v, %v", list, err)
	}
	list, err = r.List(ctx, user, ids[2])
	if err != nil || len(list) != 2 || list[0].ID != ids[1] {
		t.Fatalf("List before = %+v, %v", list, err)
	}

	if err := r.Read(ctx, user, ids[0]); err != nil {
		t.Fatal(err)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM notifications WHERE id=$1 AND is_read`, ids[0]); n != 1 {
		t.Fatal("not marked read")
	}
	if err := r.Read(ctx, other, ids[1]); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Read foreign = %v", err)
	}
}

func TestRepositoryEnqueueStatusMetrics(t *testing.T) {
	db := testutil.OpenTestDB(t)
	r := NewRepository(db)
	ctx := context.Background()
	user := seedUser(t, db, "a@x.io")
	seedDevice(t, db, user, "tok-1")
	seedDevice(t, db, user, "tok-2")
	stale := seedDevice(t, db, user, "tok-stale")
	testutil.Exec(t, db, `UPDATE device_tokens SET updated_at=NOW()-INTERVAL '31 days' WHERE id=$1`, stale)
	testutil.Exec(t, db, `INSERT INTO device_tokens(user_id,token,device_type) VALUES($1,'tok-nosession','web')`, user)

	p := PushNotificationPayload{IdempotencyKey: "evt-1", Title: "T", Body: "B", ImageURL: "http://img"}
	res, err := r.Enqueue(ctx, user, p)
	if err != nil || res.Status != "queued" || res.Notification.ID == 0 {
		t.Fatalf("Enqueue = %+v, %v", res, err)
	}
	if res.Notification.Data == nil {
		t.Fatal("nil data must be normalised to an empty map")
	}
	id := res.Notification.ID
	if n := testutil.Count(t, db, `SELECT count(*) FROM notification_deliveries WHERE notification_id=$1 AND status='pending'`, id); n != 2 {
		t.Fatalf("deliveries = %d, want 2 (active, fresh, with session)", n)
	}

	// Same key and payload is an idempotent replay.
	again, err := r.Enqueue(ctx, user, p)
	if err != nil || again.Notification.ID != id {
		t.Fatalf("replay = %+v, %v", again, err)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM notification_deliveries`); n != 2 {
		t.Fatalf("replay created deliveries: %d", n)
	}
	// Same key with different content conflicts.
	p.Body = "changed"
	if _, err := r.Enqueue(ctx, user, p); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflict = %v", err)
	}
	// Without a key every call creates a notification.
	if _, err := r.Enqueue(ctx, user, PushNotificationPayload{Title: "T", Body: "B"}); err != nil {
		t.Fatal(err)
	}
	if n := testutil.Count(t, db, `SELECT count(*) FROM notifications`); n != 2 {
		t.Fatalf("notifications = %d", n)
	}
	// Unknown user violates the foreign key.
	if _, err := r.Enqueue(ctx, user+999, PushNotificationPayload{Title: "T", Body: "B"}); err == nil {
		t.Fatal("expected FK error")
	}

	testutil.Exec(t, db, `UPDATE notification_deliveries SET status='sent' WHERE id=(SELECT min(id) FROM notification_deliveries)`)
	rows, err := r.Status(ctx, user, id)
	if err != nil || len(rows) != 2 || rows[0].Status != "sent" || rows[1].Status != "pending" {
		t.Fatalf("Status = %+v, %v", rows, err)
	}
	if _, err := r.Status(ctx, user+1, id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("foreign Status = %v", err)
	}
	m, err := r.Metrics(ctx)
	if err != nil || m["sent"] != 1 || m["pending"] != 3 {
		t.Fatalf("Metrics = %v, %v", m, err)
	}

	s := NewService(r)
	full, err := s.Status(ctx, user, id)
	if err != nil || full["success_count"] != 1 || full["pending_count"] != 1 || full["push_error"] != false {
		t.Fatalf("service Status = %v, %v", full, err)
	}
}

func TestServiceSendTx(t *testing.T) {
	db := testutil.OpenTestDB(t)
	s := NewService(NewRepository(db))
	ctx := context.Background()
	user := seedUser(t, db, "a@x.io")

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := s.SendTx(ctx, tx, user, PushNotificationPayload{Title: "", Body: "B"}); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("invalid = %v", err)
	}
	n, err := s.SendTx(ctx, tx, user, PushNotificationPayload{Title: "T", Body: "B"})
	if err != nil || n.ID == 0 {
		t.Fatalf("SendTx = %+v, %v", n, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if c := testutil.Count(t, db, `SELECT count(*) FROM notifications`); c != 1 {
		t.Fatalf("count = %d", c)
	}
}

func TestRepositoryClosedDB(t *testing.T) {
	r := NewRepository(closedDB(t))
	ctx := context.Background()
	checks := map[string]error{
		"Register": r.Register(ctx, 1, "t", "android"),
		"Remove":   r.Remove(ctx, 1, "t"),
		"Create":   r.Create(ctx, &Notification{UserID: 1}),
		"Read":     r.Read(ctx, 1, 1),
	}
	_, checks["Tokens"] = r.Tokens(ctx, 1)
	_, checks["List"] = r.List(ctx, 1, 0)
	_, checks["Enqueue"] = r.Enqueue(ctx, 1, PushNotificationPayload{Title: "T", Body: "B"})
	_, checks["Status"] = r.Status(ctx, 1, 1)
	_, checks["Metrics"] = r.Metrics(ctx)
	for name, err := range checks {
		if err == nil {
			t.Errorf("%s: expected error on closed db", name)
		}
	}
}

func TestServiceValidationExtra(t *testing.T) {
	s := NewService(&mockRepo{})
	ctx := context.Background()
	bad := []PushNotificationPayload{
		{Title: "T", Body: "B", Token: "x"},
		{Title: "T", Body: "B", Tokens: []string{"x"}},
		{Title: "T", Body: "B", Data: map[string]string{"message_type": "x"}},
		{Title: "T", Body: string(make([]byte, 3500))},
	}
	for i, p := range bad {
		if _, err := s.Send(ctx, 1, p); !errors.Is(err, ErrInvalidPayload) {
			t.Errorf("case %d: err = %v", i, err)
		}
	}
	if _, err := s.Send(ctx, 1, PushNotificationPayload{Title: "T", Body: "B", IdempotencyKey: string(make([]byte, 128))}); err != nil {
		t.Errorf("128-byte key must be accepted: %v", err)
	}
	repoErr := errors.New("boom")
	if _, err := NewService(&mockRepo{statusErr: repoErr}).Status(ctx, 1, 1); !errors.Is(err, repoErr) {
		t.Errorf("Status err = %v", err)
	}
	st, _ := NewService(&mockRepo{statuses: []DeliveryStatus{{Status: "cancelled"}, {Status: "retry"}}}).Status(ctx, 1, 1)
	if st["failure_count"] != 1 || st["pending_count"] != 1 {
		t.Errorf("Status = %v", st)
	}
}

func TestFormatEventKey(t *testing.T) {
	if got := formatEventKey(3, "temperature", 99); got != "sensor:3:temperature:99" {
		t.Fatal(got)
	}
}

func TestNewMessagingWithoutCredentials(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(t.TempDir(), "missing.json"))
	if c, err := NewMessaging(context.Background(), "demo"); err == nil || c != nil {
		t.Fatalf("NewMessaging = %v, %v; want credentials error", c, err)
	}
}
