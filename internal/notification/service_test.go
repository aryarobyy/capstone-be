package notification

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type mockRepo struct {
	enqueueCalls []PushNotificationPayload
	enqueueErr   error
	registered   []string
	removed      []string
	listResult   []Notification
	listErr      error
	readCalls    []int64
	readErr      error
	statuses     []DeliveryStatus
	statusErr    error
	metricsMap   map[string]int64
	metricsErr   error
}

func (m *mockRepo) Enqueue(ctx context.Context, user int64, p PushNotificationPayload) (*SendResult, error) {
	if m.enqueueErr != nil {
		return nil, m.enqueueErr
	}
	m.enqueueCalls = append(m.enqueueCalls, p)
	return &SendResult{
		Status: "queued",
		Notification: &Notification{
			ID:     100,
			UserID: user,
			Title:  p.Title,
			Body:   p.Body,
			Data:   p.Data,
		},
	}, nil
}

func (m *mockRepo) Status(ctx context.Context, user, id int64) ([]DeliveryStatus, error) {
	if m.statusErr != nil {
		return nil, m.statusErr
	}
	return m.statuses, nil
}

func (m *mockRepo) Metrics(ctx context.Context) (map[string]int64, error) {
	if m.metricsErr != nil {
		return nil, m.metricsErr
	}
	return m.metricsMap, nil
}

func (m *mockRepo) Register(ctx context.Context, user int64, token, device string) error {
	m.registered = append(m.registered, token)
	return nil
}

func (m *mockRepo) Remove(ctx context.Context, user int64, token string) error {
	m.removed = append(m.removed, token)
	return nil
}

func (m *mockRepo) Tokens(ctx context.Context, user int64) ([]string, error) {
	return nil, nil
}

func (m *mockRepo) Create(ctx context.Context, n *Notification) error {
	return nil
}

func (m *mockRepo) List(ctx context.Context, user, before int64) ([]Notification, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.listResult, nil
}

func (m *mockRepo) Read(ctx context.Context, user, id int64) error {
	if m.readErr != nil {
		return m.readErr
	}
	m.readCalls = append(m.readCalls, id)
	return nil
}

