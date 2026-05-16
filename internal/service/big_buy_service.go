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

// BigBuyService defines business logic for big buy operations.
// Big buys are high-value, one-time or infrequent purchases tracked separately
// from regular transactions and included in monthly projections.
type BigBuyService interface {
	Create(ctx context.Context, b domain.BigBuy) (*domain.BigBuy, error)
	Update(ctx context.Context, b domain.BigBuy) (*domain.BigBuy, error)
	Delete(ctx context.Context, id string, userID string) error
	ListByMonth(ctx context.Context, userID string, year int, month int) ([]domain.BigBuy, error)
}

type bigBuyService struct {
	db             *pgxpool.Pool
	bigBuyRepo     repository.BigBuyRepository
	accountRepo    repository.AccountRepository
	enableEventLog bool
	logger         *slog.Logger
}

// NewBigBuyService creates a new BigBuyService with the given dependencies.
func NewBigBuyService(
	db *pgxpool.Pool,
	bigBuyRepo repository.BigBuyRepository,
	accountRepo repository.AccountRepository,
	enableEventLog bool,
	logger *slog.Logger,
) BigBuyService {
	return &bigBuyService{
		db:             db,
		bigBuyRepo:     bigBuyRepo,
		accountRepo:    accountRepo,
		enableEventLog: enableEventLog,
		logger:         logger,
	}
}

// Create validates and persists a new big buy entry, atomically updating the account balance.
// Validation rules:
//   - amount must be negative (stored as expense)
//   - title must be non-empty
//   - date must be non-zero
//   - category_id must be non-empty
//
// ALL operations (insert big buy, update balance, append event) execute inside a single
// database transaction. If ANY step fails, the entire operation is rolled back — no partial state.
// Requirements: 8.1, 8.2
func (s *bigBuyService) Create(ctx context.Context, b domain.BigBuy) (*domain.BigBuy, error) {
	// Validation
	// Amount must be negative (stored as expense)
	if b.Amount >= 0 {
		return nil, fmt.Errorf("%w: amount must be negative (expense)", domain.ErrValidation)
	}
	if b.Title == "" {
		return nil, fmt.Errorf("%w: title is required", domain.ErrValidation)
	}
	if b.Date.IsZero() {
		return nil, fmt.Errorf("%w: date is required", domain.ErrValidation)
	}
	if b.CategoryID == "" {
		return nil, fmt.Errorf("%w: category_id is required", domain.ErrValidation)
	}

	// Set defaults
	now := time.Now().UTC()
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	b.CreatedAt = now

	// Begin database transaction — ALL OR NOTHING
	dbTx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer dbTx.Rollback(ctx) // Safe to call even after commit; no-op if already committed

	// Insert big buy
	if err := s.bigBuyRepo.Insert(ctx, dbTx, b); err != nil {
		return nil, fmt.Errorf("failed to insert big buy: %w", err)
	}

	// Atomically update account balance
	// Amount is negative (expense), so this reduces the balance
	// This uses: UPDATE accounts SET current_balance = current_balance + $1 WHERE user_id = $2
	// Single atomic SQL statement — no race conditions.
	if err := s.accountRepo.AdjustBalance(ctx, dbTx, b.UserID, b.Amount); err != nil {
		return nil, fmt.Errorf("failed to update account balance: %w", err)
	}

	// Append event log entry if enabled
	if s.enableEventLog {
		if err := s.appendEvent(ctx, dbTx, b.UserID, "CREATE_BIG_BUY", b.ID, b); err != nil {
			s.logger.Warn("failed to append event log",
				slog.String("user_id", b.UserID),
				slog.String("big_buy_id", b.ID),
				slog.String("error", err.Error()),
			)
			// Continue — event log is best-effort for MVP
		}
	}

	// Commit the transaction — all operations succeed together
	if err := dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &b, nil
}

