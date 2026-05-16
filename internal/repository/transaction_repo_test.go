package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifrunruhin12/money-manager/internal/domain"
)

func TestTransactionRepository_InsertAndGet(t *testing.T) {
	pool := SetupTestDB(t)
	defer CleanupTestDB(t, pool)
	TruncateTables(t, pool)

	userRepo := NewUserRepository(pool)
	categoryRepo := NewCategoryRepository(pool)
	txRepo := NewTransactionRepository(pool)
	ctx := context.Background()

	// Create test user
	userID := uuid.New().String()
	user := domain.User{
		ID:           userID,
		Email:        "tx-test@example.com",
		PasswordHash: "hash",
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
		Name:      "Food",
		CreatedAt: time.Now().UTC(),
	}
	err = categoryRepo.Insert(ctx, pool, category)
	require.NoError(t, err)

	tests := []struct {
		name        string
		transaction domain.Transaction
	}{
		{
			name: "insert and get manual transaction",
			transaction: domain.Transaction{
				ID:         uuid.New().String(),
				UserID:     userID,
				Type:       "manual",
				CategoryID: categoryID,
				Amount:     -500,
				IsSkipped:  false,
				Note:       "Lunch",
				Date:       time.Now().UTC(),
				CreatedAt:  time.Now().UTC(),
				UpdatedAt:  time.Now().UTC(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Insert transaction
			_, err := txRepo.Insert(ctx, pool, tt.transaction)
			require.NoError(t, err)

			// Get by ID
			retrieved, err := txRepo.GetByID(ctx, tt.transaction.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.transaction.ID, retrieved.ID)
			assert.Equal(t, tt.transaction.UserID, retrieved.UserID)
			assert.Equal(t, tt.transaction.Amount, retrieved.Amount)
			assert.Equal(t, tt.transaction.Type, retrieved.Type)
		})
	}
}

func TestTransactionRepository_GetHistory(t *testing.T) {
	pool := SetupTestDB(t)
	defer CleanupTestDB(t, pool)
	TruncateTables(t, pool)

	userRepo := NewUserRepository(pool)
	categoryRepo := NewCategoryRepository(pool)
	txRepo := NewTransactionRepository(pool)
	ctx := context.Background()

	// Create test user
	userID := uuid.New().String()
	user := domain.User{
		ID:           userID,
		Email:        "history-test@example.com",
		PasswordHash: "hash",
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
		Name:      "Food",
		CreatedAt: time.Now().UTC(),
	}
	err = categoryRepo.Insert(ctx, pool, category)
	require.NoError(t, err)

	// Create original transaction
	originalID := uuid.New().String()
	original := domain.Transaction{
		ID:           originalID,
		UserID:       userID,
		Type:         domain.TransactionTypeRuleGenerated,
		CategoryID:   categoryID,
		Amount:       -100,
		IsSkipped:    false,
		IsOverridden: false,
		Note:         "Original",
		Date:         time.Now().UTC(),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_, err = txRepo.Insert(ctx, pool, original)
	require.NoError(t, err)

	// Create first override
	time.Sleep(10 * time.Millisecond) // Ensure different created_at
	override1ID := uuid.New().String()
	sourceType := domain.SourceTypeTransaction
	override1 := domain.Transaction{
		ID:           override1ID,
		UserID:       userID,
		Type:         domain.TransactionTypeOverride,
		CategoryID:   categoryID,
		Amount:       -150,
		IsSkipped:    false,
		IsOverridden: false,
		SourceID:     &originalID,
		SourceType:   &sourceType,
		Note:         "First override",
		Date:         time.Now().UTC(),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_, err = txRepo.Insert(ctx, pool, override1)
	require.NoError(t, err)

	// Create second override
	time.Sleep(10 * time.Millisecond) // Ensure different created_at
	override2ID := uuid.New().String()
	override2 := domain.Transaction{
		ID:           override2ID,
		UserID:       userID,
		Type:         domain.TransactionTypeOverride,
		CategoryID:   categoryID,
		Amount:       -200,
		IsSkipped:    false,
		IsOverridden: false,
		SourceID:     &originalID,
		SourceType:   &sourceType,
		Note:         "Second override",
		Date:         time.Now().UTC(),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_, err = txRepo.Insert(ctx, pool, override2)
	require.NoError(t, err)

	t.Run("get history from original transaction", func(t *testing.T) {
		history, err := txRepo.GetHistory(ctx, originalID)
		require.NoError(t, err)
		require.Len(t, history, 3)

		// Verify chronological order (original → override1 → override2)
		assert.Equal(t, originalID, history[0].ID)
		assert.Equal(t, "Original", history[0].Note)

		assert.Equal(t, override1ID, history[1].ID)
		assert.Equal(t, "First override", history[1].Note)

		assert.Equal(t, override2ID, history[2].ID)
		assert.Equal(t, "Second override", history[2].Note)
	})

	t.Run("get history from override transaction", func(t *testing.T) {
		// Should return the same history when querying from an override
		history, err := txRepo.GetHistory(ctx, override1ID)
		require.NoError(t, err)
		require.Len(t, history, 3)

		// Verify same chronological order
		assert.Equal(t, originalID, history[0].ID)
		assert.Equal(t, override1ID, history[1].ID)
		assert.Equal(t, override2ID, history[2].ID)
	})

	t.Run("get history for non-existent transaction", func(t *testing.T) {
		_, err := txRepo.GetHistory(ctx, uuid.New().String())
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("get history for transaction with no overrides", func(t *testing.T) {
		// Create a standalone transaction
		standaloneID := uuid.New().String()
		standalone := domain.Transaction{
			ID:         standaloneID,
			UserID:     userID,
			Type:       domain.TransactionTypeManual,
			CategoryID: categoryID,
			Amount:     -50,
			Note:       "Standalone",
			Date:       time.Now().UTC(),
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}
		_, err := txRepo.Insert(ctx, pool, standalone)
		require.NoError(t, err)

		history, err := txRepo.GetHistory(ctx, standaloneID)
		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Equal(t, standaloneID, history[0].ID)
	})
}
