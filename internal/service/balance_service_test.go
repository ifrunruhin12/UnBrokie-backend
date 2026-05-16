package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/ifrunruhin12/money-manager/internal/domain"
	"github.com/ifrunruhin12/money-manager/internal/repository"
)

// Property 1: Balance Invariant
// Validates: Requirements 1.2, 1.3, 1.4, 1.5, 5.3
//
// The balance SHALL always equal:
// starting_balance + sum(non-skipped, non-overridden transaction amounts) + sum(big_buy amounts)
//
// This property tests that regardless of the sequence of operations (create transactions,
// skip transactions, override transactions, create big buys), the computed balance always
// matches the mathematical invariant.
func TestProperty_BalanceInvariant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	pool := repository.SetupTestDB(t)
	defer pool.Close()

	rapid.Check(t, func(rt *rapid.T) {
		// Clean state for each iteration
		repository.TruncateTables(t, pool)

		ctx := context.Background()
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

		// Setup repositories
		userRepo := repository.NewUserRepository(pool)
		accountRepo := repository.NewAccountRepository(pool)
		transactionRepo := repository.NewTransactionRepository(pool)
		bigBuyRepo := repository.NewBigBuyRepository(pool)
		categoryRepo := repository.NewCategoryRepository(pool)

		// Create balance service
		balanceService := NewBalanceService(
			accountRepo,
			transactionRepo,
			bigBuyRepo,
			1*time.Minute, // staleness threshold
			logger,
		)

		// Create test user
		userID := uuid.New().String()
		// Generate a simple email without special characters to avoid UTF-8 encoding issues
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}$`).Draw(rt, "email_prefix")
		user := domain.User{
			ID:           userID,
			Email:        emailPrefix + "@example.com",
			PasswordHash: "test_hash",
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		err := userRepo.Insert(ctx, pool, user)
		require.NoError(t, err)

		// Create test category
		categoryID := uuid.New().String()
		category := domain.Category{
			ID:        categoryID,
			UserID:    userID,
			Name:      "Test Category",
			CreatedAt: time.Now().UTC(),
		}
		err = categoryRepo.Insert(ctx, pool, category)
		require.NoError(t, err)

		// Generate random starting balance
		startingBalance := rapid.IntRange(-100000, 100000).Draw(rt, "starting_balance")

		// Create account
		account := domain.Account{
			ID:              uuid.New().String(),
			UserID:          userID,
			StartingBalance: startingBalance,
			CurrentBalance:  startingBalance,
			BalanceDirty:    false,
			Currency:        "BDT",
			Timezone:        "UTC",
			CreatedAt:       time.Now().UTC(),
		}
		err = accountRepo.Insert(ctx, pool, account)
		require.NoError(t, err)

		// Generate arbitrary transactions
		numTransactions := rapid.IntRange(0, 20).Draw(rt, "num_transactions")
		transactions := make([]domain.Transaction, 0, numTransactions)
		expectedTxSum := 0

		for i := 0; i < numTransactions; i++ {
			amount := rapid.IntRange(-10000, 10000).Filter(func(x int) bool { return x != 0 }).Draw(rt, "tx_amount")
			isSkipped := rapid.Bool().Draw(rt, "is_skipped")
			isOverridden := rapid.Bool().Draw(rt, "is_overridden")

			tx := domain.Transaction{
				ID:           uuid.New().String(),
				UserID:       userID,
				Type:         "manual",
				CategoryID:   categoryID,
				Amount:       amount,
				IsSkipped:    isSkipped,
				IsOverridden: isOverridden,
				Date:         time.Now().UTC(),
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			}

			_, err := transactionRepo.Insert(ctx, pool, tx)
			require.NoError(t, err)

			transactions = append(transactions, tx)

			// Only count non-skipped, non-overridden transactions
			if !isSkipped && !isOverridden {
				expectedTxSum += amount
			}
		}

		// Generate arbitrary big buys
		numBigBuys := rapid.IntRange(0, 10).Draw(rt, "num_big_buys")
		expectedBigBuySum := 0

		for i := 0; i < numBigBuys; i++ {
			// Big buys are always negative (expenses)
			amount := rapid.IntRange(-50000, -1).Draw(rt, "big_buy_amount")

			// Generate a simple ASCII title to avoid UTF-8 encoding issues
			titleLen := rapid.IntRange(5, 30).Draw(rt, "title_len")
			title := rapid.StringMatching(`^[a-zA-Z0-9 ]+$`).Filter(func(s string) bool {
				return len(s) >= 5 && len(s) <= 30
			}).Draw(rt, "big_buy_title")
			if len(title) > titleLen {
				title = title[:titleLen]
			}

			bigBuy := domain.BigBuy{
				ID:         uuid.New().String(),
				UserID:     userID,
				Title:      title,
				Amount:     amount,
				CategoryID: categoryID,
				Date:       time.Now().UTC(),
				CreatedAt:  time.Now().UTC(),
			}

			err := bigBuyRepo.Insert(ctx, pool, bigBuy)
			require.NoError(t, err)

			expectedBigBuySum += amount
		}

		// Compute expected balance using the invariant formula
		expectedBalance := startingBalance + expectedTxSum + expectedBigBuySum

		// Get balance from service
		actualBalance, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)

		// Assert the invariant holds
		assert.Equal(t, expectedBalance, actualBalance,
			"Balance invariant violated: starting=%d, txSum=%d, bigBuySum=%d, expected=%d, actual=%d",
			startingBalance, expectedTxSum, expectedBigBuySum, expectedBalance, actualBalance)
	})
}

// Property 2: Starting Balance Round-Trip
// Validates: Requirements 1.1
//
// Setting a starting balance and then reading it back SHALL return the same value.
// This property tests that the starting balance persistence is correct.
func TestProperty_StartingBalanceRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	pool := repository.SetupTestDB(t)
	defer pool.Close()

	rapid.Check(t, func(rt *rapid.T) {
		// Clean state for each iteration
		repository.TruncateTables(t, pool)

		ctx := context.Background()
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

		// Setup repositories
		userRepo := repository.NewUserRepository(pool)
		accountRepo := repository.NewAccountRepository(pool)
		transactionRepo := repository.NewTransactionRepository(pool)
		bigBuyRepo := repository.NewBigBuyRepository(pool)

		// Create balance service
		balanceService := NewBalanceService(
			accountRepo,
			transactionRepo,
			bigBuyRepo,
			1*time.Minute,
			logger,
		)

		// Create test user
		userID := uuid.New().String()
		// Generate a simple email without special characters to avoid UTF-8 encoding issues
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}$`).Draw(rt, "email_prefix")
		user := domain.User{
			ID:           userID,
			Email:        emailPrefix + "@example.com",
			PasswordHash: "test_hash",
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		err := userRepo.Insert(ctx, pool, user)
		require.NoError(t, err)

		// Generate random starting balance
		startingBalance := rapid.IntRange(-1000000, 1000000).Draw(rt, "starting_balance")

		// Create account with the starting balance
		account := domain.Account{
			ID:              uuid.New().String(),
			UserID:          userID,
			StartingBalance: startingBalance,
			CurrentBalance:  startingBalance,
			BalanceDirty:    false,
			Currency:        "BDT",
			Timezone:        "UTC",
			CreatedAt:       time.Now().UTC(),
		}
		err = accountRepo.Insert(ctx, pool, account)
		require.NoError(t, err)

		// Get balance from service (should equal starting balance with no transactions)
		actualBalance, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)

		// Assert round-trip property
		assert.Equal(t, startingBalance, actualBalance,
			"Starting balance round-trip failed: set=%d, got=%d", startingBalance, actualBalance)

		// Now update the starting balance
		newStartingBalance := rapid.IntRange(-1000000, 1000000).Draw(rt, "new_starting_balance")
		err = accountRepo.UpdateStartingBalance(ctx, userID, newStartingBalance)
		require.NoError(t, err)

		// Get balance again (should equal new starting balance)
		actualBalance, err = balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)

		// Assert round-trip property after update
		assert.Equal(t, newStartingBalance, actualBalance,
			"Starting balance round-trip after update failed: set=%d, got=%d", newStartingBalance, actualBalance)
	})
}
