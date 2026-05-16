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

// Property 21: Transaction Field Validation
// Validates: Requirements 5.7, 5.8
//
// The system SHALL reject transactions with:
// - zero amount
// - missing category_id
// - missing date
//
// This property tests that all validation rules are enforced consistently.
func TestProperty_TransactionFieldValidation(t *testing.T) {
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
		categoryRepo := repository.NewCategoryRepository(pool)

		// Create transaction service
		transactionService := NewTransactionService(
			pool,
			transactionRepo,
			accountRepo,
			false, // disable event log for tests
			logger,
		)

		// Create test user
		userID := uuid.New().String()
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}`).Draw(rt, "email_prefix")
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

		// Create account
		account := domain.Account{
			ID:              uuid.New().String(),
			UserID:          userID,
			StartingBalance: 0,
			CurrentBalance:  0,
			BalanceDirty:    false,
			Currency:        "BDT",
			Timezone:        "UTC",
			CreatedAt:       time.Now().UTC(),
		}
		err = accountRepo.Insert(ctx, pool, account)
		require.NoError(t, err)

		// Test Case 1: Zero amount should be rejected
		txZeroAmount := domain.Transaction{
			ID:         uuid.New().String(),
			UserID:     userID,
			Type:       domain.TransactionTypeManual,
			CategoryID: categoryID,
			Amount:     0, // INVALID: zero amount
			Date:       time.Now().UTC(),
		}
		_, err = transactionService.Create(ctx, txZeroAmount)
		assert.Error(t, err, "Expected error for zero amount")
		assert.ErrorIs(t, err, domain.ErrValidation, "Expected ErrValidation for zero amount")

		// Test Case 2: Missing category_id should be rejected
		amount := rapid.IntRange(-10000, 10000).Filter(func(x int) bool { return x != 0 }).Draw(rt, "amount")
		txMissingCategory := domain.Transaction{
			ID:         uuid.New().String(),
			UserID:     userID,
			Type:       domain.TransactionTypeManual,
			CategoryID: "", // INVALID: missing category_id
			Amount:     amount,
			Date:       time.Now().UTC(),
		}
		_, err = transactionService.Create(ctx, txMissingCategory)
		assert.Error(t, err, "Expected error for missing category_id")
		assert.ErrorIs(t, err, domain.ErrValidation, "Expected ErrValidation for missing category_id")

		// Test Case 3: Missing date should be rejected
		txMissingDate := domain.Transaction{
			ID:         uuid.New().String(),
			UserID:     userID,
			Type:       domain.TransactionTypeManual,
			CategoryID: categoryID,
			Amount:     amount,
			Date:       time.Time{}, // INVALID: zero time (missing date)
		}
		_, err = transactionService.Create(ctx, txMissingDate)
		assert.Error(t, err, "Expected error for missing date")
		assert.ErrorIs(t, err, domain.ErrValidation, "Expected ErrValidation for missing date")

		// Test Case 4: Valid transaction should succeed
		txValid := domain.Transaction{
			ID:         uuid.New().String(),
			UserID:     userID,
			Type:       domain.TransactionTypeManual,
			CategoryID: categoryID,
			Amount:     amount,
			Date:       time.Now().UTC(),
		}
		created, err := transactionService.Create(ctx, txValid)
		assert.NoError(t, err, "Expected no error for valid transaction")
		assert.NotNil(t, created, "Expected created transaction to be non-nil")
		if created != nil {
			assert.Equal(t, amount, created.Amount, "Amount should match")
			assert.Equal(t, categoryID, created.CategoryID, "CategoryID should match")
			assert.False(t, created.Date.IsZero(), "Date should not be zero")
		}
	})
}

// Property 22: Transaction Date Range Query Sorted Descending
// Validates: Requirements 5.9
//
// The system SHALL return transactions for a given date range sorted by date descending, id descending.
// This property tests that:
// 1. Only transactions within [from, to] are returned
// 2. Results are sorted by date DESC, id DESC
// 3. Pagination works correctly with cursor
func TestProperty_TransactionDateRangeQuerySortedDescending(t *testing.T) {
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
		categoryRepo := repository.NewCategoryRepository(pool)

		// Create transaction service
		transactionService := NewTransactionService(
			pool,
			transactionRepo,
			accountRepo,
			false, // disable event log for tests
			logger,
		)

		// Create test user
		userID := uuid.New().String()
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}`).Draw(rt, "email_prefix")
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

		// Create account
		account := domain.Account{
			ID:              uuid.New().String(),
			UserID:          userID,
			StartingBalance: 0,
			CurrentBalance:  0,
			BalanceDirty:    false,
			Currency:        "BDT",
			Timezone:        "UTC",
			CreatedAt:       time.Now().UTC(),
		}
		err = accountRepo.Insert(ctx, pool, account)
		require.NoError(t, err)

		// Generate a base date and create transactions with varying dates
		baseDate := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		numTransactions := rapid.IntRange(5, 20).Draw(rt, "num_transactions")

		createdTransactions := make([]domain.Transaction, 0, numTransactions)

		for i := 0; i < numTransactions; i++ {
			// Generate dates within a range (some before, some after baseDate)
			dayOffset := rapid.IntRange(-30, 30).Draw(rt, "day_offset")
			txDate := baseDate.AddDate(0, 0, dayOffset)

			amount := rapid.IntRange(-10000, 10000).Filter(func(x int) bool { return x != 0 }).Draw(rt, "amount")

			tx := domain.Transaction{
				ID:         uuid.New().String(),
				UserID:     userID,
				Type:       domain.TransactionTypeManual,
				CategoryID: categoryID,
				Amount:     amount,
				Date:       txDate,
			}

			created, err := transactionService.Create(ctx, tx)
			require.NoError(t, err)
			createdTransactions = append(createdTransactions, *created)

			// Small delay to ensure different timestamps for transactions on the same date
			time.Sleep(1 * time.Millisecond)
		}

		// Define query range: [baseDate - 10 days, baseDate + 10 days]
		fromDate := baseDate.AddDate(0, 0, -10)
		toDate := baseDate.AddDate(0, 0, 10)

		// Query transactions within the date range
		results, cursor, err := transactionService.ListByDateRange(ctx, userID, fromDate, toDate, 100, time.Time{}, "")
		require.NoError(t, err)
		assert.Nil(t, cursor, "Expected no cursor for small result set")

		// Filter expected transactions (those within [fromDate, toDate])
		expectedTransactions := make([]domain.Transaction, 0)
		for _, tx := range createdTransactions {
			if (tx.Date.Equal(fromDate) || tx.Date.After(fromDate)) &&
				(tx.Date.Equal(toDate) || tx.Date.Before(toDate)) {
				expectedTransactions = append(expectedTransactions, tx)
			}
		}

		// Assert: number of results matches expected
		assert.Equal(t, len(expectedTransactions), len(results),
			"Number of results should match transactions within date range")

		// Assert: results are sorted by date DESC, id DESC
		for i := 0; i < len(results)-1; i++ {
			current := results[i]
			next := results[i+1]

			// Date should be descending (current >= next)
			if current.Date.After(next.Date) {
				// OK: current date is after next date
				continue
			} else if current.Date.Equal(next.Date) {
				// Same date: ID should be descending (current >= next)
				assert.True(t, current.ID >= next.ID,
					"For same date, IDs should be sorted descending: current=%s, next=%s",
					current.ID, next.ID)
			} else {
				// current.Date.Before(next.Date) — VIOLATION
				t.Errorf("Transactions not sorted by date descending: current=%v, next=%v",
					current.Date, next.Date)
			}
		}

		// Test pagination if we have enough transactions
		if len(results) > 5 {
			// Query with a limit smaller than total results
			limit := 5
			page1, cursor1, err := transactionService.ListByDateRange(ctx, userID, fromDate, toDate, limit, time.Time{}, "")
			require.NoError(t, err)
			assert.NotNil(t, cursor1, "Expected cursor for paginated results")
			assert.Equal(t, limit, len(page1), "First page should have exactly 'limit' items")

			// Query second page using cursor
			page2, cursor2, err := transactionService.ListByDateRange(ctx, userID, fromDate, toDate, limit, cursor1.Date, cursor1.ID)
			require.NoError(t, err)
			assert.True(t, len(page2) > 0, "Second page should have results")

			// Assert: no overlap between pages
			for _, tx1 := range page1 {
				for _, tx2 := range page2 {
					assert.NotEqual(t, tx1.ID, tx2.ID, "Pages should not overlap")
				}
			}

			// Assert: page2 items come after page1 items in sort order (date DESC, id DESC)
			lastPage1 := page1[len(page1)-1]
			firstPage2 := page2[0]

			// firstPage2 should be "less than" lastPage1 in descending order
			// i.e., firstPage2.Date <= lastPage1.Date
			assert.True(t,
				firstPage2.Date.Before(lastPage1.Date) ||
					(firstPage2.Date.Equal(lastPage1.Date) && firstPage2.ID < lastPage1.ID),
				"Second page should continue from first page in descending order")

			// If there's a cursor2, verify it points to the last item of page2
			if cursor2 != nil {
				lastPage2 := page2[len(page2)-1]
				assert.Equal(t, lastPage2.Date, cursor2.Date, "Cursor should point to last item date")
				assert.Equal(t, lastPage2.ID, cursor2.ID, "Cursor should point to last item ID")
			}
		}
	})
}

