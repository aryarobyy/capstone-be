package notification

import "time"

type DeviceToken struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Token      string    `json:"token"`
	DeviceType string    `json:"device_type"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Notification struct {
	ID            int64             `json:"id"`
	UserID        int64             `json:"user_id"`
	Title         string            `json:"title"`
	Body          string            `json:"body"`
	Data          map[string]string `json:"data,omitempty"`
	FirebaseMsgID *string           `json:"firebase_msg_id,omitempty"`
	IsRead        bool              `json:"is_read"`
	CreatedAt     time.Time         `json:"created_at"`
}

type PushNotificationPayload struct {
	IdempotencyKey string   `json:"-"`
	Token          string   `json:"token,omitempty"`
	Tokens         []string `json:"tokens,omitempty"`
	Topic          string   `json:"topic,omitempty"`

	Title    string `json:"title"`
	Body     string `json:"body"`
	ImageURL string `json:"image_url,omitempty"`

	Data map[string]string `json:"data,omitempty"`
}
