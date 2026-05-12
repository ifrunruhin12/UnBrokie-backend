package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifrunruhin12/money-manager/internal/domain"
	"github.com/ifrunruhin12/money-manager/internal/repository"
)

func TestCategoryService_Create(t *testing.T) {
	db := repository.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewCategoryRepository(db)
	svc := NewCategoryService(repo, db)

	ctx := context.Background()
	userID := "test-user-1"

	t.Run("valid category", func(t *testing.T) {
		cat, err := svc.Create(ctx, userID, "Groceries")
		require.NoError(t, err)
		assert.NotEmpty(t, cat.ID)
		assert.Equal(t, userID, cat.UserID)
		assert.Equal(t, "Groceries", cat.Name)
		assert.False(t, cat.CreatedAt.IsZero())
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := svc.Create(ctx, userID, "")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("name too long", func(t *testing.T) {
		longName := string(make([]byte, 101))
		_, err := svc.Create(ctx, userID, longName)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
		assert.Contains(t, err.Error(), "cannot exceed 100 characters")
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := svc.Create(ctx, userID, "Food")
		require.NoError(t, err)

		_, err = svc.Create(ctx, userID, "Food")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrConflict)
	})
}

func TestCategoryService_Update(t *testing.T) {
	db := repository.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewCategoryRepository(db)
	svc := NewCategoryService(repo, db)

	ctx := context.Background()
	userID := "test-user-2"

	// Create initial category
	cat, err := svc.Create(ctx, userID, "Transport")
	require.NoError(t, err)

	t.Run("valid update", func(t *testing.T) {
		updated, err := svc.Update(ctx, cat.ID, userID, "Transportation")
		require.NoError(t, err)
		assert.Equal(t, "Transportation", updated.Name)
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := svc.Update(ctx, cat.ID, userID, "")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("name too long", func(t *testing.T) {
		longName := string(make([]byte, 101))
		_, err := svc.Update(ctx, cat.ID, userID, longName)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("non-existent category", func(t *testing.T) {
		_, err := svc.Update(ctx, "non-existent-id", userID, "NewName")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestCategoryService_Delete(t *testing.T) {
	db := repository.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewCategoryRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	bbRepo := repository.NewBigBuyRepository(db)
	svc := NewCategoryService(repo, db)

	ctx := context.Background()
	userID := "test-user-3"

	t.Run("delete unreferenced category", func(t *testing.T) {
		cat, err := svc.Create(ctx, userID, "Unused")
		require.NoError(t, err)

		err = svc.Delete(ctx, cat.ID, userID)
		require.NoError(t, err)

		// Verify it's deleted
		cats, err := svc.List(ctx, userID)
		require.NoError(t, err)
		for _, c := range cats {
			assert.NotEqual(t, cat.ID, c.ID)
		}
	})

	t.Run("delete category referenced by transaction", func(t *testing.T) {
		cat, err := svc.Create(ctx, userID, "Referenced")
		require.NoError(t, err)

		// Create a transaction referencing this category
		tx := domain.Transaction{
			ID:         uuid.New().String(),
			UserID:     userID,
			Type:       "manual",
			CategoryID: cat.ID,
			Amount:     -100,
			IsSkipped:  false,
			Date:       time.Now().UTC(),
			CreatedAt:  time.Now().UTC(),
		}
		_, err = txRepo.Insert(ctx, db, tx)
		require.NoError(t, err)

		err = svc.Delete(ctx, cat.ID, userID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrConflict)
		assert.Contains(t, err.Error(), "referenced by one or more transactions")

		// Clean up
		_ = txRepo.SoftDelete(ctx, db, tx.ID)
	})

	t.Run("delete category referenced by big buy", func(t *testing.T) {
		cat, err := svc.Create(ctx, userID, "BigBuyRef")
		require.NoError(t, err)

		// Create a big buy referencing this category
		bb := domain.BigBuy{
			ID:         uuid.New().String(),
			UserID:     userID,
			Title:      "Test Big Buy",
			Amount:     -1000,
			CategoryID: cat.ID,
			Date:       time.Now().UTC(),
			CreatedAt:  time.Now().UTC(),
		}
		err = bbRepo.Insert(ctx, db, bb)
		require.NoError(t, err)

		err = svc.Delete(ctx, cat.ID, userID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrConflict)
		assert.Contains(t, err.Error(), "referenced by one or more big buys")

		// Clean up
		_ = bbRepo.Delete(ctx, db, bb.ID)
	})
}

func TestCategoryService_List(t *testing.T) {
	db := repository.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewCategoryRepository(db)
	svc := NewCategoryService(repo, db)

	ctx := context.Background()
	userID := "test-user-4"

	// Create multiple categories
	_, err := svc.Create(ctx, userID, "Food")
	require.NoError(t, err)
	_, err = svc.Create(ctx, userID, "Transport")
	require.NoError(t, err)
	_, err = svc.Create(ctx, userID, "Health")
	require.NoError(t, err)

	cats, err := svc.List(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, cats, 3)

	// Verify sorted by name
	names := []string{cats[0].Name, cats[1].Name, cats[2].Name}
	assert.Equal(t, []string{"Food", "Health", "Transport"}, names)
}

func TestSeedDefaultCategories(t *testing.T) {
	db := repository.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewCategoryRepository(db)
	ctx := context.Background()
	userID := "test-user-5"

	err := SeedDefaultCategories(ctx, repo, db, userID)
	require.NoError(t, err)

	// Verify all default categories were created
	cats, err := repo.ListByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, cats, 7)

	expectedNames := []string{"Food", "Transport", "Extra Food", "Health", "Big Buy", "Savings", "Hobby"}
	actualNames := make([]string, len(cats))
	for i, cat := range cats {
		actualNames[i] = cat.Name
	}

	for _, expected := range expectedNames {
		assert.Contains(t, actualNames, expected)
	}
}
