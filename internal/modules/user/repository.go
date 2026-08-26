package user

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
)

type UserRepository interface {
	Detail(ctx context.Context, req UserDetailRequest) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context, req ListUserRequest) ([]User, error)
	Update(ctx context.Context, req UpdateUserRequest) error
	Delete(ctx context.Context, req DeleteUserRequest) error
}

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Detail(ctx context.Context, req UserDetailRequest) (*User, error) {
	query := `
		SELECT id, name, email, COALESCE(msisdn, ''), password, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u User
	err := r.db.QueryRowContext(ctx, query, req.ID).Scan(
		&u.ID, &u.Name, &u.Email, &u.Msisdn, &u.Password, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, name, email, COALESCE(msisdn, ''), password, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&u.ID, &u.Name, &u.Email, &u.Msisdn, &u.Password, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) List(ctx context.Context, req ListUserRequest) ([]User, error) {
	query := `
		SELECT id, name, email, COALESCE(msisdn, ''), password, created_at, updated_at
		FROM users
		ORDER BY id DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User = make([]User, 0)
	for rows.Next() {
		var u User
		err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Msisdn, &u.Password, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userRepository) Update(ctx context.Context, req UpdateUserRequest) error {
	query := `
		UPDATE users
		SET name = COALESCE(NULLIF($1, ''), name), 
		    email = COALESCE(NULLIF($2, ''), email), 
		    msisdn = COALESCE(NULLIF($3, ''), msisdn),
		    updated_at = $4
		WHERE id = $5
	`
	updatedAt := time.Now()
	result, err := r.db.ExecContext(ctx, query, req.Name, req.Email, req.Msisdn, updatedAt, req.ID)
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "duplicate key") {
			return ErrEmailAlreadyExists
		}
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *userRepository) Delete(ctx context.Context, req DeleteUserRequest) error {
	query := `
		DELETE FROM users
		WHERE id = $1
	`
	result, err := r.db.ExecContext(ctx, query, req.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}
