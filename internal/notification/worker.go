package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"time"

	"capstone-be/internal/session"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
)

// Workers lease rows before network IO. A crashed worker's lease can be reclaimed.
// FCM delivery is at least once: clients deduplicate using notification_id.
type Worker struct {
	db       *sql.DB
	client   MessagingClient
	sessions session.Store
}

func NewWorker(db *sql.DB, client MessagingClient, sessions session.Store) *Worker {
	return &Worker{db, client, sessions}
}
func (w *Worker) Run(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanup.C:
			c, cancel := context.WithTimeout(ctx, 30*time.Second)
			if err := w.sessions.Cleanup(c); err != nil {
				slog.Error("session cleanup failed")
			}
			cancel()
		case <-tick.C:
			if w.client == nil {
				continue
			}
			for i := 0; i < 20 && ctx.Err() == nil; i++ {
				worked, err := w.Once(ctx)
				if err != nil {
					slog.Error("notification worker storage failure")
					break
				}
				if !worked {
					break
				}
			}
		}
	}
}
func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 7 {
		attempt = 7
	}
	base := time.Minute * time.Duration(1<<(attempt-1))
	if base > time.Hour {
		base = time.Hour
	}
	return base + time.Duration(rand.Int64N(int64(base/4)+1))
}
func classify(err error) (code string, retry, remove bool) {
	switch {
	case err == nil:
		return "", false, false
	case messaging.IsRegistrationTokenNotRegistered(err):
		return "unregistered", false, true
	case messaging.IsInvalidArgument(err):
		return "invalid_argument", false, false
	case messaging.IsSenderIDMismatch(err):
		return "sender_id_mismatch", false, false
	case messaging.IsThirdPartyAuthError(err):
		return "third_party_auth", false, false
	case messaging.IsQuotaExceeded(err):
		return "quota_exceeded", true, false
	case messaging.IsUnavailable(err):
		return "unavailable", true, false
	case messaging.IsInternal(err):
		return "internal", true, false
	default:
		return "transport_or_service_error", true, false
	}
}
func (w *Worker) Once(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	lease := uuid.NewString()
	var id, nid int64
	var attempts int
	err := w.db.QueryRowContext(ctx, `UPDATE notification_deliveries SET status='processing',attempts=attempts+1,lease_id=$1,lease_until=NOW()+INTERVAL '2 minutes' WHERE id=(SELECT id FROM notification_deliveries WHERE (status IN ('pending','retry') AND next_attempt_at<=NOW()) OR (status='processing' AND lease_until<NOW()) ORDER BY next_attempt_at,id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,notification_id,attempts`, lease).Scan(&id, &nid, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var token, title, body, imageURL string
	var data []byte
	var device, user int64
	err = w.db.QueryRowContext(ctx, `SELECT t.id,t.token,n.user_id,n.title,n.body,n.data,n.image_url FROM notification_deliveries d JOIN notifications n ON n.id=d.notification_id JOIN device_tokens t ON t.id=d.device_id AND t.user_id=n.user_id WHERE d.id=$1 AND t.updated_at>NOW()-INTERVAL '30 days' AND EXISTS(SELECT 1 FROM sessions s WHERE s.device_id=t.id AND s.user_id=t.user_id AND s.revoked_at IS NULL AND s.expires_at>NOW())`, id).Scan(&device, &token, &user, &title, &body, &data, &imageURL)
	status, code, msgid := "sent", "", ""
	next := time.Now()
	remove := false
	if errors.Is(err, sql.ErrNoRows) {
		status, code = "cancelled", "device_inactive"
	} else if err != nil {
		return true, err
	} else if attempts > 8 {
		status, code = "failed", "attempt_limit"
	} else {
		payload := map[string]string{}
		if err := json.Unmarshal(data, &payload); err != nil {
			status, code = "failed", "invalid_stored_payload"
		} else {
			payload["notification_id"] = strconv.FormatInt(nid, 10)
			response, sendErr := w.client.SendEachForMulticast(ctx, &messaging.MulticastMessage{Tokens: []string{token}, Notification: &messaging.Notification{Title: title, Body: body, ImageURL: imageURL}, Data: payload, Android: &messaging.AndroidConfig{Priority: "high"}})
			if sendErr == nil {
				if response == nil || len(response.Responses) != 1 {
					sendErr = errors.New("invalid batch response")
				} else if !response.Responses[0].Success {
					sendErr = response.Responses[0].Error
					if sendErr == nil {
						sendErr = errors.New("failed response")
					}
				} else {
					msgid = response.Responses[0].MessageID
				}
			}
			var retry bool
			code, retry, remove = classify(sendErr)
			if sendErr != nil {
				status = "failed"
				if retry && attempts < 8 {
					status = "retry"
					next = time.Now().Add(retryDelay(attempts))
				}
			}
		}
	}
	// Persist outcome using a fresh bounded context even if send timed out.
	finish, done := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer done()
	tx, err := w.db.BeginTx(finish, nil)
	if err != nil {
		return true, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(finish, `UPDATE notification_deliveries SET status=$3,last_error=NULLIF($4,''),firebase_message_id=NULLIF($5,''),next_attempt_at=$6,lease_until=NULL,lease_id=NULL,sent_at=CASE WHEN $3='sent' THEN NOW() ELSE NULL END WHERE id=$1 AND lease_id=$2`, id, lease, status, code, msgid, next)
	if err != nil {
		return true, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return true, err
	}
	if remove && changed == 1 {
		if _, err = tx.ExecContext(finish, `DELETE FROM device_tokens WHERE id=$1 AND token=$2 AND user_id=$3`, device, token, user); err != nil {
			return true, err
		}
	}
	if err = tx.Commit(); err != nil {
		return true, err
	}
	slog.Info("notification delivery", "delivery_id", id, "notification_id", nid, "status", status, "attempt", attempts, "error_code", code)
	return true, nil
}
