package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifrunruhin12/money-manager/internal/domain"
	"github.com/ifrunruhin12/money-manager/internal/repository"
)

// TransactionService defines business logic for transaction operations.
//
// Architectural evolution note:
// As the system grows, this service will likely split into specialized methods with
// distinct invariants and validation rules:
//   - CreateManualTransaction: user-entered transactions
//   - CreateRuleGeneratedTransaction: auto-generated from rules (with idempotency)
//   - OverrideTransaction: replace a rule-generated transaction with actual value
//   - SkipTransaction: exclude from calculations
//   - RestoreTransaction: re-include in calculations
//   - DeleteTransaction: soft delete
//
// Each operation has different validation rules, side effects, and concurrency requirements.
// For MVP, we start with a unified Create method that handles the common case (manual transactions).
type TransactionService interface {
	Create(ctx context.Context, tx domain.Transaction) (*domain.Transaction, error)
	ListByDateRange(ctx context.Context, userID string, from, to time.Time, limit int, cursorDate time.Time, cursorID string) ([]domain.Transaction, *Cursor, error)
}

// Cursor represents pagination state for transaction listing.
type Cursor struct {
	Date time.Time `json:"date"`
	ID   string    `json:"id"`
}

type transactionService struct {
	db              *pgxpool.Pool
	transactionRepo repository.TransactionRepository
	accountRepo     repository.AccountRepository
	enableEventLog  bool
	logger          *slog.Logger
}

// NewTransactionService creates a new TransactionService with the given dependencies.
func NewTransactionService(
	db *pgxpool.Pool,
	transactionRepo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
	enableEventLog bool,
	logger *slog.Logger,
) TransactionService {
	return &transactionService{
		db:              db,
		transactionRepo: transactionRepo,
		accountRepo:     accountRepo,
		enableEventLog:  enableEventLog,
		logger:          logger,
	}
}