// Property 19: Skipped Transactions Excluded from All Calculations
// Validates: Requirements 5.2, 5.6, 7.3, 9.6
//
// The system SHALL exclude skipped transactions from:
// - Balance calculations
// - Projection calculations
// - Dashboard calculations
//
// This property tests that:
// 1. Creating a transaction updates the balance
// 2. Skipping that transaction removes it from the balance
// 3. The balance equals starting_balance + sum(non-skipped transactions)
func TestProperty_SkippedTransactionsExcludedFromCalculations(t *testing.T) {
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
		categoryRepo := repository.NewCategoryRepository(pool)
		bigBuyRepo := repository.NewBigBuyRepository(pool)

		// Create services
		transactionService := NewTransactionService(
			pool,
			transactionRepo,
			accountRepo,
			false, // disable event log for tests
			logger,
		)

		balanceService := NewBalanceService(
			accountRepo,
			transactionRepo,
			bigBuyRepo,
			1*time.Minute, // staleness threshold
			logger,
		)

		// Create test user
		userID := uuid.New().String()
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}`).Draw(rt, "email_prefix")
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

		// Create account with random starting balance
		startingBalance := rapid.IntRange(-100000, 100000).Draw(rt, "starting_balance")
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

		// Generate multiple transactions
		numTransactions := rapid.IntRange(3, 10).Draw(rt, "num_transactions")
		createdTransactions := make([]*domain.Transaction, 0, numTransactions)

		for i := 0; i < numTransactions; i++ {
			amount := rapid.IntRange(-10000, 10000).Filter(func(x int) bool { return x != 0 }).Draw(rt, "amount")
			tx := domain.Transaction{
				ID:         uuid.New().String(),
				UserID:     userID,
				Type:       domain.TransactionTypeManual,
				CategoryID: categoryID,
				Amount:     amount,
				Date:       time.Now().UTC(),
			}

			created, err := transactionService.Create(ctx, tx)
			require.NoError(t, err)
			createdTransactions = append(createdTransactions, created)
		}

		// Get balance after all transactions created
		balanceAfterCreate, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)

		// Calculate expected balance (starting + sum of all transactions)
		expectedBalance := startingBalance
		for _, tx := range createdTransactions {
			expectedBalance += tx.Amount
		}
		assert.Equal(t, expectedBalance, balanceAfterCreate,
			"Balance after creating transactions should equal starting_balance + sum(all transactions)")

		// Skip a random subset of transactions
		numToSkip := rapid.IntRange(1, len(createdTransactions)).Draw(rt, "num_to_skip")
		skippedIndices := make(map[int]bool)
		for i := 0; i < numToSkip; i++ {
			idx := rapid.IntRange(0, len(createdTransactions)-1).Draw(rt, "skip_index")
			if !skippedIndices[idx] {
				skippedIndices[idx] = true
				err := transactionService.Skip(ctx, createdTransactions[idx].ID, userID)
				require.NoError(t, err)
			}
		}

		// Get balance after skipping
		balanceAfterSkip, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)

		// Calculate expected balance (starting + sum of non-skipped transactions)
		expectedBalanceAfterSkip := startingBalance
		for idx, tx := range createdTransactions {
			if !skippedIndices[idx] {
				expectedBalanceAfterSkip += tx.Amount
			}
		}

		assert.Equal(t, expectedBalanceAfterSkip, balanceAfterSkip,
			"Balance after skipping should equal starting_balance + sum(non-skipped transactions)")

		// Verify skipped transactions are marked correctly in the database
		for idx, tx := range createdTransactions {
			fetched, err := transactionRepo.GetByID(ctx, tx.ID)
			require.NoError(t, err)
			if skippedIndices[idx] {
				assert.True(t, fetched.IsSkipped, "Transaction should be marked as skipped")
			} else {
				assert.False(t, fetched.IsSkipped, "Transaction should not be marked as skipped")
			}
		}
	})
}

// Property 20: Override Excludes Original Transaction
// Validates: Requirements 5.5, 7.1, 9.5
//
// The system SHALL:
// 1. Mark the original transaction as overridden
// 2. Exclude the original transaction from balance calculations
// 3. Include only the override amount in balance calculations
//
// This property tests that:
// - Creating an override marks the original as overridden
// - Balance reflects only the override amount, not the original
// - Multiple overrides work correctly (override an override)
func TestProperty_OverrideExcludesOriginalTransaction(t *testing.T) {
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
		categoryRepo := repository.NewCategoryRepository(pool)
		bigBuyRepo := repository.NewBigBuyRepository(pool)

		// Create services
		transactionService := NewTransactionService(
			pool,
			transactionRepo,
			accountRepo,
			false, // disable event log for tests
			logger,
		)

		balanceService := NewBalanceService(
			accountRepo,
			transactionRepo,
			bigBuyRepo,
			1*time.Minute, // staleness threshold
			logger,
		)

		// Create test user
		userID := uuid.New().String()
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}`).Draw(rt, "email_prefix")
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

		// Create account with random starting balance
		startingBalance := rapid.IntRange(-100000, 100000).Draw(rt, "starting_balance")
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

		// Create original transaction
		originalAmount := rapid.IntRange(-10000, 10000).Filter(func(x int) bool { return x != 0 }).Draw(rt, "original_amount")
		originalTx := domain.Transaction{
			ID:         uuid.New().String(),
			UserID:     userID,
			Type:       domain.TransactionTypeManual,
			CategoryID: categoryID,
			Amount:     originalAmount,
			Date:       time.Now().UTC(),
		}

		created, err := transactionService.Create(ctx, originalTx)
		require.NoError(t, err)

		// Get balance after original transaction
		balanceAfterOriginal, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, startingBalance+originalAmount, balanceAfterOriginal,
			"Balance should include original transaction")

		// Create override with different amount
		overrideAmount := rapid.IntRange(-10000, 10000).Filter(func(x int) bool { return x != 0 && x != originalAmount }).Draw(rt, "override_amount")
		overrideTx, err := transactionService.Override(ctx, created.ID, userID, overrideAmount, "Override note")
		require.NoError(t, err)
		require.NotNil(t, overrideTx)

		// Get balance after override
		balanceAfterOverride, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)

		// Balance should be: starting_balance + override_amount (original excluded)
		expectedBalance := startingBalance + overrideAmount
		assert.Equal(t, expectedBalance, balanceAfterOverride,
			"Balance should include only override amount, not original")

		// Verify original transaction is marked as overridden
		fetchedOriginal, err := transactionRepo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, fetchedOriginal.IsOverridden, "Original transaction should be marked as overridden")

		// Verify override transaction has correct source_id and source_type
		assert.NotNil(t, overrideTx.SourceID, "Override should have source_id")
		assert.Equal(t, created.ID, *overrideTx.SourceID, "Override source_id should point to original")
		assert.NotNil(t, overrideTx.SourceType, "Override should have source_type")
		assert.Equal(t, domain.SourceTypeTransaction, *overrideTx.SourceType, "Override source_type should be 'transaction'")

		// Test overriding an override (chained override)
		secondOverrideAmount := rapid.IntRange(-10000, 10000).Filter(func(x int) bool { return x != 0 }).Draw(rt, "second_override_amount")
		secondOverrideTx, err := transactionService.Override(ctx, overrideTx.ID, userID, secondOverrideAmount, "Second override")
		require.NoError(t, err)
		require.NotNil(t, secondOverrideTx)

		// Get balance after second override
		balanceAfterSecondOverride, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)

		// Balance should be: starting_balance + second_override_amount (both original and first override excluded)
		expectedBalanceAfterSecond := startingBalance + secondOverrideAmount
		assert.Equal(t, expectedBalanceAfterSecond, balanceAfterSecondOverride,
			"Balance should include only second override amount")

		// Verify first override is now marked as overridden
		fetchedFirstOverride, err := transactionRepo.GetByID(ctx, overrideTx.ID)
		require.NoError(t, err)
		assert.True(t, fetchedFirstOverride.IsOverridden, "First override should be marked as overridden")

		// Verify second override points to the root (original transaction)
		assert.NotNil(t, secondOverrideTx.SourceID, "Second override should have source_id")
		assert.Equal(t, created.ID, *secondOverrideTx.SourceID, "Second override should point to root (original)")
	})
}

