package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/ifrunruhin12/money-manager/internal/repository"
)

// BalanceService defines business logic for balance operations.
//
// Current implementation: synchronous reconciliation on read (fast path only).
// Future evolution: split into fast path (cached) + slow path (async reconciliation job).
// The stalenessThresh parameter enables tuning without code changes:
//   - Dev: aggressive (e.g., 1m) for immediate feedback
//   - Prod: lazy (e.g., 5m) for performance
type BalanceService interface {
	GetBalance(ctx context.Context, userID string) (int, error)
	// SetBalance sets the account's current balance (what GET /balance returns).
	// It adjusts starting_balance so that starting_balance + transactions + big_buys = balance.
	SetBalance(ctx context.Context, userID string, balance int) (int, error)
	// ForceReconcile recomputes balance from the ledger and updates the cache.
	ForceReconcile(ctx context.Context, userID string) (int, error)
}

type balanceService struct {
	accountRepo     repository.AccountRepository
	transactionRepo repository.TransactionRepository
	bigBuyRepo      repository.BigBuyRepository
	stalenessThresh time.Duration
	logger          *slog.Logger
}

// NewBalanceService creates a new BalanceService with the given dependencies.
func NewBalanceService(
	accountRepo repository.AccountRepository,
	transactionRepo repository.TransactionRepository,
	bigBuyRepo repository.BigBuyRepository,
	stalenessThresh time.Duration,
	logger *slog.Logger,
) BalanceService {
	return &balanceService{
		accountRepo:     accountRepo,
		transactionRepo: transactionRepo,
		bigBuyRepo:      bigBuyRepo,
		stalenessThresh: stalenessThresh,
		logger:          logger,
	}
}

// GetBalance returns the persisted current_balance for a user.
// Recomputation runs only when balance_dirty is true (e.g. after overrides).
// This avoids wiping a stored current_balance on every read when starting_balance is 0
// and last_reconciled_at has not been set yet.
func (s *balanceService) GetBalance(ctx context.Context, userID string) (int, error) {
	account, err := s.accountRepo.GetByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}

	if !account.BalanceDirty {
		return account.CurrentBalance, nil
	}

	trueBalance, err := s.computeTrueBalance(ctx, userID, account.StartingBalance)
	if err != nil {
		return 0, err
	}

	if account.CurrentBalance != trueBalance {
		s.logger.Warn("balance mismatch detected",
			slog.String("user_id", userID),
			slog.Int("cached_balance", account.CurrentBalance),
			slog.Int("true_balance", trueBalance),
			slog.Int("delta", trueBalance-account.CurrentBalance),
		)
	}

	// Atomically set the authoritative balance and mark reconciled
	// This avoids race conditions from concurrent mutations between recompute and update
	if err := s.accountRepo.ReconcileBalance(ctx, userID, trueBalance); err != nil {
		return 0, err
	}

	return trueBalance, nil
}

// ForceReconcile always recomputes from the ledger (used by POST /account/reconcile).
func (s *balanceService) ForceReconcile(ctx context.Context, userID string) (int, error) {
	account, err := s.accountRepo.GetByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}

	trueBalance, err := s.computeTrueBalance(ctx, userID, account.StartingBalance)
	if err != nil {
		return 0, err
	}

	if err := s.accountRepo.ReconcileBalance(ctx, userID, trueBalance); err != nil {
		return 0, err
	}

	return trueBalance, nil
}

// SetBalance sets the balance returned by GET /balance to the requested value.
func (s *balanceService) SetBalance(ctx context.Context, userID string, balance int) (int, error) {
	if _, err := s.accountRepo.GetByUserID(ctx, userID); err != nil {
		return 0, err
	}

	txSum, err := s.transactionRepo.SumNonSkippedNonOverridden(ctx, userID)
	if err != nil {
		return 0, err
	}

	bigBuySum, err := s.sumBigBuys(ctx, userID)
	if err != nil {
		return 0, err
	}

	// starting_balance + txSum + bigBuySum must equal the requested balance.
	newStarting := balance - txSum - bigBuySum
	if err := s.accountRepo.UpdateStartingBalance(ctx, userID, newStarting); err != nil {
		return 0, err
	}

	if err := s.accountRepo.ReconcileBalance(ctx, userID, balance); err != nil {
		return 0, err
	}

	return balance, nil
}

// computeTrueBalance calculates: starting_balance + SUM(transactions) + SUM(big_buys)
func (s *balanceService) computeTrueBalance(ctx context.Context, userID string, startingBalance int) (int, error) {
	txSum, err := s.transactionRepo.SumNonSkippedNonOverridden(ctx, userID)
	if err != nil {
		return 0, err
	}

	bigBuySum, err := s.sumBigBuys(ctx, userID)
	if err != nil {
		return 0, err
	}

	return startingBalance + txSum + bigBuySum, nil
}

func (s *balanceService) sumBigBuys(ctx context.Context, userID string) (int, error) {
	// TODO: Replace with BigBuyRepository.SumAll(ctx, userID) when available
	minDate := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	maxDate := time.Date(2100, 12, 31, 23, 59, 59, 999999999, time.UTC)
	return s.bigBuyRepo.SumByDateRange(ctx, userID, minDate, maxDate)
}
