package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"capstone-be/internal/testutil"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// fakeFCM records multicast messages and returns a scripted result.
type fakeFCM struct {
	calls []*messaging.MulticastMessage
	resp  *messaging.BatchResponse
	err   error
}

func (f *fakeFCM) SendEachForMulticast(_ context.Context, m *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
	f.calls = append(f.calls, m)
	return f.resp, f.err
}

func ok(id string) *messaging.BatchResponse {
	return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true, MessageID: id}}}
}
func failed(err error) *messaging.BatchResponse {
	return &messaging.BatchResponse{FailureCount: 1, Responses: []*messaging.SendResponse{{Error: err}}}
}

// fcmError obtains a genuine *internal.FirebaseError carrying the given FCM
// errorCode by sending through a real messaging client to a local HTTP stub.
func fcmError(t *testing.T, code string) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"status": "INVALID_ARGUMENT", "message": code, "details": []map[string]string{{"@type": "type.googleapis.com/google.firebase.fcm.v1.FcmError", "errorCode": code}}}})
	}))
	defer srv.Close()
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: "demo"}, option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.SendEachForMulticast(ctx, &messaging.MulticastMessage{Tokens: []string{"x"}, Notification: &messaging.Notification{Title: "t"}})
	if err != nil || len(resp.Responses) != 1 || resp.Responses[0].Error == nil {
		t.Fatalf("stub send = %+v, %v", resp, err)
	}
	return resp.Responses[0].Error
}

func TestClassify(t *testing.T) {
	cases := []struct {
		fcm, code     string
		retry, remove bool
	}{
		{"UNREGISTERED", "unregistered", false, true},
		{"INVALID_ARGUMENT", "invalid_argument", false, false},
		{"SENDER_ID_MISMATCH", "sender_id_mismatch", false, false},
		{"THIRD_PARTY_AUTH_ERROR", "third_party_auth", false, false},
		{"QUOTA_EXCEEDED", "quota_exceeded", true, false},
		{"UNAVAILABLE", "unavailable", true, false},
		{"INTERNAL", "internal", true, false},
	}
	for _, c := range cases {
		code, retry, remove := classify(fcmError(t, c.fcm))
		if code != c.code || retry != c.retry || remove != c.remove {
			t.Errorf("%s: got %s %v %v", c.fcm, code, retry, remove)
		}
	}
	if code, retry, remove := classify(nil); code != "" || retry || remove {
		t.Error("nil error must classify as success")
	}
	if code, retry, _ := classify(errors.New("dial tcp")); code != "transport_or_service_error" || !retry {
		t.Error("unknown errors must be retried")
	}
}

func TestRetryDelay(t *testing.T) {
	cases := map[int]time.Duration{0: time.Minute, 1: time.Minute, 2: 2 * time.Minute, 7: 64 * time.Minute, 50: 64 * time.Minute}
	for attempt, base := range cases {
		want := base
		if want > time.Hour {
			want = time.Hour
		}
		for i := 0; i < 20; i++ {
			if d := retryDelay(attempt); d < want || d > want+want/4 {
				t.Fatalf("retryDelay(%d) = %v, want [%v,%v]", attempt, d, want, want+want/4)
			}
		}
	}
}

type delivery struct {
	status, lastError, msgID string
	attempts                 int
	nextInFuture, sent       bool // nextInFuture: rescheduled by at least a retry delay (tolerates host/db clock skew)
}

func readDelivery(t *testing.T, db *sql.DB, id int64) delivery {
	t.Helper()
	var d delivery
	var lastErr, msg sql.NullString
	var lease sql.NullString
	err := db.QueryRow(`SELECT status,attempts,last_error,firebase_message_id,next_attempt_at>NOW()+INTERVAL '30 seconds',sent_at IS NOT NULL,lease_id::text FROM notification_deliveries WHERE id=$1`, id).Scan(&d.status, &d.attempts, &lastErr, &msg, &d.nextInFuture, &d.sent, &lease)
	if err != nil {
		t.Fatal(err)
	}
	if lease.Valid {
		t.Fatalf("lease not released: %s", lease.String)
	}
	d.lastError, d.msgID = lastErr.String, msg.String
	return d
}