// Property 24: Skip Then Restore Round-Trip
// Validates: Requirements 7.4
//
// The system SHALL support restoring a previously skipped transaction.
// This property tests that:
// 1. Skip → Restore returns the transaction to its original state
// 2. Balance is correctly adjusted through the skip/restore cycle
// 3. Multiple skip/restore cycles are idempotent
func TestProperty_SkipThenRestoreRoundTrip(t *testing.T) {
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
		categoryRepo := repository.NewCategoryRepository(pool)
		bigBuyRepo := repository.NewBigBuyRepository(pool)

		// Create services
		transactionService := NewTransactionService(
			pool,
			transactionRepo,
			accountRepo,
			false, // disable event log for tests
			logger,
		)

		balanceService := NewBalanceService(
			accountRepo,
			transactionRepo,
			bigBuyRepo,
			1*time.Minute, // staleness threshold
			logger,
		)

		// Create test user
		userID := uuid.New().String()
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}`).Draw(rt, "email_prefix")
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

		// Create account with random starting balance
		startingBalance := rapid.IntRange(-100000, 100000).Draw(rt, "starting_balance")
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

		// Create transaction
		amount := rapid.IntRange(-10000, 10000).Filter(func(x int) bool { return x != 0 }).Draw(rt, "amount")
		tx := domain.Transaction{
			ID:         uuid.New().String(),
			UserID:     userID,
			Type:       domain.TransactionTypeManual,
			CategoryID: categoryID,
			Amount:     amount,
			Date:       time.Now().UTC(),
		}

		created, err := transactionService.Create(ctx, tx)
		require.NoError(t, err)

		// Get balance after creation
		balanceAfterCreate, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)
		expectedBalanceAfterCreate := startingBalance + amount
		assert.Equal(t, expectedBalanceAfterCreate, balanceAfterCreate,
			"Balance after create should be starting_balance + amount")

		// Skip the transaction
		err = transactionService.Skip(ctx, created.ID, userID)
		require.NoError(t, err)

		// Get balance after skip
		balanceAfterSkip, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, startingBalance, balanceAfterSkip,
			"Balance after skip should equal starting_balance (transaction excluded)")

		// Verify transaction is marked as skipped
		fetchedAfterSkip, err := transactionRepo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, fetchedAfterSkip.IsSkipped, "Transaction should be marked as skipped")

		// Restore the transaction
		err = transactionService.Restore(ctx, created.ID, userID)
		require.NoError(t, err)

		// Get balance after restore
		balanceAfterRestore, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedBalanceAfterCreate, balanceAfterRestore,
			"Balance after restore should equal original balance (starting_balance + amount)")

		// Verify transaction is no longer marked as skipped
		fetchedAfterRestore, err := transactionRepo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.False(t, fetchedAfterRestore.IsSkipped, "Transaction should not be marked as skipped after restore")

		// Test idempotency: skip again
		err = transactionService.Skip(ctx, created.ID, userID)
		require.NoError(t, err)

		balanceAfterSecondSkip, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, startingBalance, balanceAfterSecondSkip,
			"Balance after second skip should equal starting_balance")

		// Restore again
		err = transactionService.Restore(ctx, created.ID, userID)
		require.NoError(t, err)

		balanceAfterSecondRestore, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedBalanceAfterCreate, balanceAfterSecondRestore,
			"Balance after second restore should equal original balance")

		// Test idempotency of skip (calling skip on already skipped transaction)
		err = transactionService.Skip(ctx, created.ID, userID)
		require.NoError(t, err)
		err = transactionService.Skip(ctx, created.ID, userID)
		require.NoError(t, err) // Should not error

		balanceAfterDoubleSkip, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, startingBalance, balanceAfterDoubleSkip,
			"Balance should remain at starting_balance after double skip")

		// Test idempotency of restore (calling restore on already active transaction)
		err = transactionService.Restore(ctx, created.ID, userID)
		require.NoError(t, err)
		err = transactionService.Restore(ctx, created.ID, userID)
		require.NoError(t, err) // Should not error

		balanceAfterDoubleRestore, err := balanceService.GetBalance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedBalanceAfterCreate, balanceAfterDoubleRestore,
			"Balance should remain at starting_balance + amount after double restore")
	})
}

// Property 25: Original Record Preserved After Override or Skip
// Validates: Requirements 7.5
//
// The system SHALL preserve the original transaction record when an override or skip is applied.
// This property tests that:
// 1. Original transaction data is unchanged after skip
// 2. Original transaction data is unchanged after override
// 3. Transaction history includes all versions (original + overrides)
func TestProperty_OriginalRecordPreservedAfterOverrideOrSkip(t *testing.T) {
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
		categoryRepo := repository.NewCategoryRepository(pool)

		// Create services
		transactionService := NewTransactionService(
			pool,
			transactionRepo,
			accountRepo,
			false, // disable event log for tests
			logger,
		)

		// Create test user
		userID := uuid.New().String()
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}`).Draw(rt, "email_prefix")
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

		// Create account
		account := domain.Account{
			ID:              uuid.New().String(),
			UserID:          userID,
			StartingBalance: 0,
			CurrentBalance:  0,
			BalanceDirty:    false,
			Currency:        "BDT",
			Timezone:        "UTC",
			CreatedAt:       time.Now().UTC(),
		}
		err = accountRepo.Insert(ctx, pool, account)
		require.NoError(t, err)

		// Create original transaction
		originalAmount := rapid.IntRange(-10000, 10000).Filter(func(x int) bool { return x != 0 }).Draw(rt, "original_amount")
		originalNote := rapid.StringMatching(`^[a-zA-Z0-9 ]{0,50}`).Draw(rt, "original_note")
		originalTx := domain.Transaction{
			ID:         uuid.New().String(),
			UserID:     userID,
			Type:       domain.TransactionTypeManual,
			CategoryID: categoryID,
			Amount:     originalAmount,
			Note:       originalNote,
			Date:       time.Now().UTC(),
		}

		created, err := transactionService.Create(ctx, originalTx)
		require.NoError(t, err)

		// Snapshot original transaction data
		originalID := created.ID
		originalAmountSnapshot := created.Amount
		originalNoteSnapshot := created.Note
		originalCategorySnapshot := created.CategoryID
		originalDateSnapshot := created.Date

		// Test Case 1: Skip preserves original data (except is_skipped flag)
		err = transactionService.Skip(ctx, created.ID, userID)
		require.NoError(t, err)

		fetchedAfterSkip, err := transactionRepo.GetByID(ctx, created.ID)
		require.NoError(t, err)

		assert.Equal(t, originalID, fetchedAfterSkip.ID, "ID should be unchanged")
		assert.Equal(t, originalAmountSnapshot, fetchedAfterSkip.Amount, "Amount should be unchanged")
		assert.Equal(t, originalNoteSnapshot, fetchedAfterSkip.Note, "Note should be unchanged")
		assert.Equal(t, originalCategorySnapshot, fetchedAfterSkip.CategoryID, "CategoryID should be unchanged")
		assert.Equal(t, originalDateSnapshot.Unix(), fetchedAfterSkip.Date.Unix(), "Date should be unchanged")
		assert.True(t, fetchedAfterSkip.IsSkipped, "IsSkipped should be true")

		// Restore for next test
		err = transactionService.Restore(ctx, created.ID, userID)
		require.NoError(t, err)

		// Test Case 2: Override preserves original data (except is_overridden flag)
		overrideAmount := rapid.IntRange(-10000, 10000).Filter(func(x int) bool { return x != 0 }).Draw(rt, "override_amount")
		overrideTx, err := transactionService.Override(ctx, created.ID, userID, overrideAmount, "Override note")
		require.NoError(t, err)
		require.NotNil(t, overrideTx)

		fetchedAfterOverride, err := transactionRepo.GetByID(ctx, created.ID)
		require.NoError(t, err)

		assert.Equal(t, originalID, fetchedAfterOverride.ID, "ID should be unchanged")
		assert.Equal(t, originalAmountSnapshot, fetchedAfterOverride.Amount, "Amount should be unchanged")
		assert.Equal(t, originalNoteSnapshot, fetchedAfterOverride.Note, "Note should be unchanged")
		assert.Equal(t, originalCategorySnapshot, fetchedAfterOverride.CategoryID, "CategoryID should be unchanged")
		assert.Equal(t, originalDateSnapshot.Unix(), fetchedAfterOverride.Date.Unix(), "Date should be unchanged")
		assert.True(t, fetchedAfterOverride.IsOverridden, "IsOverridden should be true")

		// Test Case 3: Transaction history includes all versions
		history, err := transactionService.GetHistory(ctx, created.ID, userID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(history), 2, "History should include at least original + override")

		// First entry should be the original transaction
		assert.Equal(t, originalID, history[0].ID, "First history entry should be original transaction")
		assert.Equal(t, originalAmountSnapshot, history[0].Amount, "Original amount preserved in history")

		// Last entry should be the override transaction
		lastEntry := history[len(history)-1]
		assert.Equal(t, overrideTx.ID, lastEntry.ID, "Last history entry should be override transaction")
		assert.Equal(t, overrideAmount, lastEntry.Amount, "Override amount in history")

		// Test Case 4: Create another override and verify history grows
		secondOverrideAmount := rapid.IntRange(-10000, 10000).Filter(func(x int) bool { return x != 0 }).Draw(rt, "second_override_amount")
		secondOverrideTx, err := transactionService.Override(ctx, overrideTx.ID, userID, secondOverrideAmount, "Second override")
		require.NoError(t, err)

		historyAfterSecond, err := transactionService.GetHistory(ctx, created.ID, userID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(historyAfterSecond), 3, "History should include original + 2 overrides")

		// Original should still be first
		assert.Equal(t, originalID, historyAfterSecond[0].ID, "Original should still be first in history")
		assert.Equal(t, originalAmountSnapshot, historyAfterSecond[0].Amount, "Original amount still preserved")

		// Second override should be last
		lastEntryAfterSecond := historyAfterSecond[len(historyAfterSecond)-1]
		assert.Equal(t, secondOverrideTx.ID, lastEntryAfterSecond.ID, "Second override should be last in history")
		assert.Equal(t, secondOverrideAmount, lastEntryAfterSecond.Amount, "Second override amount in history")
	})
}
