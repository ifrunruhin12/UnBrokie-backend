package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifrunruhin12/money-manager/internal/domain"
)

// AccountRepository defines persistence operations for accounts.
type AccountRepository interface {
	Insert(ctx context.Context, db DBTX, account domain.Account) error
	GetByUserID(ctx context.Context, userID string) (*domain.Account, error)
	UpdateStartingBalance(ctx context.Context, userID string, balance int) error
	UpdateTimezone(ctx context.Context, userID string, tz string) error
	AdjustBalance(ctx context.Context, db DBTX, userID string, delta int) error
	SetDirty(ctx context.Context, userID string, dirty bool) error
	SetReconciled(ctx context.Context, userID string) error
	ReconcileBalance(ctx context.Context, userID string, balance int) error
}

type accountRepository struct {
	db *pgxpool.Pool
}

// NewAccountRepository creates a new AccountRepository backed by the given pool.
func NewAccountRepository(db *pgxpool.Pool) AccountRepository {
	return &accountRepository{db: db}
}

// Insert persists a new account using the provided DBTX (pool or tx).
func (r *accountRepository) Insert(ctx context.Context, db DBTX, account domain.Account) error {
	_, err := db.Exec(ctx,
		`INSERT INTO accounts (id, user_id, starting_balance, current_balance, balance_dirty, currency, timezone, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		account.ID, account.UserID, account.StartingBalance, account.CurrentBalance,
		account.BalanceDirty, account.Currency, account.Timezone, account.CreatedAt,
	)
	return err
}

func (r *accountRepository) GetByUserID(ctx context.Context, userID string) (*domain.Account, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, user_id, starting_balance, current_balance, balance_dirty,
		        last_reconciled_at, currency, timezone, created_at
		 FROM accounts
		 WHERE user_id = $1`,
		userID,
	)

	var a domain.Account
	err := row.Scan(
		&a.ID, &a.UserID, &a.StartingBalance, &a.CurrentBalance, &a.BalanceDirty,
		&a.LastReconciledAt, &a.Currency, &a.Timezone, &a.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *accountRepository) UpdateStartingBalance(ctx context.Context, userID string, balance int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts SET starting_balance = $1, balance_dirty = TRUE WHERE user_id = $2`,
		balance, userID,
	)
	return err
}

func (r *accountRepository) UpdateTimezone(ctx context.Context, userID string, tz string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts SET timezone = $1 WHERE user_id = $2`,
		tz, userID,
	)
	return err
}

func (r *accountRepository) AdjustBalance(ctx context.Context, db DBTX, userID string, delta int) error {
	_, err := db.Exec(ctx,
		`UPDATE accounts SET current_balance = current_balance + $1 WHERE user_id = $2`,
		delta, userID,
	)
	return err
}

func (r *accountRepository) SetDirty(ctx context.Context, userID string, dirty bool) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts SET balance_dirty = $1 WHERE user_id = $2`,
		dirty, userID,
	)
	return err
}

func (r *accountRepository) SetReconciled(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts SET balance_dirty = FALSE, last_reconciled_at = NOW() WHERE user_id = $1`,
		userID,
	)
	return err
}

// ReconcileBalance atomically sets the balance to an authoritative value and marks it reconciled.
// This is the correct way to update balance during reconciliation — avoids race conditions
// from concurrent balance mutations between recompute and update.
func (r *accountRepository) ReconcileBalance(ctx context.Context, userID string, balance int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts 
		 SET current_balance = $1, 
		     balance_dirty = FALSE, 
		     last_reconciled_at = NOW() 
		 WHERE user_id = $2`,
		balance, userID,
	)
	return err
}