// seedDelivery creates user, active device, notification and one pending delivery.
func seedDelivery(t *testing.T, db *sql.DB, token string) (delivery, device int64) {
	t.Helper()
	user := seedUser(t, db, token+"@x.io")
	device = seedDevice(t, db, user, token)
	if _, err := NewRepository(db).Enqueue(context.Background(), user, PushNotificationPayload{Title: "T", Body: "B", ImageURL: "http://img", Data: map[string]string{"k": "v"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT max(id) FROM notification_deliveries`).Scan(&delivery); err != nil {
		t.Fatal(err)
	}
	return delivery, device
}

func runOnce(t *testing.T, w *Worker, wantWorked bool) {
	t.Helper()
	worked, err := w.Once(context.Background())
	if err != nil || worked != wantWorked {
		t.Fatalf("Once = %v, %v; want %v", worked, err, wantWorked)
	}
}

func TestWorkerSends(t *testing.T) {
	db := testutil.OpenTestDB(t)
	fake := &fakeFCM{resp: ok("msg-1")}
	w := NewWorker(db, fake, nil)

	runOnce(t, w, false) // empty queue
	id, _ := seedDelivery(t, db, "tok")
	runOnce(t, w, true)
	runOnce(t, w, false) // nothing left

	d := readDelivery(t, db, id)
	if d.status != "sent" || d.attempts != 1 || d.msgID != "msg-1" || !d.sent || d.lastError != "" {
		t.Fatalf("delivery = %+v", d)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("calls = %d", len(fake.calls))
	}
	m := fake.calls[0]
	if m.Tokens[0] != "tok" || m.Notification.Title != "T" || m.Notification.ImageURL != "http://img" || m.Data["k"] != "v" || m.Data["notification_id"] == "" || m.Android.Priority != "high" {
		t.Fatalf("message = %+v", m)
	}
}

func TestWorkerOutcomes(t *testing.T) {
	unregistered := fcmError(t, "UNREGISTERED")
	invalid := fcmError(t, "INVALID_ARGUMENT")
	cases := []struct {
		name          string
		resp          *messaging.BatchResponse
		err           error
		priorAttempts int
		want          delivery
		tokenRemoved  bool
	}{
		{name: "transport error retries", err: errors.New("dial"), want: delivery{status: "retry", lastError: "transport_or_service_error", attempts: 1, nextInFuture: true}},
		{name: "nil response retries", want: delivery{status: "retry", lastError: "transport_or_service_error", attempts: 1, nextInFuture: true}},
		{name: "wrong response count retries", resp: &messaging.BatchResponse{}, want: delivery{status: "retry", lastError: "transport_or_service_error", attempts: 1, nextInFuture: true}},
		{name: "unsuccessful without error retries", resp: failed(nil), want: delivery{status: "retry", lastError: "transport_or_service_error", attempts: 1, nextInFuture: true}},
		{name: "invalid argument fails", resp: failed(invalid), want: delivery{status: "failed", lastError: "invalid_argument", attempts: 1}},
		{name: "unregistered fails and removes token", resp: failed(unregistered), want: delivery{status: "failed", lastError: "unregistered", attempts: 1}, tokenRemoved: true},
		{name: "retryable at eighth attempt fails", err: errors.New("dial"), priorAttempts: 7, want: delivery{status: "failed", lastError: "transport_or_service_error", attempts: 8}},
		{name: "attempt limit", resp: ok("never"), priorAttempts: 8, want: delivery{status: "failed", lastError: "attempt_limit", attempts: 9}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := testutil.OpenTestDB(t)
			fake := &fakeFCM{resp: c.resp, err: c.err}
			id, device := seedDelivery(t, db, "tok")
			testutil.Exec(t, db, `UPDATE notification_deliveries SET attempts=$2 WHERE id=$1`, id, c.priorAttempts)
			runOnce(t, NewWorker(db, fake, nil), true)
			if d := readDelivery(t, db, id); d != c.want {
				t.Fatalf("delivery = %+v, want %+v", d, c.want)
			}
			if n := testutil.Count(t, db, `SELECT count(*) FROM device_tokens WHERE id=$1`, device); (n == 0) != c.tokenRemoved {
				t.Fatalf("device rows = %d, removed want %v", n, c.tokenRemoved)
			}
		})
	}
}

func TestWorkerSkipsAndCancels(t *testing.T) {
	db := testutil.OpenTestDB(t)
	fake := &fakeFCM{resp: ok("m")}
	w := NewWorker(db, fake, nil)
	id, device := seedDelivery(t, db, "tok")

	// Not yet due.
	testutil.Exec(t, db, `UPDATE notification_deliveries SET status='retry',next_attempt_at=NOW()+INTERVAL '1 hour' WHERE id=$1`, id)
	runOnce(t, w, false)
	// Leased by another live worker.
	testutil.Exec(t, db, `UPDATE notification_deliveries SET status='processing',next_attempt_at=NOW(),lease_id=gen_random_uuid(),lease_until=NOW()+INTERVAL '1 minute' WHERE id=$1`, id)
	runOnce(t, w, false)
	// Crashed worker's lease expired: reclaimed and sent.
	testutil.Exec(t, db, `UPDATE notification_deliveries SET lease_until=NOW()-INTERVAL '1 second' WHERE id=$1`, id)
	runOnce(t, w, true)
	if d := readDelivery(t, db, id); d.status != "sent" || d.attempts != 1 {
		t.Fatalf("reclaimed = %+v", d)
	}

	// Device whose session was revoked is cancelled without a send.
	testutil.Exec(t, db, `UPDATE notification_deliveries SET status='pending',sent_at=NULL,next_attempt_at=NOW() WHERE id=$1`, id)
	testutil.Exec(t, db, `UPDATE sessions SET revoked_at=NOW() WHERE device_id=$1`, device)
	runOnce(t, w, true)
	if d := readDelivery(t, db, id); d.status != "cancelled" || d.lastError != "device_inactive" || d.sent {
		t.Fatalf("cancelled = %+v", d)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("calls = %d", len(fake.calls))
	}
}

func TestWorkerInvalidStoredPayload(t *testing.T) {
	db := testutil.OpenTestDB(t)
	fake := &fakeFCM{resp: ok("m")}
	id, _ := seedDelivery(t, db, "tok")
	testutil.Exec(t, db, `UPDATE notifications SET data='[1]'::jsonb`)
	runOnce(t, NewWorker(db, fake, nil), true)
	if d := readDelivery(t, db, id); d.status != "failed" || d.lastError != "invalid_stored_payload" {
		t.Fatalf("delivery = %+v", d)
	}
	if len(fake.calls) != 0 {
		t.Fatal("must not send invalid payload")
	}
}

func TestWorkerStorageError(t *testing.T) {
	if worked, err := NewWorker(closedDB(t), &fakeFCM{}, nil).Once(context.Background()); err == nil || worked {
		t.Fatalf("Once = %v, %v", worked, err)
	}
}

func TestWorkerRunStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	NewWorker(nil, nil, nil).Run(ctx) // returns immediately, never touches nil deps
}

func TestWorkerRunDelivers(t *testing.T) {
	db := testutil.OpenTestDB(t)
	id, _ := seedDelivery(t, db, "tok")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { NewWorker(db, &fakeFCM{resp: ok("m")}, nil).Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var status string
		if err := db.QueryRow(`SELECT status FROM notification_deliveries WHERE id=$1`, id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status == "sent" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Run did not deliver, status %q", status)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
