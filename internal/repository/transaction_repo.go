package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifrunruhin12/money-manager/internal/domain"
)

// TransactionRepository defines persistence operations for transactions.
// All mutating methods accept a DBTX so callers can pass either a *pgxpool.Pool
// (standalone) or a pgx.Tx (inside a caller-managed transaction).
// Read-only methods use the pool directly — they never need transactional context.
type TransactionRepository interface {
	// Insert persists a new transaction using ON CONFLICT DO NOTHING on the idempotency constraint.
	// Returns (rowsAffected, error): rowsAffected == 0 means the row already existed (idempotent skip).
	Insert(ctx context.Context, db DBTX, tx domain.Transaction) (int64, error)
	// Update persists changes to an existing transaction.
	Update(ctx context.Context, db DBTX, tx domain.Transaction) error
	// SoftDelete sets deleted_at = NOW() on a transaction.
	SoftDelete(ctx context.Context, db DBTX, id string) error
	// SetOverridden marks a single transaction as overridden.
	SetOverridden(ctx context.Context, db DBTX, id string, overridden bool) error
	// SetOverriddenBySourceID marks all non-deleted transactions with the given source_id as overridden.
	SetOverriddenBySourceID(ctx context.Context, db DBTX, sourceID string) error

	// GetByID retrieves a transaction by ID. Returns domain.ErrNotFound if not found or soft-deleted.
	GetByID(ctx context.Context, id string) (*domain.Transaction, error)
	// GetByIDForUpdate retrieves a transaction with a row lock (SELECT FOR UPDATE), preventing concurrent modifications.
	// Must be called within a transaction. Use this for Override, Skip, and Restore operations to prevent race conditions.
	GetByIDForUpdate(ctx context.Context, db DBTX, id string) (*domain.Transaction, error)
	// ListByDateRange returns transactions for a user within [from, to], sorted date DESC, id DESC.
	// Supports cursor pagination: pass non-zero cursorDate and non-empty cursorID to page after a previous result.
	// limit <= 0 defaults to 50.
	ListByDateRange(ctx context.Context, userID string, from, to time.Time, limit int, cursorDate time.Time, cursorID string) ([]domain.Transaction, error)
	// ExistsForSourceAndDate checks whether a non-deleted transaction already exists for the given source and logical date.
	ExistsForSourceAndDate(ctx context.Context, sourceID string, date time.Time) (bool, error)
	// SumNonSkippedNonOverridden returns the sum of amounts for all active transactions for a user.
	SumNonSkippedNonOverridden(ctx context.Context, userID string) (int, error)
	// GetHistory walks the source_id chain starting from txID and returns all related records
	// in chronological order (original → overrides). Returns domain.ErrNotFound if txID doesn't exist.
	GetHistory(ctx context.Context, txID string) ([]domain.Transaction, error)
}

type transactionRepository struct {
	db *pgxpool.Pool
}

// NewTransactionRepository creates a new TransactionRepository backed by the given pool.
func NewTransactionRepository(db *pgxpool.Pool) TransactionRepository {
	return &transactionRepository{db: db}
}

const txSelectCols = `id, user_id, type, category_id, amount,
	is_skipped, is_overridden, source_id, source_type,
	note, date, generation_date, updated_at, deleted_at, created_at`

// scanTxRow scans a single transaction from a pgx.Rows cursor.
// Centralises field ordering so all query sites stay in sync.
func scanTxRow(rows pgx.Rows) (domain.Transaction, error) {
	var t domain.Transaction
	err := rows.Scan(
		&t.ID, &t.UserID, &t.Type, &t.CategoryID, &t.Amount,
		&t.IsSkipped, &t.IsOverridden, &t.SourceID, &t.SourceType,
		&t.Note, &t.Date, &t.GenerationDate, &t.UpdatedAt, &t.DeletedAt, &t.CreatedAt,
	)
	return t, err
}

