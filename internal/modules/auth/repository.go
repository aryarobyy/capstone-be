package auth

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

type AuthRepository interface {
	Create(ctx context.Context, u *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
}

type postgresAuthRepository struct {
	db *sql.DB
}

func NewAuthRepository(db *sql.DB) AuthRepository {
	return &postgresAuthRepository{db: db}
}

func (r *postgresAuthRepository) Create(ctx context.Context, u *User) error {
	query := `
		INSERT INTO users (name, email, password, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()

	err := r.db.QueryRowContext(ctx, query, u.Name, u.Email, u.Password, u.CreatedAt, u.UpdatedAt).Scan(&u.ID)
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "duplicate key") {
			return ErrEmailAlreadyExists
		}
		return err
	}
	return nil
}

func (r *postgresAuthRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, name, email, password, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&u.ID, &u.Name, &u.Email, &u.Password, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}