// Create validates and persists a new transaction, atomically updating the account balance.
// Validation rules:
//   - amount must be non-zero
//   - category_id must be non-empty
//   - date must be non-zero
//
// ALL operations (insert transaction, update balance, append event) execute inside a single
// database transaction. If ANY step fails, the entire operation is rolled back — no partial state.
// This is critical for financial correctness: we never want a transaction to exist without its
// corresponding balance update, or vice versa.
func (s *transactionService) Create(ctx context.Context, tx domain.Transaction) (*domain.Transaction, error) {
	// Validation
	if tx.Amount == 0 {
		return nil, fmt.Errorf("%w: amount must be non-zero", domain.ErrValidation)
	}
	if tx.CategoryID == "" {
		return nil, fmt.Errorf("%w: category_id is required", domain.ErrValidation)
	}
	if tx.Date.IsZero() {
		return nil, fmt.Errorf("%w: date is required", domain.ErrValidation)
	}

	// Set defaults
	now := time.Now().UTC()
	if tx.ID == "" {
		tx.ID = uuid.New().String()
	}
	if tx.Type == "" {
		tx.Type = domain.TransactionTypeManual
	}
	tx.CreatedAt = now
	tx.UpdatedAt = now

	// Begin database transaction — ALL OR NOTHING
	dbTx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer dbTx.Rollback(ctx) // Safe to call even after commit; no-op if already committed

	// Insert transaction
	rowsAffected, err := s.transactionRepo.Insert(ctx, dbTx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to insert transaction: %w", err)
	}

	// If rowsAffected == 0, the transaction already existed (idempotent skip via ON CONFLICT DO NOTHING).
	// This is expected for rule-generated transactions with duplicate (source_id, source_type, generation_date).
	// For manual transactions, this should not happen unless there's a programming error.
	// We treat this as success and return the transaction as-is WITHOUT updating balance
	// (balance was already updated when the original transaction was created).
	if rowsAffected == 0 {
		// Rollback the no-op transaction
		dbTx.Rollback(ctx)
		
		// NOTE: This fetch may fail if the conflict was NOT on tx.ID but on the unique constraint
		// (source_id, source_type, generation_date). For manual transactions, source_id is typically NULL,
		// so this should succeed. For rule-generated transactions, the caller should handle idempotency
		// at a higher level (e.g., check ExistsForSourceAndDate before calling Create).
		// Future improvement: add GetBySourceAndGenerationDate to repository for proper idempotent recovery.
		existing, err := s.transactionRepo.GetByID(ctx, tx.ID)
		if err != nil {
			// If fetch fails, it means the conflict was on a different row (different ID).
			// This is a sign of improper idempotency handling by the caller.
			// Return a descriptive error instead of silently failing.
			return nil, fmt.Errorf("transaction already exists (conflict on unique constraint) but failed to fetch by ID: %w", err)
		}
		return existing, nil
	}

	// Atomically update account balance
	// This uses: UPDATE accounts SET current_balance = current_balance + $1 WHERE user_id = $2
	// Single atomic SQL statement — no race conditions.
	// CRITICAL: We pass dbTx here so the balance update happens in the SAME transaction
	// as the transaction insert. This ensures atomicity: both succeed or both fail together.
	if err := s.accountRepo.AdjustBalance(ctx, dbTx, tx.UserID, tx.Amount); err != nil {
		// Balance update failed — rollback will undo the transaction insert.
		// No partial state persists. This is the correct behavior.
		return nil, fmt.Errorf("failed to update account balance: %w", err)
	}

	// Append event log entry if enabled
	// Event log failure is non-fatal for MVP, but we log it for debugging.
	// In production, event log failures would be monitored and alerted.
	if s.enableEventLog {
		if err := s.appendEvent(ctx, dbTx, tx.UserID, "CREATE_TX", tx.ID, tx); err != nil {
			// Log the error but don't fail the transaction
			s.logger.Warn("failed to append event log",
				slog.String("user_id", tx.UserID),
				slog.String("transaction_id", tx.ID),
				slog.String("error", err.Error()),
			)
			// Continue — event log is best-effort for MVP
		}
	}

	// Commit the transaction — all operations succeed together
	if err := dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &tx, nil
}

// ListByDateRange retrieves transactions for a user within [from, to], sorted by date DESC, id DESC.
// Supports cursor-based pagination: pass non-zero cursorDate and non-empty cursorID to page after a previous result.
// Returns (transactions, nextCursor, error). If there are no more results, nextCursor is nil.
func (s *transactionService) ListByDateRange(
	ctx context.Context,
	userID string,
	from, to time.Time,
	limit int,
	cursorDate time.Time,
	cursorID string,
) ([]domain.Transaction, *Cursor, error) {
	if limit <= 0 {
		limit = 50
	}

	// Fetch limit+1 to detect if there are more results
	txs, err := s.transactionRepo.ListByDateRange(ctx, userID, from, to, limit+1, cursorDate, cursorID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	var nextCursor *Cursor
	if len(txs) > limit {
		// More results exist; construct cursor from the last returned item
		lastTx := txs[limit-1]
		nextCursor = &Cursor{
			Date: lastTx.Date,
			ID:   lastTx.ID,
		}
		txs = txs[:limit]
	}

	return txs, nextCursor, nil
}

// appendEvent writes an event log entry to the events table within the given transaction.
// This ensures event log entries are part of the same atomic transaction as the operation they log.
// Failures are logged but do not block the transaction for MVP.
//
// TODO: Extract to EventRepository for proper separation of concerns.
// Services should orchestrate business rules; repositories should own persistence.
// Future: eventRepo.Append(ctx, db, event) instead of raw SQL here.
func (s *transactionService) appendEvent(ctx context.Context, db repository.DBTX, userID, eventType, entityID string, payload interface{}) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	eventID := uuid.New().String()
	_, err = db.Exec(ctx,
		`INSERT INTO events (id, user_id, type, entity_id, payload, created_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())`,
		eventID, userID, eventType, entityID, payloadJSON,
	)
	return err
}
