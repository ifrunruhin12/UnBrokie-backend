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

func TestRuleRepository_InsertAndList(t *testing.T) {
	pool := SetupTestDB(t)
	defer CleanupTestDB(t, pool)
	TruncateTables(t, pool)

	userRepo := NewUserRepository(pool)
	categoryRepo := NewCategoryRepository(pool)
	ruleRepo := NewRuleRepository(pool)
	ctx := context.Background()

	// Create test user
	userID := uuid.New().String()
	user := domain.User{
		ID:           userID,
		Email:        "rule-test@example.com",
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
		Name:      "Transport",
		CreatedAt: time.Now().UTC(),
	}
	err = categoryRepo.Insert(ctx, pool, category)
	require.NoError(t, err)

	tests := []struct {
		name string
		rule domain.FixedRule
	}{
		{
			name: "insert and list fixed rule",
			rule: domain.FixedRule{
				ID:         uuid.New().String(),
				UserID:     userID,
				Name:       "Daily Transport",
				CategoryID: categoryID,
				Amount:     100,
				Frequency:  "daily",
				IsActive:   true,
				CreatedAt:  time.Now().UTC(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Insert rule
			err := ruleRepo.InsertFixed(ctx, pool, tt.rule)
			require.NoError(t, err)

			// List active rules
			rules, err := ruleRepo.ListActiveFixed(ctx, userID)
			require.NoError(t, err)
			assert.Len(t, rules, 1)
			assert.Equal(t, tt.rule.ID, rules[0].ID)
			assert.Equal(t, tt.rule.Name, rules[0].Name)
		})
	}
}