func TestServiceSendValidation(t *testing.T) {
	repo := &mockRepo{}
	s := NewService(repo)
	ctx := context.Background()

	testCases := []struct {
		name    string
		user    int64
		payload PushNotificationPayload
		wantErr bool
	}{
		{
			name:    "invalid user id",
			user:    0,
			payload: PushNotificationPayload{Title: "Title", Body: "Body"},
			wantErr: true,
		},
		{
			name:    "empty title",
			user:    1,
			payload: PushNotificationPayload{Title: "   ", Body: "Body"},
			wantErr: true,
		},
		{
			name:    "empty body",
			user:    1,
			payload: PushNotificationPayload{Title: "Title", Body: ""},
			wantErr: true,
		},
		{
			name:    "reserved key in data (google.)",
			user:    1,
			payload: PushNotificationPayload{Title: "Title", Body: "Body", Data: map[string]string{"google.reserved": "val"}},
			wantErr: true,
		},
		{
			name:    "reserved key in data (gcm.)",
			user:    1,
			payload: PushNotificationPayload{Title: "Title", Body: "Body", Data: map[string]string{"gcm.notification": "val"}},
			wantErr: true,
		},
		{
			name:    "reserved key from",
			user:    1,
			payload: PushNotificationPayload{Title: "Title", Body: "Body", Data: map[string]string{"from": "sender"}},
			wantErr: true,
		},
		{
			name:    "topic or token specified in payload",
			user:    1,
			payload: PushNotificationPayload{Title: "Title", Body: "Body", Topic: "alerts"},
			wantErr: true,
		},
		{
			name:    "idempotency key too long",
			user:    1,
			payload: PushNotificationPayload{Title: "Title", Body: "Body", IdempotencyKey: strings.Repeat("x", 129)},
			wantErr: true,
		},
		{
			name:    "valid payload",
			user:    1,
			payload: PushNotificationPayload{Title: "Title", Body: "Body", Data: map[string]string{"sensor_id": "5"}},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Send(ctx, tc.user, tc.payload)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Send() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestServiceSendAndSendNotification(t *testing.T) {
	repo := &mockRepo{}
	s := NewService(repo)
	ctx := context.Background()

	res, err := s.SendNotification(ctx, 42, "Sensor Alert", "Temperature high", map[string]string{"temp": "45"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "queued" || res.Notification.ID != 100 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(repo.enqueueCalls) != 1 {
		t.Fatalf("expected 1 call to repo.Enqueue, got %d", len(repo.enqueueCalls))
	}
	call := repo.enqueueCalls[0]
	if call.Title != "Sensor Alert" || call.Body != "Temperature high" || call.Data["temp"] != "45" {
		t.Fatalf("unexpected payload in repo: %+v", call)
	}

	// SendToUser alias test
	res2, err := s.SendToUser(ctx, 42, PushNotificationPayload{Title: "Alias Test", Body: "Body text"})
	if err != nil || res2.Notification.Title != "Alias Test" {
		t.Fatalf("SendToUser failed: %+v, %v", res2, err)
	}
}

func TestServiceRepoError(t *testing.T) {
	repoErr := errors.New("database failure")
	repo := &mockRepo{enqueueErr: repoErr}
	s := NewService(repo)

	_, err := s.Send(context.Background(), 1, PushNotificationPayload{Title: "T", Body: "B"})
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repoErr, got %v", err)
	}
}

func TestServiceListAndMarkRead(t *testing.T) {
	notifications := []Notification{
		{ID: 1, UserID: 5, Title: "N1", Body: "B1"},
		{ID: 2, UserID: 5, Title: "N2", Body: "B2"},
	}
	repo := &mockRepo{listResult: notifications}
	s := NewService(repo)
	ctx := context.Background()

	list, err := s.List(ctx, 5, 0)
	if err != nil || len(list) != 2 {
		t.Fatalf("unexpected List result: %v, %v", list, err)
	}

	if err := s.MarkRead(ctx, 5, 1); err != nil {
		t.Fatalf("unexpected MarkRead error: %v", err)
	}
	if len(repo.readCalls) != 1 || repo.readCalls[0] != 1 {
		t.Fatalf("unexpected readCalls: %v", repo.readCalls)
	}
}

func TestServiceDeviceManagement(t *testing.T) {
	repo := &mockRepo{}
	s := NewService(repo)
	ctx := context.Background()

	if err := s.RegisterDevice(ctx, 1, "token-123", "android"); err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}
	if len(repo.registered) != 1 || repo.registered[0] != "token-123" {
		t.Fatalf("unexpected registered: %v", repo.registered)
	}

	if err := s.RemoveDevice(ctx, 1, "token-123"); err != nil {
		t.Fatalf("RemoveDevice failed: %v", err)
	}
	if len(repo.removed) != 1 || repo.removed[0] != "token-123" {
		t.Fatalf("unexpected removed: %v", repo.removed)
	}
}

func TestServiceStatusAndMetrics(t *testing.T) {
	repo := &mockRepo{
		statuses: []DeliveryStatus{
			{ID: 1, Status: "sent"},
			{ID: 2, Status: "failed"},
			{ID: 3, Status: "pending"},
		},
		metricsMap: map[string]int64{
			"sent":   10,
			"failed": 2,
		},
	}
	s := NewService(repo)
	ctx := context.Background()

	status, err := s.Status(ctx, 1, 100)
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}
	if status["success_count"] != 1 || status["failure_count"] != 1 || status["pending_count"] != 1 || status["push_error"] != true {
		t.Fatalf("unexpected status map: %+v", status)
	}

	metrics, err := s.Metrics(ctx)
	if err != nil {
		t.Fatalf("Metrics error: %v", err)
	}
	if metrics["sent"] != 10 || metrics["failed"] != 2 {
		t.Fatalf("unexpected metrics map: %+v", metrics)
	}
}