// Update modifies an existing big buy entry and adjusts the account balance accordingly.
// The balance delta is calculated as (newAmount - oldAmount) and applied atomically.
// Sets balance_dirty = true to trigger recompute on next read.
// Requirements: 8.4
func (s *bigBuyService) Update(ctx context.Context, b domain.BigBuy) (*domain.BigBuy, error) {
	// Validation
	// Amount must be negative (stored as expense)
	if b.Amount >= 0 {
		return nil, fmt.Errorf("%w: amount must be negative (expense)", domain.ErrValidation)
	}
	if b.Title == "" {
		return nil, fmt.Errorf("%w: title is required", domain.ErrValidation)
	}
	if b.Date.IsZero() {
		return nil, fmt.Errorf("%w: date is required", domain.ErrValidation)
	}
	if b.CategoryID == "" {
		return nil, fmt.Errorf("%w: category_id is required", domain.ErrValidation)
	}

	// Begin database transaction — ALL OR NOTHING
	dbTx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer dbTx.Rollback(ctx)

	// Load existing big buy to get the old amount
	// Use FOR UPDATE to prevent concurrent modifications (race condition protection)
	existing, err := s.bigBuyRepo.GetByIDForUpdate(ctx, dbTx, b.ID, b.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing big buy: %w", err)
	}

	// Update the big buy
	if err := s.bigBuyRepo.Update(ctx, dbTx, b); err != nil {
		return nil, fmt.Errorf("failed to update big buy: %w", err)
	}

	// Calculate balance delta: (newAmount - oldAmount)
	// Both amounts are negative, so if newAmount is more negative, delta is negative (reduces balance)
	balanceDelta := b.Amount - existing.Amount
	if balanceDelta != 0 {
		if err := s.accountRepo.AdjustBalance(ctx, dbTx, b.UserID, balanceDelta); err != nil {
			return nil, fmt.Errorf("failed to adjust balance: %w", err)
		}
	}

	// Set balance_dirty = true to trigger recompute on next read
	// This is a safety measure for complex multi-table operations
	if err := s.accountRepo.SetDirty(ctx, dbTx, b.UserID, true); err != nil {
		return nil, fmt.Errorf("failed to set balance dirty: %w", err)
	}

	// Append event log entry if enabled
	if s.enableEventLog {
		eventPayload := map[string]interface{}{
			"big_buy":       b,
			"old_amount":    existing.Amount,
			"balance_delta": balanceDelta,
		}
		if err := s.appendEvent(ctx, dbTx, b.UserID, "UPDATE_BIG_BUY", b.ID, eventPayload); err != nil {
			s.logger.Warn("failed to append event log",
				slog.String("user_id", b.UserID),
				slog.String("big_buy_id", b.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	// Commit the transaction
	if err := dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &b, nil
}

// Delete soft-deletes a big buy entry (sets deleted_at = NOW()) and atomically
// reverses its balance contribution.
// Requirements: 8.5
func (s *bigBuyService) Delete(ctx context.Context, id string, userID string) error {
	// Begin database transaction — ALL OR NOTHING
	dbTx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer dbTx.Rollback(ctx)

	// Load the big buy to get the amount before deletion
	// Use FOR UPDATE to prevent concurrent modifications (race condition protection)
	existing, err := s.bigBuyRepo.GetByIDForUpdate(ctx, dbTx, id, userID)
	if err != nil {
		return fmt.Errorf("failed to load big buy: %w", err)
	}

	// Soft delete the big buy
	if err := s.bigBuyRepo.Delete(ctx, dbTx, id); err != nil {
		return fmt.Errorf("failed to delete big buy: %w", err)
	}

	// Atomically reverse the balance contribution
	// Amount is negative (expense), so -amount is positive (reverses the expense)
	if err := s.accountRepo.AdjustBalance(ctx, dbTx, userID, -existing.Amount); err != nil {
		return fmt.Errorf("failed to adjust balance: %w", err)
	}

	// Set balance_dirty = true to trigger recompute on next read
	// This is a safety measure for complex multi-table operations
	if err := s.accountRepo.SetDirty(ctx, dbTx, userID, true); err != nil {
		return fmt.Errorf("failed to set balance dirty: %w", err)
	}

	// Append event log entry if enabled
	if s.enableEventLog {
		eventPayload := map[string]interface{}{
			"big_buy_id": id,
			"amount":     existing.Amount,
		}
		if err := s.appendEvent(ctx, dbTx, userID, "DELETE_BIG_BUY", id, eventPayload); err != nil {
			s.logger.Warn("failed to append event log",
				slog.String("user_id", userID),
				slog.String("big_buy_id", id),
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

// ListByMonth retrieves all non-deleted big buys for a user in the given month,
// sorted by date ascending.
// Requirements: 8.6
func (s *bigBuyService) ListByMonth(ctx context.Context, userID string, year int, month int) ([]domain.BigBuy, error) {
	buys, err := s.bigBuyRepo.ListByMonth(ctx, userID, year, month)
	if err != nil {
		return nil, fmt.Errorf("failed to list big buys: %w", err)
	}
	return buys, nil
}

// appendEvent writes an event log entry to the events table within the given transaction.
// This ensures event log entries are part of the same atomic transaction as the operation they log.
// Failures are logged but do not block the transaction for MVP.
func (s *bigBuyService) appendEvent(ctx context.Context, db repository.DBTX, userID, eventType, entityID string, payload interface{}) error {
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
