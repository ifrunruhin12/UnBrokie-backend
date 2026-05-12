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

func TestAccountRepository_InsertAndGet(t *testing.T) {
	pool := SetupTestDB(t)
	defer cleanupTestDB(t, pool)
	truncateTables(t, pool)

	userRepo := NewUserRepository(pool)
	accountRepo := NewAccountRepository(pool)
	ctx := context.Background()

	// Create a test user first
	userID := uuid.New().String()
	user := domain.User{
		ID:           userID,
		Email:        "account-test@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	err := userRepo.Insert(ctx, pool, user)
	require.NoError(t, err)

	tests := []struct {
		name    string
		account domain.Account
	}{
		{
			name: "insert and get account",
			account: domain.Account{
				ID:              uuid.New().String(),
				UserID:          userID,
				StartingBalance: 10000,
				CurrentBalance:  10000,
				BalanceDirty:    false,
				Currency:        "BDT",
				Timezone:        "Asia/Dhaka",
				CreatedAt:       time.Now().UTC(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Insert account
			err := accountRepo.Insert(ctx, pool, tt.account)
			require.NoError(t, err)

			// Get by user ID
			retrieved, err := accountRepo.GetByUserID(ctx, userID)
			require.NoError(t, err)
			assert.Equal(t, tt.account.ID, retrieved.ID)
			assert.Equal(t, tt.account.UserID, retrieved.UserID)
			assert.Equal(t, tt.account.StartingBalance, retrieved.StartingBalance)
			assert.Equal(t, tt.account.Timezone, retrieved.Timezone)
		})
	}
}

func TestAccountRepository_ReconcileBalance(t *testing.T) {
	pool := SetupTestDB(t)
	defer cleanupTestDB(t, pool)
	truncateTables(t, pool)

	userRepo := NewUserRepository(pool)
	accountRepo := NewAccountRepository(pool)
	ctx := context.Background()

	// Create a test user
	userID := uuid.New().String()
	user := domain.User{
		ID:           userID,
		Email:        "reconcile-test@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	err := userRepo.Insert(ctx, pool, user)
	require.NoError(t, err)

	// Create an account with dirty balance
	account := domain.Account{
		ID:              uuid.New().String(),
		UserID:          userID,
		StartingBalance: 10000,
		CurrentBalance:  9500, // stale/incorrect cached value
		BalanceDirty:    true,
		Currency:        "BDT",
		Timezone:        "UTC",
		CreatedAt:       time.Now().UTC(),
	}
	err = accountRepo.Insert(ctx, pool, account)
	require.NoError(t, err)

	// Reconcile with the true balance
	trueBalance := 12000
	err = accountRepo.ReconcileBalance(ctx, userID, trueBalance)
	require.NoError(t, err)

	// Verify the balance was updated atomically
	retrieved, err := accountRepo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, trueBalance, retrieved.CurrentBalance, "balance should be set to true balance")
	assert.False(t, retrieved.BalanceDirty, "balance_dirty should be cleared")
	assert.NotNil(t, retrieved.LastReconciledAt, "last_reconciled_at should be set")
	assert.WithinDuration(t, time.Now().UTC(), *retrieved.LastReconciledAt, 2*time.Second, "last_reconciled_at should be recent")
}