func (r *transactionRepository) Insert(ctx context.Context, db DBTX, tx domain.Transaction) (int64, error) {
	tag, err := db.Exec(ctx,
		`INSERT INTO transactions
		 (id, user_id, type, category_id, amount, is_skipped, is_overridden,
		  source_id, source_type, note, date, generation_date, updated_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		 ON CONFLICT (source_id, source_type, generation_date) 
		 WHERE (source_id IS NOT NULL AND deleted_at IS NULL)
		 DO NOTHING`,
		tx.ID, tx.UserID, tx.Type, tx.CategoryID, tx.Amount,
		tx.IsSkipped, tx.IsOverridden,
		tx.SourceID, tx.SourceType,
		tx.Note, tx.Date, tx.GenerationDate, tx.UpdatedAt, tx.CreatedAt,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *transactionRepository) Update(ctx context.Context, db DBTX, tx domain.Transaction) error {
	tag, err := db.Exec(ctx,
		`UPDATE transactions
		 SET type=$1, category_id=$2, amount=$3, is_skipped=$4, is_overridden=$5,
		     source_id=$6, source_type=$7, note=$8, date=$9, generation_date=$10, updated_at=$11
		 WHERE id=$12 AND user_id=$13 AND deleted_at IS NULL`,
		tx.Type, tx.CategoryID, tx.Amount, tx.IsSkipped, tx.IsOverridden,
		tx.SourceID, tx.SourceType, tx.Note, tx.Date, tx.GenerationDate, tx.UpdatedAt,
		tx.ID, tx.UserID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *transactionRepository) SoftDelete(ctx context.Context, db DBTX, id string) error {
	tag, err := db.Exec(ctx,
		`UPDATE transactions
		 SET deleted_at = NOW(), updated_at = NOW()
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

func (r *transactionRepository) SetOverridden(ctx context.Context, db DBTX, id string, overridden bool) error {
	tag, err := db.Exec(ctx,
		`UPDATE transactions
		 SET is_overridden = $1, updated_at = NOW()
		 WHERE id = $2 AND deleted_at IS NULL`,
		overridden, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *transactionRepository) SetOverriddenBySourceID(ctx context.Context, db DBTX, sourceID string) error {
	_, err := db.Exec(ctx,
		`UPDATE transactions
		 SET is_overridden = TRUE, updated_at = NOW()
		 WHERE source_id = $1 AND deleted_at IS NULL`,
		sourceID,
	)
	return err
}

func (r *transactionRepository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+txSelectCols+`
		 FROM transactions
		 WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, domain.ErrNotFound
	}
	t, err := scanTxRow(rows)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *transactionRepository) GetByIDForUpdate(ctx context.Context, db DBTX, id string) (*domain.Transaction, error) {
	rows, err := db.Query(ctx,
		`SELECT `+txSelectCols+`
		 FROM transactions
		 WHERE id = $1 AND deleted_at IS NULL
		 FOR UPDATE`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, domain.ErrNotFound
	}
	t, err := scanTxRow(rows)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *transactionRepository) ListByDateRange(
	ctx context.Context,
	userID string,
	from, to time.Time,
	limit int,
	cursorDate time.Time,
	cursorID string,
) ([]domain.Transaction, error) {
	if limit <= 0 {
		limit = 50
	}

	var (
		rows pgx.Rows
		err  error
	)

	if !cursorDate.IsZero() && cursorID != "" {
		rows, err = r.db.Query(ctx,
			`SELECT `+txSelectCols+`
			 FROM transactions
			 WHERE user_id = $1
			   AND date BETWEEN $2 AND $3
			   AND deleted_at IS NULL
			   AND (date, id) < ($4, $5)
			 ORDER BY date DESC, id DESC
			 LIMIT $6`,
			userID, from, to, cursorDate, cursorID, limit,
		)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT `+txSelectCols+`
			 FROM transactions
			 WHERE user_id = $1
			   AND date BETWEEN $2 AND $3
			   AND deleted_at IS NULL
			 ORDER BY date DESC, id DESC
			 LIMIT $4`,
			userID, from, to, limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []domain.Transaction
	for rows.Next() {
		t, err := scanTxRow(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, t)
	}
	return txs, rows.Err()
}

func (r *transactionRepository) ExistsForSourceAndDate(ctx context.Context, sourceID string, date time.Time) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM transactions
		   WHERE source_id = $1
		     AND generation_date = $2::date
		     AND deleted_at IS NULL
		 )`,
		sourceID, date,
	).Scan(&exists)
	return exists, err
}

func (r *transactionRepository) SumNonSkippedNonOverridden(ctx context.Context, userID string) (int, error) {
	var sum int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0)
		 FROM transactions
		 WHERE user_id = $1
		   AND is_skipped = FALSE
		   AND is_overridden = FALSE
		   AND deleted_at IS NULL`,
		userID,
	).Scan(&sum)
	return sum, err
}

// GetHistory walks the source_id chain starting from txID and returns all related records
// in chronological order (original → overrides).
//
// Algorithm:
// 1. Load the starting transaction by ID
// 2. If it has source_type='transaction', walk backwards to find the root
// 3. Collect the root transaction
// 4. Find all override transactions that point to the root (source_id = root.ID AND source_type = 'transaction')
// 5. Sort by created_at ASC to get chronological order
//
// Returns domain.ErrNotFound if txID doesn't exist or is soft-deleted.
func (r *transactionRepository) GetHistory(ctx context.Context, txID string) ([]domain.Transaction, error) {
	// Load the starting transaction
	startTx, err := r.GetByID(ctx, txID)
	if err != nil {
		return nil, err
	}

	// Resolve the root transaction by walking backwards
	rootID := txID
	if startTx.SourceType != nil && *startTx.SourceType == domain.SourceTypeTransaction && startTx.SourceID != nil {
		rootID = *startTx.SourceID
	}

	// Collect all transactions in the chain:
	// 1. The root transaction
	// 2. All override transactions that point to the root
	//
	// Query: SELECT all transactions WHERE (id = rootID) OR (source_id = rootID AND source_type = 'transaction')
	// Order by created_at ASC to get chronological order (original first, then overrides in order)
	rows, err := r.db.Query(ctx,
		`SELECT `+txSelectCols+`
		 FROM transactions
		 WHERE (id = $1 OR (source_id = $1 AND source_type = 'transaction'))
		   AND deleted_at IS NULL
		 ORDER BY created_at ASC`,
		rootID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []domain.Transaction
	for rows.Next() {
		t, err := scanTxRow(rows)
		if err != nil {
			return nil, err
		}
		history = append(history, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// If no results, the root transaction was soft-deleted or doesn't exist
	if len(history) == 0 {
		return nil, domain.ErrNotFound
	}

	return history, nil
}
