package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifrunruhin12/money-manager/internal/domain"
)

type BigBuyRepository interface {
	Insert(ctx context.Context, db DBTX, b domain.BigBuy) error
	Update(ctx context.Context, db DBTX, b domain.BigBuy) error
	Delete(ctx context.Context, db DBTX, id string) error
	GetByID(ctx context.Context, db DBTX, id string, userID string) (*domain.BigBuy, error)
	GetByIDForUpdate(ctx context.Context, db DBTX, id string, userID string) (*domain.BigBuy, error)
	ListByMonth(ctx context.Context, userID string, year int, month int) ([]domain.BigBuy, error)
	SumByDateRange(ctx context.Context, userID string, from, to time.Time) (int, error)
}

type bigBuyRepository struct {
	db *pgxpool.Pool
}

func NewBigBuyRepository(db *pgxpool.Pool) BigBuyRepository {
	return &bigBuyRepository{db: db}
}

const bigBuySelectCols = `id, user_id, title, amount, category_id, note, date, deleted_at, created_at`

func scanBigBuyRow(rows pgx.Rows) (domain.BigBuy, error) {
	var b domain.BigBuy
	err := rows.Scan(
		&b.ID, &b.UserID, &b.Title, &b.Amount, &b.CategoryID,
		&b.Note, &b.Date, &b.DeletedAt, &b.CreatedAt,
	)
	return b, err
}

func (r *bigBuyRepository) Insert(ctx context.Context, db DBTX, b domain.BigBuy) error {
	_, err := db.Exec(ctx,
		`INSERT INTO big_buys (id, user_id, title, amount, category_id, note, date, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		b.ID, b.UserID, b.Title, b.Amount, b.CategoryID, b.Note, b.Date, b.CreatedAt,
	)
	return err
}

func (r *bigBuyRepository) Update(ctx context.Context, db DBTX, b domain.BigBuy) error {
	tag, err := db.Exec(ctx,
		`UPDATE big_buys
		 SET title = $1, amount = $2, category_id = $3, note = $4, date = $5
		 WHERE id = $6 AND user_id = $7 AND deleted_at IS NULL`,
		b.Title, b.Amount, b.CategoryID, b.Note, b.Date,
		b.ID, b.UserID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *bigBuyRepository) Delete(ctx context.Context, db DBTX, id string) error {
	tag, err := db.Exec(ctx,
		`UPDATE big_buys
		 SET deleted_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *bigBuyRepository) GetByID(ctx context.Context, db DBTX, id string, userID string) (*domain.BigBuy, error) {
	row := db.QueryRow(ctx,
		`SELECT `+bigBuySelectCols+`
		 FROM big_buys
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		id, userID,
	)

	var b domain.BigBuy
	err := row.Scan(
		&b.ID, &b.UserID, &b.Title, &b.Amount, &b.CategoryID,
		&b.Note, &b.Date, &b.DeletedAt, &b.CreatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

func (r *bigBuyRepository) GetByIDForUpdate(ctx context.Context, db DBTX, id string, userID string) (*domain.BigBuy, error) {
	row := db.QueryRow(ctx,
		`SELECT `+bigBuySelectCols+`
		 FROM big_buys
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		 FOR UPDATE`,
		id, userID,
	)

	var b domain.BigBuy
	err := row.Scan(
		&b.ID, &b.UserID, &b.Title, &b.Amount, &b.CategoryID,
		&b.Note, &b.Date, &b.DeletedAt, &b.CreatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

func (r *bigBuyRepository) ListByMonth(ctx context.Context, userID string, year int, month int) ([]domain.BigBuy, error) {
	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

	rows, err := r.db.Query(ctx,
		`SELECT `+bigBuySelectCols+`
		 FROM big_buys
		 WHERE user_id = $1
		   AND date BETWEEN $2 AND $3
		   AND deleted_at IS NULL
		 ORDER BY date ASC`,
		userID, monthStart, monthEnd,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buys []domain.BigBuy
	for rows.Next() {
		b, err := scanBigBuyRow(rows)
		if err != nil {
			return nil, err
		}
		buys = append(buys, b)
	}
	return buys, rows.Err()
}

func (r *bigBuyRepository) SumByDateRange(ctx context.Context, userID string, from, to time.Time) (int, error) {
	var sum int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0)
		 FROM big_buys
		 WHERE user_id = $1
		   AND date BETWEEN $2 AND $3
		   AND deleted_at IS NULL`,
		userID, from, to,
	).Scan(&sum)
	return sum, err
}
