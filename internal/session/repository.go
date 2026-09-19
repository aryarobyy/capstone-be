package session

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrInactive = errors.New("session is inactive")

type Store interface {
	CreateRefresh(context.Context, string, int64, []byte) error
	Rotate(context.Context, []byte, []byte) (string, int64, error)
	RevokeRefresh(context.Context, []byte) error
	BindInstallation(context.Context, string, int64, string, string, string) error
	Cleanup(context.Context) error
	Create(context.Context, string, int64, string, string, time.Time) error
	Active(context.Context, string, int64) error
	Revoke(context.Context, string, int64) error
	BindDevice(context.Context, string, int64, string, string) error
	RemoveDevice(context.Context, string, int64, string) error
}
type repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Store { return &repository{db} }

// Lock the user before changing devices/sessions to serialize concurrent logins and logouts.
func (r *repository) transaction(ctx context.Context, user int64, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&id); err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *repository) Create(ctx context.Context, sid string, user int64, token, kind string, expiry time.Time) error {
	return r.transaction(ctx, user, func(tx *sql.Tx) error {
		var device any
		if token != "" {
			id, err := upsertDevice(ctx, tx, user, token, kind)
			if err != nil {
				return err
			}
			device = id
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO sessions(id,user_id,device_id,expires_at) VALUES($1,$2,$3,$4)`, sid, user, device, expiry)
		return err
	})
}

// Existing installations may switch accounts. Invalidate their old sessions before reassignment.
func upsertDevice(ctx context.Context, tx *sql.Tx, user int64, token, kind string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `INSERT INTO device_tokens(user_id,token,device_type) VALUES($1,$2,$3) ON CONFLICT(token) DO UPDATE SET user_id=EXCLUDED.user_id,device_type=EXCLUDED.device_type,updated_at=NOW() RETURNING id`, user, token, kind).Scan(&id)
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=NOW() WHERE device_id=$1 AND user_id<>$2 AND revoked_at IS NULL`, id, user)
	return id, err
}
func (r *repository) Active(ctx context.Context, sid string, user int64) error {
	var active bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sessions s WHERE s.id=$1 AND s.user_id=$2 AND s.revoked_at IS NULL AND s.expires_at>NOW() AND (s.device_id IS NULL OR EXISTS(SELECT 1 FROM device_tokens d WHERE d.id=s.device_id AND d.user_id=s.user_id)))`, sid, user).Scan(&active)
	if err != nil {
		return err
	}
	if !active {
		return ErrInactive
	}
	return nil
}
func (r *repository) Revoke(ctx context.Context, sid string, user int64) error {
	return r.transaction(ctx, user, func(tx *sql.Tx) error {
		var device sql.NullInt64
		err := tx.QueryRowContext(ctx, `UPDATE sessions SET revoked_at=COALESCE(revoked_at,NOW()) WHERE id=$1 AND user_id=$2 RETURNING device_id`, sid, user).Scan(&device)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInactive
		}
		if err != nil {
			return err
		}
		if device.Valid {
			_, err = tx.ExecContext(ctx, `DELETE FROM device_tokens d WHERE d.id=$1 AND d.user_id=$2 AND NOT EXISTS(SELECT 1 FROM sessions s WHERE s.device_id=d.id AND s.revoked_at IS NULL AND s.expires_at>NOW())`, device.Int64, user)
		}
		return err
	})
}
func (r *repository) BindDevice(ctx context.Context, sid string, user int64, token, kind string) error {
	return r.transaction(ctx, user, func(tx *sql.Tx) error {
		var old sql.NullInt64
		err := tx.QueryRowContext(ctx, `SELECT device_id FROM sessions WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL AND expires_at>NOW() FOR UPDATE`, sid, user).Scan(&old)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInactive
		}
		if err != nil {
			return err
		}
		id, err := upsertDevice(ctx, tx, user, token, kind)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE sessions SET device_id=$1 WHERE id=$2`, id, sid)
		if err != nil {
			return err
		}
		if old.Valid && old.Int64 != id {
			_, err = tx.ExecContext(ctx, `DELETE FROM device_tokens d WHERE id=$1 AND user_id=$2 AND NOT EXISTS(SELECT 1 FROM sessions s WHERE s.device_id=d.id AND s.revoked_at IS NULL AND s.expires_at>NOW())`, old.Int64, user)
		}
		return err
	})
}
func (r *repository) RemoveDevice(ctx context.Context, sid string, user int64, token string) error {
	return r.transaction(ctx, user, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM device_tokens d USING sessions s WHERE s.id=$1 AND s.user_id=$2 AND s.revoked_at IS NULL AND s.expires_at>NOW() AND d.id=s.device_id AND d.user_id=$2 AND d.token=$3`, sid, user, token)
		return err
	})
}
