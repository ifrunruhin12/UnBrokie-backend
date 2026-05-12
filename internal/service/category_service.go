package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifrunruhin12/money-manager/internal/domain"
	"github.com/ifrunruhin12/money-manager/internal/repository"
)

// CategoryService defines business logic operations for categories.
type CategoryService interface {
	Create(ctx context.Context, userID, name string) (*domain.Category, error)
	Update(ctx context.Context, id, userID, name string) (*domain.Category, error)
	Delete(ctx context.Context, id, userID string) error
	List(ctx context.Context, userID string) ([]domain.Category, error)
}

type categoryService struct {
	repo repository.CategoryRepository
	pool *pgxpool.Pool
}

// NewCategoryService creates a new CategoryService.
func NewCategoryService(repo repository.CategoryRepository, pool *pgxpool.Pool) CategoryService {
	return &categoryService{
		repo: repo,
		pool: pool,
	}
}

// Create validates and creates a new category.
func (s *categoryService) Create(ctx context.Context, userID, name string) (*domain.Category, error) {
	// Trim and validate name: non-empty and ≤100 chars
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: category name cannot be empty", domain.ErrValidation)
	}
	if len(name) > 100 {
		return nil, fmt.Errorf("%w: category name cannot exceed 100 characters", domain.ErrValidation)
	}

	cat := domain.Category{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}

	// Insert via repo using pool; repo maps unique violation to ErrConflict
	if err := s.repo.Insert(ctx, s.pool, cat); err != nil {
		return nil, err
	}

	return &cat, nil
}

// Update validates and updates an existing category.
func (s *categoryService) Update(ctx context.Context, id, userID, name string) (*domain.Category, error) {
	// Trim and validate name: non-empty and ≤100 chars
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: category name cannot be empty", domain.ErrValidation)
	}
	if len(name) > 100 {
		return nil, fmt.Errorf("%w: category name cannot exceed 100 characters", domain.ErrValidation)
	}

	cat := domain.Category{
		ID:     id,
		UserID: userID,
		Name:   name,
	}

	// Update via repo; repo maps unique violation to ErrConflict and no rows to ErrNotFound
	if err := s.repo.Update(ctx, cat); err != nil {
		return nil, err
	}

	return &cat, nil
}

// Delete checks references and deletes a category if not referenced.
// Verifies ownership by checking userID on all operations.
func (s *categoryService) Delete(ctx context.Context, id, userID string) error {
	// Check if referenced by transactions (scoped to this user)
	refByTx, err := s.repo.IsReferencedByTransactions(ctx, id, userID)
	if err != nil {
		return err
	}
	if refByTx {
		return fmt.Errorf("%w: category is referenced by one or more transactions and cannot be deleted", domain.ErrConflict)
	}

	// Check if referenced by big buys (scoped to this user)
	refByBB, err := s.repo.IsReferencedByBigBuys(ctx, id, userID)
	if err != nil {
		return err
	}
	if refByBB {
		return fmt.Errorf("%w: category is referenced by one or more big buys and cannot be deleted", domain.ErrConflict)
	}

	// Hard delete via repo (includes userID check to prevent cross-user deletion)
	return s.repo.Delete(ctx, id, userID)
}

// List retrieves all categories for a user.
func (s *categoryService) List(ctx context.Context, userID string) ([]domain.Category, error) {
	return s.repo.ListByUser(ctx, userID)
}

// SeedDefaultCategories creates the default categories for a new user.
// This is called during user registration inside the same transaction.
func SeedDefaultCategories(ctx context.Context, repo repository.CategoryRepository, db repository.DBTX, userID string) error {
	defaultCategories := []string{"Food", "Transport", "Extra Food", "Health", "Big Buy", "Savings", "Hobby"}
	now := time.Now().UTC()

	for _, name := range defaultCategories {
		cat := domain.Category{
			ID:        uuid.New().String(),
			UserID:    userID,
			Name:      name,
			CreatedAt: now,
		}
		if err := repo.Insert(ctx, db, cat); err != nil {
			return fmt.Errorf("failed to seed category %s: %w", name, err)
		}
	}

	return nil
}
