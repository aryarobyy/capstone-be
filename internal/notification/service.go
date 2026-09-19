package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"firebase.google.com/go/v4/messaging"
)

var (
	ErrInvalidPayload = errors.New("title/body required, payload must fit 3500 bytes, and routing fields must be empty")
	ErrUnavailable    = errors.New("FCM is not configured")
)

type MessagingClient interface {
	SendEachForMulticast(context.Context, *messaging.MulticastMessage) (*messaging.BatchResponse, error)
}

type SendResult struct {
	Status       string        `json:"status"`
	Notification *Notification `json:"notification"`
	SuccessCount int           `json:"success_count"`
	FailureCount int           `json:"failure_count"`
	PushError    bool          `json:"push_error"`
}

// Service defines the notification domain operations for use by other backend components.
type Service interface {
	Send(ctx context.Context, user int64, p PushNotificationPayload) (*SendResult, error)
	SendToUser(ctx context.Context, user int64, p PushNotificationPayload) (*SendResult, error)
	SendNotification(ctx context.Context, user int64, title, body string, data map[string]string) (*SendResult, error)
	SendTx(ctx context.Context, tx *sql.Tx, user int64, p PushNotificationPayload) (*Notification, error)
	RegisterDevice(ctx context.Context, userID int64, token, deviceType string) error
	RemoveDevice(ctx context.Context, userID int64, token string) error
	List(ctx context.Context, userID, beforeID int64) ([]Notification, error)
	MarkRead(ctx context.Context, userID, notificationID int64) error
	Status(ctx context.Context, userID, id int64) (map[string]any, error)
	Metrics(ctx context.Context) (map[string]int64, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func validatePayload(user int64, p PushNotificationPayload) error {
	encoded, _ := json.Marshal(p)
	if user <= 0 || strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Body) == "" || len(encoded) > 3500 || p.Token != "" || len(p.Tokens) > 0 || p.Topic != "" {
		return ErrInvalidPayload
	}
	for key := range p.Data {
		if key == "from" || key == "message_type" || strings.HasPrefix(key, "google.") || strings.HasPrefix(key, "gcm.") {
			return ErrInvalidPayload
		}
	}
	if len(p.IdempotencyKey) > 128 {
		return ErrInvalidPayload
	}
	return nil
}

// Send enqueues a push notification for asynchronous delivery to the user's active devices.
func (s *service) Send(ctx context.Context, user int64, p PushNotificationPayload) (*SendResult, error) {
	if err := validatePayload(user, p); err != nil {
		return nil, err
	}
	return s.repo.Enqueue(ctx, user, p)
}

// SendToUser is an alias for Send for backward compatibility.
func (s *service) SendToUser(ctx context.Context, user int64, p PushNotificationPayload) (*SendResult, error) {
	return s.Send(ctx, user, p)
}

// SendNotification is a helper to easily send a notification with a title, body, and custom data.
func (s *service) SendNotification(ctx context.Context, user int64, title, body string, data map[string]string) (*SendResult, error) {
	return s.Send(ctx, user, PushNotificationPayload{
		Title: title,
		Body:  body,
		Data:  data,
	})
}

// SendTx enqueues a push notification within an active database transaction.
func (s *service) SendTx(ctx context.Context, tx *sql.Tx, user int64, p PushNotificationPayload) (*Notification, error) {
	if err := validatePayload(user, p); err != nil {
		return nil, err
	}
	return EnqueueTx(ctx, tx, user, p)
}

func (s *service) RegisterDevice(ctx context.Context, userID int64, token, deviceType string) error {
	return s.repo.Register(ctx, userID, token, deviceType)
}

func (s *service) RemoveDevice(ctx context.Context, userID int64, token string) error {
	return s.repo.Remove(ctx, userID, token)
}

func (s *service) List(ctx context.Context, userID, beforeID int64) ([]Notification, error) {
	return s.repo.List(ctx, userID, beforeID)
}

func (s *service) MarkRead(ctx context.Context, userID, notificationID int64) error {
	return s.repo.Read(ctx, userID, notificationID)
}
