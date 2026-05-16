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
	Skip(ctx context.Context, txID string, userID string) error
	Restore(ctx context.Context, txID string, userID string) error
	Override(ctx context.Context, originalID string, userID string, amount int, note string) (*domain.Transaction, error)
	GetHistory(ctx context.Context, txID string, userID string) ([]domain.Transaction, error)
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

// Skip marks a transaction as skipped and atomically adjusts the account balance.
// Skipped transactions are excluded from all balance, projection, and dashboard calculations.
// Requirements: 5.6, 7.3
func (s *transactionService) Skip(ctx context.Context, txID string, userID string) error {
	// Begin database transaction — ALL OR NOTHING
	dbTx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer dbTx.Rollback(ctx)

	// Load the transaction to verify ownership and get the amount
	// Use FOR UPDATE to prevent concurrent modifications (race condition protection)
	tx, err := s.transactionRepo.GetByIDForUpdate(ctx, dbTx, txID)
	if err != nil {
		return fmt.Errorf("failed to load transaction: %w", err)
	}

	// Verify ownership
	if tx.UserID != userID {
		return fmt.Errorf("%w: transaction does not belong to user", domain.ErrNotFound)
	}

	// If already skipped, this is a no-op (idempotent)
	if tx.IsSkipped {
		return nil
	}

	// Set is_skipped = true and update updated_at
	tx.IsSkipped = true
	tx.UpdatedAt = time.Now().UTC()
	if err := s.transactionRepo.Update(ctx, dbTx, *tx); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Atomically adjust balance: current_balance -= tx.amount
	// (Removing the transaction's contribution from the balance)
	if err := s.accountRepo.AdjustBalance(ctx, dbTx, userID, -tx.Amount); err != nil {
		return fmt.Errorf("failed to adjust balance: %w", err)
	}

	// Append event log entry if enabled
	if s.enableEventLog {
		if err := s.appendEvent(ctx, dbTx, userID, "SKIP_TX", txID, tx); err != nil {
			s.logger.Warn("failed to append event log",
				slog.String("user_id", userID),
				slog.String("transaction_id", txID),
				slog.String("error", err.Error()),
			)
		}
	}

	// Commit the transaction
	if err := dbTx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Restore marks a previously skipped transaction as active and atomically adjusts the account balance.
// Requirements: 7.4
func (s *transactionService) Restore(ctx context.Context, txID string, userID string) error {
	// Begin database transaction — ALL OR NOTHING
	dbTx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer dbTx.Rollback(ctx)

	// Load the transaction to verify ownership and get the amount
	// Use FOR UPDATE to prevent concurrent modifications (race condition protection)
	tx, err := s.transactionRepo.GetByIDForUpdate(ctx, dbTx, txID)
	if err != nil {
		return fmt.Errorf("failed to load transaction: %w", err)
	}

	// Verify ownership
	if tx.UserID != userID {
		return fmt.Errorf("%w: transaction does not belong to user", domain.ErrNotFound)
	}

	// If not skipped, this is a no-op (idempotent)
	if !tx.IsSkipped {
		return nil
	}

	// Set is_skipped = false and update updated_at
	tx.IsSkipped = false
	tx.UpdatedAt = time.Now().UTC()
	if err := s.transactionRepo.Update(ctx, dbTx, *tx); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Atomically adjust balance: current_balance += tx.amount
	// (Re-adding the transaction's contribution to the balance)
	if err := s.accountRepo.AdjustBalance(ctx, dbTx, userID, tx.Amount); err != nil {
		return fmt.Errorf("failed to adjust balance: %w", err)
	}

	// Append event log entry if enabled
	if s.enableEventLog {
		if err := s.appendEvent(ctx, dbTx, userID, "RESTORE_TX", txID, tx); err != nil {
			s.logger.Warn("failed to append event log",
				slog.String("user_id", userID),
				slog.String("transaction_id", txID),
				slog.String("error", err.Error()),
			)
		}
	}

	// Commit the transaction
	if err := dbTx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Override creates a new override transaction that replaces an original transaction.
// The original transaction is marked as overridden and excluded from calculations.
// If the original transaction is itself an override, the new override replaces it
// and points to the root transaction.
// Requirements: 5.5, 7.1, 7.5
func (s *transactionService) Override(ctx context.Context, originalID string, userID string, amount int, note string) (*domain.Transaction, error) {
	// Validation
	if amount == 0 {
		return nil, fmt.Errorf("%w: amount must be non-zero", domain.ErrValidation)
	}

	// Begin database transaction — ALL OR NOTHING
	dbTx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer dbTx.Rollback(ctx)

	// Load the original transaction
	// Use FOR UPDATE to prevent concurrent modifications (race condition protection)
	originalTx, err := s.transactionRepo.GetByIDForUpdate(ctx, dbTx, originalID)
	if err != nil {
		return nil, fmt.Errorf("failed to load original transaction: %w", err)
	}

	// Verify ownership
	if originalTx.UserID != userID {
		return nil, fmt.Errorf("%w: transaction does not belong to user", domain.ErrNotFound)
	}

	// Resolve the root transaction by walking the source_id chain
	// If the original transaction has source_type='transaction', it's an override itself
	// and we need to find the root transaction it points to.
	rootID := originalID
	if originalTx.SourceType != nil && *originalTx.SourceType == domain.SourceTypeTransaction && originalTx.SourceID != nil {
		rootID = *originalTx.SourceID
	}

	// Mark all previous overrides for this root as overridden
	// This handles the case where we're overriding an override
	if err := s.transactionRepo.SetOverriddenBySourceID(ctx, dbTx, rootID); err != nil {
		return nil, fmt.Errorf("failed to mark previous overrides: %w", err)
	}

	// Mark the root transaction as overridden
	if err := s.transactionRepo.SetOverridden(ctx, dbTx, rootID, true); err != nil {
		return nil, fmt.Errorf("failed to mark root transaction as overridden: %w", err)
	}

	// Create the new override transaction
	now := time.Now().UTC()
	overrideTx := domain.Transaction{
		ID:         uuid.New().String(),
		UserID:     userID,
		Type:       domain.TransactionTypeOverride,
		CategoryID: originalTx.CategoryID,
		Amount:     amount,
		IsSkipped:  false,
		IsOverridden: false,
		SourceID:   &rootID,
		SourceType: sourceTypePtr(domain.SourceTypeTransaction),
		Note:       note,
		Date:       originalTx.Date,
		GenerationDate: nil, // Override transactions don't have generation_date
		UpdatedAt:  now,
		CreatedAt:  now,
	}

	// Insert the override transaction
	rowsAffected, err := s.transactionRepo.Insert(ctx, dbTx, overrideTx)
	if err != nil {
		return nil, fmt.Errorf("failed to insert override transaction: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("failed to insert override transaction: conflict")
	}

	// Adjust balance by (-original.amount + newAmount)
	// This removes the original transaction's contribution and adds the override's contribution
	balanceDelta := -originalTx.Amount + amount
	if err := s.accountRepo.AdjustBalance(ctx, dbTx, userID, balanceDelta); err != nil {
		return nil, fmt.Errorf("failed to adjust balance: %w", err)
	}

	// Set balance_dirty = true to trigger recompute on next read
	// This is a safety measure for complex multi-table operations
	if err := s.accountRepo.SetDirty(ctx, dbTx, userID, true); err != nil {
		return nil, fmt.Errorf("failed to set balance dirty: %w", err)
	}

	// Append event log entry if enabled
	if s.enableEventLog {
		eventPayload := map[string]interface{}{
			"override_tx": overrideTx,
			"original_id": originalID,
			"root_id":     rootID,
		}
		if err := s.appendEvent(ctx, dbTx, userID, "OVERRIDE_TX", overrideTx.ID, eventPayload); err != nil {
			s.logger.Warn("failed to append event log",
				slog.String("user_id", userID),
				slog.String("transaction_id", overrideTx.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	// Commit the transaction
	if err := dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &overrideTx, nil
}

// GetHistory retrieves the full history of a transaction, including the original
// and all override transactions in chronological order.
// Verifies that the transaction belongs to the specified user.
// Requirements: 5.5, 7.5
func (s *transactionService) GetHistory(ctx context.Context, txID string, userID string) ([]domain.Transaction, error) {
	// Fetch the history from the repository
	history, err := s.transactionRepo.GetHistory(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction history: %w", err)
	}

	// Verify ownership: check that the first transaction (root) belongs to the user
	// All transactions in the chain must belong to the same user by design,
	// so we only need to check the first one.
	if len(history) == 0 {
		return nil, domain.ErrNotFound
	}

	if history[0].UserID != userID {
		return nil, fmt.Errorf("%w: transaction does not belong to user", domain.ErrNotFound)
	}

	return history, nil
}

// sourceTypePtr returns a pointer to the given SourceType.
// Helper for setting nullable SourceType fields.
func sourceTypePtr(st domain.SourceType) *domain.SourceType {
	return &st
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
