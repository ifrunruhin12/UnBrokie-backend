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

func TestCategoryRepository_InsertAndList(t *testing.T) {
	pool := SetupTestDB(t)
	defer cleanupTestDB(t, pool)
	truncateTables(t, pool)

	userRepo := NewUserRepository(pool)
	categoryRepo := NewCategoryRepository(pool)
	ctx := context.Background()

	// Create a test user first
	userID := uuid.New().String()
	user := domain.User{
		ID:           userID,
		Email:        "category-test@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	err := userRepo.Insert(ctx, pool, user)
	require.NoError(t, err)

	tests := []struct {
		name     string
		category domain.Category
	}{
		{
			name: "insert and list categories",
			category: domain.Category{
				ID:        uuid.New().String(),
				UserID:    userID,
				Name:      "Food",
				CreatedAt: time.Now().UTC(),
			},
		},
		{
			name: "insert second category",
			category: domain.Category{
				ID:        uuid.New().String(),
				UserID:    userID,
				Name:      "Transport",
				CreatedAt: time.Now().UTC(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Insert category
			err := categoryRepo.Insert(ctx, pool, tt.category)
			require.NoError(t, err)
		})
	}

	// List all categories for user
	categories, err := categoryRepo.ListByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, categories, 2)
}
