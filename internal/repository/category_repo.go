package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifrunruhin12/money-manager/internal/domain"
)

// CategoryRepository defines persistence operations for categories.
type CategoryRepository interface {
	Insert(ctx context.Context, db DBTX, c domain.Category) error
	Update(ctx context.Context, c domain.Category) error
	Delete(ctx context.Context, id, userID string) error
	ListByUser(ctx context.Context, userID string) ([]domain.Category, error)
	IsReferencedByTransactions(ctx context.Context, id, userID string) (bool, error)
	IsReferencedByBigBuys(ctx context.Context, id, userID string) (bool, error)
}

type categoryRepository struct {
	db *pgxpool.Pool
}

// NewCategoryRepository creates a new CategoryRepository backed by the given pool.
func NewCategoryRepository(db *pgxpool.Pool) CategoryRepository {
	return &categoryRepository{db: db}
}

// Insert persists a new category using the provided DBTX (pool or tx).
func (r *categoryRepository) Insert(ctx context.Context, db DBTX, c domain.Category) error {
	_, err := db.Exec(ctx,
		`INSERT INTO categories (id, user_id, name, created_at) VALUES ($1, $2, $3, $4)`,
		c.ID, c.UserID, c.Name, c.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgErrCodeUniqueViolation {
			return domain.ErrConflict
		}
		return err
	}
	return nil
}

func (r *categoryRepository) Update(ctx context.Context, c domain.Category) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE categories SET name = $1 WHERE id = $2 AND user_id = $3`,
		c.Name, c.ID, c.UserID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgErrCodeUniqueViolation {
			return domain.ErrConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *categoryRepository) Delete(ctx context.Context, id, userID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM categories WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *categoryRepository) ListByUser(ctx context.Context, userID string) ([]domain.Category, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, name, created_at FROM categories WHERE user_id = $1 ORDER BY name ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.CreatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (r *categoryRepository) IsReferencedByTransactions(ctx context.Context, id, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM transactions 
			WHERE category_id = $1 
			  AND user_id = $2 
			  AND deleted_at IS NULL
		)`,
		id, userID,
	).Scan(&exists)
	return exists, err
}

func (r *categoryRepository) IsReferencedByBigBuys(ctx context.Context, id, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM big_buys 
			WHERE category_id = $1 
			  AND user_id = $2 
			  AND deleted_at IS NULL
		)`,
		id, userID,
	).Scan(&exists)
	return exists, err
}
