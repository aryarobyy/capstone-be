package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"
)

const Lifetime = 30 * 24 * time.Hour

func NewRefresh() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	return raw, HashRefresh(raw), nil
}
func HashRefresh(raw string) []byte { sum := sha256.Sum256([]byte(raw)); return sum[:] }
func (r *repository) CreateRefresh(ctx context.Context, sid string, user int64, hash []byte) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO sessions(id,user_id,expires_at,refresh_hash) VALUES($1,$2,$3,$4)`, sid, user, time.Now().Add(Lifetime), hash)
	return err
}

// Rotation is compare-and-swap: concurrent requests cannot both consume the same refresh token.
// Session expiry is absolute; refreshing does not extend it indefinitely.
func (r *repository) Rotate(ctx context.Context, oldHash, newHash []byte) (string, int64, error) {
	var sid string
	var user int64
	err := r.db.QueryRowContext(ctx, `UPDATE sessions SET refresh_hash=$2 WHERE refresh_hash=$1 AND revoked_at IS NULL AND expires_at>NOW() RETURNING id,user_id`, oldHash, newHash).Scan(&sid, &user)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, ErrInactive
	}
	return sid, user, err
}
func (r *repository) RevokeRefresh(ctx context.Context, hash []byte) error {
	var sid string
	var user int64
	err := r.db.QueryRowContext(ctx, `SELECT id,user_id FROM sessions WHERE refresh_hash=$1`, hash).Scan(&sid, &user)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return r.Revoke(ctx, sid, user)
}

func (r *repository) BindInstallation(ctx context.Context, sid string, user int64, token, kind, installation string) error {
	return r.transaction(ctx, user, func(tx *sql.Tx) error {
		// Installation lock also serializes account switches on the same installation.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, installation); err != nil {
			return err
		}
		var active bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL AND expires_at>NOW())`, sid, user).Scan(&active); err != nil {
			return err
		}
		if !active {
			return ErrInactive
		}
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM device_tokens WHERE installation_id=$1 FOR UPDATE`, installation).Scan(&id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if errors.Is(err, sql.ErrNoRows) {
			id, err = upsertDevice(ctx, tx, user, token, kind)
			if err != nil {
				return err
			}
		} else {
			// Retire any older registration that already has the new token.
			if _, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=NOW() WHERE device_id IN(SELECT id FROM device_tokens WHERE token=$1 AND id<>$2) AND revoked_at IS NULL`, token, id); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `DELETE FROM device_tokens WHERE token=$1 AND id<>$2`, token, id); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=NOW() WHERE device_id=$1 AND user_id<>$2 AND revoked_at IS NULL`, id, user); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE device_tokens SET user_id=$2,token=$3,device_type=$4,installation_id=$5,updated_at=NOW() WHERE id=$1`, id, user, token, kind, installation); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE sessions SET device_id=$1 WHERE id=$2`, id, sid)
		return err
	})
}

func (r *repository) Cleanup(ctx context.Context) error {
	returnErr := func() error {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err = tx.ExecContext(ctx, `DELETE FROM device_tokens d WHERE updated_at<NOW()-INTERVAL '30 days' OR NOT EXISTS(SELECT 1 FROM sessions s WHERE s.device_id=d.id AND s.revoked_at IS NULL AND s.expires_at>NOW())`); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at<NOW()-INTERVAL '7 days' OR revoked_at<NOW()-INTERVAL '7 days'`); err != nil {
			return err
		}
		return tx.Commit()
	}
	return returnErr()
}
