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
	defer cleanupTestDB(t, pool)
	truncateTables(t, pool)

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
