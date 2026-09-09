package area

import (
	"context"
	"database/sql"
	"errors"
)

var (
	ErrAreaNotFound = errors.New("area not found")
)

type AreaRepository interface {
	Exists(ctx context.Context, id int64) (bool, error)
	FindByID(ctx context.Context, id int64) (*Area, error)
}

type areaRepository struct {
	db *sql.DB
}

func NewAreaRepository(db *sql.DB) AreaRepository {
	return &areaRepository{db: db}
}

func (r *areaRepository) Exists(ctx context.Context, id int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM areas WHERE id = $1)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *areaRepository) FindByID(ctx context.Context, id int64) (*Area, error) {
	query := `SELECT id, farm_id, name, COALESCE(icon, ''), created_at, updated_at FROM areas WHERE id = $1`
	var a Area
	err := r.db.QueryRowContext(ctx, query, id).Scan(&a.ID, &a.FarmID, &a.Name, &a.Icon, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAreaNotFound
		}
		return nil, err
	}
	return &a, nil
}
