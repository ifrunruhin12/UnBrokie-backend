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

func TestUserRepository_InsertAndGet(t *testing.T) {
	pool := setupTestDB(t)
	defer cleanupTestDB(t, pool)
	truncateTables(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	tests := []struct {
		name  string
		user  domain.User
		email string
	}{
		{
			name: "insert and get by email",
			user: domain.User{
				ID:           uuid.New().String(),
				Email:        "test@example.com",
				PasswordHash: "hashed_password_123",
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			},
			email: "test@example.com",
		},
		{
			name: "insert and get by ID",
			user: domain.User{
				ID:           uuid.New().String(),
				Email:        "another@example.com",
				PasswordHash: "hashed_password_456",
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			},
			email: "another@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Insert user
			err := repo.Insert(ctx, pool, tt.user)
			require.NoError(t, err)

			// Get by email
			retrieved, err := repo.GetByEmail(ctx, tt.email)
			require.NoError(t, err)
			assert.Equal(t, tt.user.ID, retrieved.ID)
			assert.Equal(t, tt.user.Email, retrieved.Email)
			assert.Equal(t, tt.user.PasswordHash, retrieved.PasswordHash)

			// Get by ID
			retrievedByID, err := repo.GetByID(ctx, tt.user.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.user.ID, retrievedByID.ID)
			assert.Equal(t, tt.user.Email, retrievedByID.Email)
		})
	}
}
