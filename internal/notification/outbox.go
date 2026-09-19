package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrIdempotencyConflict = errors.New("idempotency key already used with a different payload")

// EnqueueTx is shared by HTTP sends and sensor ingestion. Inbox and jobs commit together.
func EnqueueTx(ctx context.Context, tx *sql.Tx, user int64, p PushNotificationPayload) (*Notification, error) {
	if p.Data == nil {
		p.Data = map[string]string{}
	}
	data, err := json.Marshal(p.Data)
	if err != nil {
		return nil, err
	}
	n := &Notification{UserID: user, Title: p.Title, Body: p.Body, Data: p.Data}
	err = tx.QueryRowContext(ctx, `INSERT INTO notifications(user_id,title,body,data,image_url,event_key) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')) ON CONFLICT(user_id,event_key) WHERE event_key IS NOT NULL DO NOTHING RETURNING id,created_at`, user, p.Title, p.Body, string(data), p.ImageURL, p.IdempotencyKey).Scan(&n.ID, &n.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		var matches bool
		err = tx.QueryRowContext(ctx, `SELECT id,created_at,title=$3 AND body=$4 AND data=$5::jsonb AND image_url=$6 FROM notifications WHERE user_id=$1 AND event_key=$2`, user, p.IdempotencyKey, p.Title, p.Body, string(data), p.ImageURL).Scan(&n.ID, &n.CreatedAt, &matches)
		if err != nil {
			return nil, err
		}
		if !matches {
			return nil, ErrIdempotencyConflict
		}
		return n, nil
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_deliveries(notification_id,device_id) SELECT $1,d.id FROM device_tokens d WHERE d.user_id=$2 AND d.updated_at>NOW()-INTERVAL '30 days' AND EXISTS(SELECT 1 FROM sessions s WHERE s.device_id=d.id AND s.user_id=d.user_id AND s.revoked_at IS NULL AND s.expires_at>NOW())`, n.ID, user)
	return n, err
}
func (r *repository) Enqueue(ctx context.Context, user int64, p PushNotificationPayload) (*SendResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	n, err := EnqueueTx(ctx, tx, user, p)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &SendResult{Notification: n, Status: "queued"}, nil
}

type DeliveryStatus struct {
	ID        int64   `json:"id"`
	Status    string  `json:"status"`
	Attempts  int     `json:"attempts"`
	LastError *string `json:"last_error"`
	MessageID *string `json:"firebase_message_id"`
}

func (r *repository) Status(ctx context.Context, user, id int64) ([]DeliveryStatus, error) {
	var exists bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM notifications WHERE id=$1 AND user_id=$2)`, id, user).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, sql.ErrNoRows
	}
	rows, err := r.db.QueryContext(ctx, `SELECT d.id,d.status,d.attempts,d.last_error,d.firebase_message_id FROM notification_deliveries d JOIN notifications n ON n.id=d.notification_id WHERE n.id=$1 AND n.user_id=$2 ORDER BY d.id`, id, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeliveryStatus{}
	for rows.Next() {
		var d DeliveryStatus
		if err := rows.Scan(&d.ID, &d.Status, &d.Attempts, &d.LastError, &d.MessageID); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *service) Status(ctx context.Context, user, id int64) (map[string]any, error) {
	rows, err := s.repo.Status(ctx, user, id)
	if err != nil {
		return nil, err
	}
	success, failed, pending := 0, 0, 0
	for _, d := range rows {
		switch d.Status {
		case "sent":
			success++
		case "failed", "cancelled":
			failed++
		default:
			pending++
		}
	}
	return map[string]any{"deliveries": rows, "success_count": success, "failure_count": failed, "pending_count": pending, "push_error": failed > 0}, nil
}
func (r *repository) Metrics(ctx context.Context) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT status,count(*) FROM notification_deliveries GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		out[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
func (s *service) Metrics(ctx context.Context) (map[string]int64, error) { return s.repo.Metrics(ctx) }
func formatEventKey(sensor int64, parameter string, stamp int64) string {
	return fmt.Sprintf("sensor:%d:%s:%d", sensor, parameter, stamp)
}
