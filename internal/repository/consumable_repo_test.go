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

func TestConsumableRepository_InsertAndGet(t *testing.T) {
	pool := SetupTestDB(t)
	defer cleanupTestDB(t, pool)
	truncateTables(t, pool)

	userRepo := NewUserRepository(pool)
	consumableRepo := NewConsumableRepository(pool)
	ctx := context.Background()

	// Create test user
	userID := uuid.New().String()
	user := domain.User{
		ID:           userID,
		Email:        "consumable-test@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	err := userRepo.Insert(ctx, pool, user)
	require.NoError(t, err)

	tests := []struct {
		name       string
		consumable domain.ConsumableRule
	}{
		{
			name: "insert and get consumable",
			consumable: domain.ConsumableRule{
				ID:               uuid.New().String(),
				UserID:           userID,
				Name:             "Eggs",
				Stock:            30,
				UsagePerDay:      2,
				RestockAmount:    30,
				RestockCost:      300,
				RestockThreshold: 5,
				IsDepleted:       false,
				LastRestockDate:  nil,
				CreatedAt:        time.Now().UTC(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Insert consumable
			err := consumableRepo.Insert(ctx, pool, tt.consumable)
			require.NoError(t, err)

			// Get by ID
			retrieved, err := consumableRepo.GetByID(ctx, tt.consumable.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.consumable.ID, retrieved.ID)
			assert.Equal(t, tt.consumable.Name, retrieved.Name)
			assert.Equal(t, tt.consumable.Stock, retrieved.Stock)

			// List active
			consumables, err := consumableRepo.ListActive(ctx, userID)
			require.NoError(t, err)
			assert.Len(t, consumables, 1)
		})
	}
}
