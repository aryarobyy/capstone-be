package notification

import (
	"context"
	"database/sql"
	"encoding/json"
)

type Repository interface {
	Enqueue(context.Context, int64, PushNotificationPayload) (*SendResult, error)
	Status(context.Context, int64, int64) ([]DeliveryStatus, error)
	Metrics(context.Context) (map[string]int64, error)
	Register(context.Context, int64, string, string) error
	Remove(context.Context, int64, string) error
	Tokens(context.Context, int64) ([]string, error)
	Create(context.Context, *Notification) error
	List(context.Context, int64, int64) ([]Notification, error)
	Read(context.Context, int64, int64) error
}
type repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Repository { return &repository{db} }
func (r *repository) Register(ctx context.Context, user int64, token, device string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO device_tokens(user_id,token,device_type) VALUES($1,$2,$3) ON CONFLICT(token) DO UPDATE SET user_id=EXCLUDED.user_id, device_type=EXCLUDED.device_type, updated_at=NOW()`, user, token, device)
	return err
}
func (r *repository) Remove(ctx context.Context, user int64, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM device_tokens WHERE user_id=$1 AND token=$2`, user, token)
	return err
}
func (r *repository) Tokens(ctx context.Context, user int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT d.token FROM device_tokens d WHERE d.user_id=$1 AND EXISTS (SELECT 1 FROM sessions s WHERE s.device_id=d.id AND s.user_id=d.user_id AND s.revoked_at IS NULL AND s.expires_at>NOW()) ORDER BY d.id`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens := []string{}
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}
func (r *repository) Create(ctx context.Context, n *Notification) error {
	data, err := json.Marshal(n.Data)
	if err != nil {
		return err
	}
	return r.db.QueryRowContext(ctx, `INSERT INTO notifications(user_id,title,body,data) VALUES($1,$2,$3,$4) RETURNING id,created_at`, n.UserID, n.Title, n.Body, string(data)).Scan(&n.ID, &n.CreatedAt)
}
func (r *repository) List(ctx context.Context, user, before int64) ([]Notification, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,user_id,title,body,data,is_read,created_at FROM notifications WHERE user_id=$1 AND ($2::bigint=0 OR id<$2) ORDER BY id DESC LIMIT 50`, user, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Notification{}
	for rows.Next() {
		var n Notification
		var data []byte
		if err := rows.Scan(&n.ID, &n.UserID, &n.Title, &n.Body, &data, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &n.Data); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
func (r *repository) Read(ctx context.Context, user, id int64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE notifications SET is_read=TRUE WHERE user_id=$1 AND id=$2`, user, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}
