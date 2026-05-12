package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/ifrunruhin12/money-manager/internal/domain"
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

// GetBalance returns the current balance for a user.
// If balance_dirty is true OR the balance is stale, it recomputes the true balance.
func (s *balanceService) GetBalance(ctx context.Context, userID string) (int, error) {
	account, err := s.accountRepo.GetByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}

	needsRecompute := account.BalanceDirty || s.isStale(account)

	if !needsRecompute {
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

// computeTrueBalance calculates: starting_balance + SUM(transactions) + SUM(big_buys)
func (s *balanceService) computeTrueBalance(ctx context.Context, userID string, startingBalance int) (int, error) {
	txSum, err := s.transactionRepo.SumNonSkippedNonOverridden(ctx, userID)
	if err != nil {
		return 0, err
	}

	// TODO: Replace with BigBuyRepository.SumAll(ctx, userID) when available
	// Current approach leaks implementation details (date range logic) into service layer.
	// A dedicated SumAll method would:
	//   - Make intent clearer (sum ALL big buys, not "sum within arbitrary range")
	//   - Decouple service from repository query shape
	//   - Simplify this code
	minDate := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	maxDate := time.Date(2100, 12, 31, 23, 59, 59, 999999999, time.UTC)
	bigBuySum, err := s.bigBuyRepo.SumByDateRange(ctx, userID, minDate, maxDate)
	if err != nil {
		return 0, err
	}

	return startingBalance + txSum + bigBuySum, nil
}

// isStale checks if the account balance cache is stale.
func (s *balanceService) isStale(account *domain.Account) bool {
	if account.LastReconciledAt == nil {
		return true
	}
	elapsed := time.Since(*account.LastReconciledAt)
	return elapsed > s.stalenessThresh
}
