package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/ifrunruhin12/money-manager/internal/domain"
	"github.com/ifrunruhin12/money-manager/internal/repository"
)

func TestCategoryService_Create(t *testing.T) {
	db := repository.SetupTestDB(t)
	defer db.Close()
	repository.TruncateTables(t, db)

	repo := repository.NewCategoryRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewCategoryService(repo, db)

	ctx := context.Background()
	userID := "test-user-1"

	// Create test user
	user := domain.User{
		ID:           userID,
		Email:        "test1@example.com",
		PasswordHash: "test_hash",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	err := userRepo.Insert(ctx, db, user)
	require.NoError(t, err)

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
	repository.TruncateTables(t, db)

	repo := repository.NewCategoryRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewCategoryService(repo, db)

	ctx := context.Background()
	userID := "test-user-2"

	// Create test user
	user := domain.User{
		ID:           userID,
		Email:        "test2@example.com",
		PasswordHash: "test_hash",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	err := userRepo.Insert(ctx, db, user)
	require.NoError(t, err)

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
	repository.TruncateTables(t, db)

	repo := repository.NewCategoryRepository(db)
	userRepo := repository.NewUserRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	bbRepo := repository.NewBigBuyRepository(db)
	svc := NewCategoryService(repo, db)

	ctx := context.Background()
	userID := "test-user-3"

	// Create test user
	user := domain.User{
		ID:           userID,
		Email:        "test3@example.com",
		PasswordHash: "test_hash",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	err := userRepo.Insert(ctx, db, user)
	require.NoError(t, err)

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
	repository.TruncateTables(t, db)

	repo := repository.NewCategoryRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewCategoryService(repo, db)

	ctx := context.Background()
	userID := "test-user-4"

	// Create test user
	user := domain.User{
		ID:           userID,
		Email:        "test4@example.com",
		PasswordHash: "test_hash",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	err := userRepo.Insert(ctx, db, user)
	require.NoError(t, err)

	// Create multiple categories
	_, err = svc.Create(ctx, userID, "Food")
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
	repository.TruncateTables(t, db)

	repo := repository.NewCategoryRepository(db)
	userRepo := repository.NewUserRepository(db)
	ctx := context.Background()
	userID := "test-user-5"

	// Create test user
	user := domain.User{
		ID:           userID,
		Email:        "test5@example.com",
		PasswordHash: "test_hash",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	err := userRepo.Insert(ctx, db, user)
	require.NoError(t, err)

	err = SeedDefaultCategories(ctx, repo, db, userID)
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

// Property 3: Category Creation Round-Trip
// Validates: Requirements 2.2
//
// Creating a category with a valid name and then retrieving it SHALL return
// a category with the same name and user ID.
func TestProperty_CategoryCreationRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	pool := repository.SetupTestDB(t)
	defer pool.Close()

	rapid.Check(t, func(rt *rapid.T) {
		// Clean state for each iteration
		repository.TruncateTables(t, pool)

		ctx := context.Background()
		repo := repository.NewCategoryRepository(pool)
		userRepo := repository.NewUserRepository(pool)
		svc := NewCategoryService(repo, pool)

		// Create test user
		userID := uuid.New().String()
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}$`).Draw(rt, "email_prefix")
		user := domain.User{
			ID:           userID,
			Email:        emailPrefix + "@example.com",
			PasswordHash: "test_hash",
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		err := userRepo.Insert(ctx, pool, user)
		require.NoError(t, err)

		// Generate valid category name (1-100 chars, non-empty after trim)
		// Use ASCII alphanumeric + spaces to avoid encoding issues
		nameLen := rapid.IntRange(1, 100).Draw(rt, "name_len")
		name := rapid.StringMatching(`^[a-zA-Z0-9 ]+$`).
			Filter(func(s string) bool {
				trimmed := strings.TrimSpace(s)
				return len(trimmed) >= 1 && len(trimmed) <= 100
			}).
			Draw(rt, "category_name")

		// Ensure name is within valid length after generation
		name = strings.TrimSpace(name)
		if len(name) > nameLen {
			name = name[:nameLen]
		}
		if len(name) == 0 {
			name = "A" // Ensure non-empty
		}

		// Create category
		created, err := svc.Create(ctx, userID, name)
		require.NoError(t, err)
		require.NotNil(t, created)

		// Verify created category has correct fields
		assert.NotEmpty(t, created.ID)
		assert.Equal(t, userID, created.UserID)
		assert.Equal(t, strings.TrimSpace(name), created.Name)
		assert.False(t, created.CreatedAt.IsZero())

		// Retrieve all categories for user
		cats, err := svc.List(ctx, userID)
		require.NoError(t, err)

		// Find the created category
		var found *domain.Category
		for i := range cats {
			if cats[i].ID == created.ID {
				found = &cats[i]
				break
			}
		}

		// Assert round-trip property
		require.NotNil(t, found, "Created category not found in list")
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, created.UserID, found.UserID)
		assert.Equal(t, created.Name, found.Name)
	})
}

// Property 4: Category Name Uniqueness
// Validates: Requirements 2.5
//
// Creating two categories with the same name for the same user SHALL fail
// with ErrConflict on the second attempt.
func TestProperty_CategoryNameUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	pool := repository.SetupTestDB(t)
	defer pool.Close()

	rapid.Check(t, func(rt *rapid.T) {
		// Clean state for each iteration
		repository.TruncateTables(t, pool)

		ctx := context.Background()
		repo := repository.NewCategoryRepository(pool)
		userRepo := repository.NewUserRepository(pool)
		svc := NewCategoryService(repo, pool)

		// Create test user
		userID := uuid.New().String()
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}$`).Draw(rt, "email_prefix")
		user := domain.User{
			ID:           userID,
			Email:        emailPrefix + "@example.com",
			PasswordHash: "test_hash",
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		err := userRepo.Insert(ctx, pool, user)
		require.NoError(t, err)

		// Generate valid category name
		name := rapid.StringMatching(`^[a-zA-Z0-9 ]+$`).
			Filter(func(s string) bool {
				trimmed := strings.TrimSpace(s)
				return len(trimmed) >= 1 && len(trimmed) <= 100
			}).
			Draw(rt, "category_name")

		name = strings.TrimSpace(name)
		if len(name) == 0 {
			name = "TestCategory"
		}

		// Create first category
		first, err := svc.Create(ctx, userID, name)
		require.NoError(t, err)
		require.NotNil(t, first)

		// Attempt to create second category with same name
		second, err := svc.Create(ctx, userID, name)

		// Assert uniqueness constraint is enforced
		assert.Error(t, err, "Expected error when creating duplicate category")
		assert.ErrorIs(t, err, domain.ErrConflict, "Expected ErrConflict for duplicate category name")
		assert.Nil(t, second, "Second category should be nil on conflict")

		// Verify only one category exists
		cats, err := svc.List(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, cats, 1, "Should have exactly one category after duplicate attempt")
		assert.Equal(t, name, cats[0].Name)
	})
}

// Property 5: Category Deletion Blocked When Referenced
// Validates: Requirements 2.4, 8.2
//
// Deleting a category that is referenced by transactions or big buys SHALL fail
// with ErrConflict. Deleting an unreferenced category SHALL succeed.
func TestProperty_CategoryDeletionBlockedWhenReferenced(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	pool := repository.SetupTestDB(t)
	defer pool.Close()

	rapid.Check(t, func(rt *rapid.T) {
		// Clean state for each iteration
		repository.TruncateTables(t, pool)

		ctx := context.Background()
		repo := repository.NewCategoryRepository(pool)
		userRepo := repository.NewUserRepository(pool)
		txRepo := repository.NewTransactionRepository(pool)
		bbRepo := repository.NewBigBuyRepository(pool)
		svc := NewCategoryService(repo, pool)

		// Create test user
		userID := uuid.New().String()
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}$`).Draw(rt, "email_prefix")
		user := domain.User{
			ID:           userID,
			Email:        emailPrefix + "@example.com",
			PasswordHash: "test_hash",
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		err := userRepo.Insert(ctx, pool, user)
		require.NoError(t, err)

		// Create category
		name := rapid.StringMatching(`^[a-zA-Z]{5,20}$`).Draw(rt, "category_name")
		cat, err := svc.Create(ctx, userID, name)
		require.NoError(t, err)

		// Decide what to reference: 0=nothing, 1=transaction, 2=big_buy, 3=both
		refType := rapid.IntRange(0, 3).Draw(rt, "ref_type")

		if refType == 1 || refType == 3 {
			// Create a transaction referencing this category
			tx := domain.Transaction{
				ID:         uuid.New().String(),
				UserID:     userID,
				Type:       "manual",
				CategoryID: cat.ID,
				Amount:     rapid.IntRange(-10000, -1).Draw(rt, "tx_amount"),
				IsSkipped:  false,
				Date:       time.Now().UTC(),
				CreatedAt:  time.Now().UTC(),
				UpdatedAt:  time.Now().UTC(),
			}
			_, err := txRepo.Insert(ctx, pool, tx)
			require.NoError(t, err)
		}

		if refType == 2 || refType == 3 {
			// Create a big buy referencing this category
			bb := domain.BigBuy{
				ID:         uuid.New().String(),
				UserID:     userID,
				Title:      "Test Big Buy",
				Amount:     rapid.IntRange(-50000, -1).Draw(rt, "bb_amount"),
				CategoryID: cat.ID,
				Date:       time.Now().UTC(),
				CreatedAt:  time.Now().UTC(),
			}
			err := bbRepo.Insert(ctx, pool, bb)
			require.NoError(t, err)
		}

		// Attempt to delete category
		err = svc.Delete(ctx, cat.ID, userID)

		if refType == 0 {
			// No references: deletion should succeed
			assert.NoError(t, err, "Deletion should succeed for unreferenced category")

			// Verify category is deleted
			cats, err := svc.List(ctx, userID)
			require.NoError(t, err)
			for _, c := range cats {
				assert.NotEqual(t, cat.ID, c.ID, "Deleted category should not appear in list")
			}
		} else {
			// Has references: deletion should fail with ErrConflict
			assert.Error(t, err, "Deletion should fail for referenced category")
			assert.ErrorIs(t, err, domain.ErrConflict, "Expected ErrConflict for referenced category")

			// Verify category still exists
			cats, err := svc.List(ctx, userID)
			require.NoError(t, err)
			found := false
			for _, c := range cats {
				if c.ID == cat.ID {
					found = true
					break
				}
			}
			assert.True(t, found, "Referenced category should still exist after failed deletion")
		}
	})
}

// Property 6: Category Name Validation
// Validates: Requirements 2.6
//
// Creating or updating a category with an empty name or a name exceeding 100 characters
// SHALL fail with ErrValidation.
func TestProperty_CategoryNameValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	pool := repository.SetupTestDB(t)
	defer pool.Close()

	rapid.Check(t, func(rt *rapid.T) {
		// Clean state for each iteration
		repository.TruncateTables(t, pool)

		ctx := context.Background()
		repo := repository.NewCategoryRepository(pool)
		userRepo := repository.NewUserRepository(pool)
		svc := NewCategoryService(repo, pool)

		// Create test user
		userID := uuid.New().String()
		emailPrefix := rapid.StringMatching(`^[a-z]{3,10}$`).Draw(rt, "email_prefix")
		user := domain.User{
			ID:           userID,
			Email:        emailPrefix + "@example.com",
			PasswordHash: "test_hash",
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		err := userRepo.Insert(ctx, pool, user)
		require.NoError(t, err)

		// Test invalid names
		// Case 1: Empty string
		emptyName := ""
		_, err = svc.Create(ctx, userID, emptyName)
		assert.Error(t, err, "Empty name should fail validation")
		assert.ErrorIs(t, err, domain.ErrValidation, "Expected ErrValidation for empty name")

		// Case 2: Whitespace-only string
		whitespaceName := rapid.StringMatching(`^[ \t\n]+$`).
			Filter(func(s string) bool { return len(s) > 0 && len(s) <= 10 }).
			Draw(rt, "whitespace_name")
		_, err = svc.Create(ctx, userID, whitespaceName)
		assert.Error(t, err, "Whitespace-only name should fail validation")
		assert.ErrorIs(t, err, domain.ErrValidation, "Expected ErrValidation for whitespace-only name")

		// Case 3: Name exceeding 100 characters
		longNameLen := rapid.IntRange(101, 200).Draw(rt, "long_name_len")
		longName := strings.Repeat("A", longNameLen)
		_, err = svc.Create(ctx, userID, longName)
		assert.Error(t, err, "Name exceeding 100 chars should fail validation")
		assert.ErrorIs(t, err, domain.ErrValidation, "Expected ErrValidation for name > 100 chars")
		assert.Contains(t, err.Error(), "cannot exceed 100 characters")

		// Case 4: Valid name at boundary (exactly 100 chars)
		validName := strings.Repeat("B", 100)
		cat, err := svc.Create(ctx, userID, validName)
		assert.NoError(t, err, "Name with exactly 100 chars should be valid")
		require.NotNil(t, cat)
		assert.Equal(t, validName, cat.Name)

		// Test update validation
		// Create a valid category first
		validCat, err := svc.Create(ctx, userID, "ValidCategory")
		require.NoError(t, err)

		// Try to update with empty name
		_, err = svc.Update(ctx, validCat.ID, userID, "")
		assert.Error(t, err, "Update with empty name should fail validation")
		assert.ErrorIs(t, err, domain.ErrValidation)

		// Try to update with name > 100 chars
		longUpdateName := strings.Repeat("C", 101)
		_, err = svc.Update(ctx, validCat.ID, userID, longUpdateName)
		assert.Error(t, err, "Update with name > 100 chars should fail validation")
		assert.ErrorIs(t, err, domain.ErrValidation)

		// Valid update should succeed
		validUpdateName := "UpdatedCategory"
		updated, err := svc.Update(ctx, validCat.ID, userID, validUpdateName)
		assert.NoError(t, err, "Update with valid name should succeed")
		require.NotNil(t, updated)
		assert.Equal(t, validUpdateName, updated.Name)
	})
}
