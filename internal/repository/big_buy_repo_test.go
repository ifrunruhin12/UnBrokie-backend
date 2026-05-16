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

func TestBigBuyRepository_InsertAndList(t *testing.T) {
	pool := SetupTestDB(t)
	defer CleanupTestDB(t, pool)
	TruncateTables(t, pool)

	userRepo := NewUserRepository(pool)
	categoryRepo := NewCategoryRepository(pool)
	bigBuyRepo := NewBigBuyRepository(pool)
	ctx := context.Background()

	// Create test user
	userID := uuid.New().String()
	user := domain.User{
		ID:           userID,
		Email:        "bigbuy-test@example.com",
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
		Name:      "Big Buy",
		CreatedAt: time.Now().UTC(),
	}
	err = categoryRepo.Insert(ctx, pool, category)
	require.NoError(t, err)

	tests := []struct {
		name   string
		bigBuy domain.BigBuy
	}{
		{
			name: "insert and list big buy",
			bigBuy: domain.BigBuy{
				ID:         uuid.New().String(),
				UserID:     userID,
				Title:      "New Laptop",
				Amount:     -50000,
				CategoryID: categoryID,
				Note:       "Work laptop",
				Date:       time.Now().UTC(),
				CreatedAt:  time.Now().UTC(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Insert big buy
			err := bigBuyRepo.Insert(ctx, pool, tt.bigBuy)
			require.NoError(t, err)

			// List by month
			year, month, _ := tt.bigBuy.Date.Date()
			bigBuys, err := bigBuyRepo.ListByMonth(ctx, userID, year, int(month))
			require.NoError(t, err)
			assert.Len(t, bigBuys, 1)
			assert.Equal(t, tt.bigBuy.ID, bigBuys[0].ID)
			assert.Equal(t, tt.bigBuy.Title, bigBuys[0].Title)
		})
	}
}
