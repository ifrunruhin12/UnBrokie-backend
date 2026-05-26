package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/ifrunruhin12/money-manager/internal/config"
	"github.com/ifrunruhin12/money-manager/internal/domain"
	"github.com/ifrunruhin12/money-manager/internal/repository"
	"github.com/ifrunruhin12/money-manager/internal/utils"
)

var defaultCategories = []string{
	"Food", "Transport", "Extra Food", "Health", "Big Buy", "Savings", "Hobby",
}

var emailRegex = regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`)

type AuthService interface {
	Register(ctx context.Context, email, password string) (string, error)
	Login(ctx context.Context, email, password string) (string, error)
}

type authService struct {
	db         *pgxpool.Pool
	users      repository.UserRepository
	accounts   repository.AccountRepository
	categories repository.CategoryRepository
	cfg        *config.Config
}

func NewAuthService(
	db *pgxpool.Pool,
	users repository.UserRepository,
	accounts repository.AccountRepository,
	categories repository.CategoryRepository,
	cfg *config.Config,
) AuthService {
	return &authService{
		db:         db,
		users:      users,
		accounts:   accounts,
		categories: categories,
		cfg:        cfg,
	}
}

func (s *authService) Register(ctx context.Context, email, password string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	email = strings.TrimSpace(strings.ToLower(email))

	if !emailRegex.MatchString(email) {
		return "", fmt.Errorf("%w: invalid email format", domain.ErrValidation)
	}
	if len(password) < 8 {
		return "", fmt.Errorf("%w: password must be at least 8 characters", domain.ErrValidation)
	}

	// Best-effort pre-check; DB unique constraint is the real guard against races.
	_, err := s.users.GetByEmail(ctx, email)
	if err == nil {
		return "", domain.ErrConflict
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return "", fmt.Errorf("check existing email: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	userID := uuid.New().String()
	now := time.Now().UTC()

	// Single DB transaction: user → account → default categories.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	user := domain.User{
		ID:           userID,
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err = s.users.Insert(ctx, tx, user); err != nil {
		// Handle TOCTOU race: concurrent registration with same email.
		if errors.Is(err, domain.ErrConflict) {
			return "", domain.ErrConflict
		}
		return "", fmt.Errorf("insert user: %w", err)
	}

	account := domain.Account{
		ID:              uuid.New().String(),
		UserID:          userID,
		StartingBalance: 0,
		CurrentBalance:  0,
		BalanceDirty:    false,
		Currency:        "BDT",
		Timezone:        "UTC",
		CreatedAt:       now,
	}
	if err = s.accounts.Insert(ctx, tx, account); err != nil {
		return "", fmt.Errorf("insert account: %w", err)
	}

	for _, name := range defaultCategories {
		cat := domain.Category{
			ID:        uuid.New().String(),
			UserID:    userID,
			Name:      name,
			CreatedAt: now,
		}
		if err = s.categories.Insert(ctx, tx, cat); err != nil {
			return "", fmt.Errorf("insert category %q: %w", name, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit transaction: %w", err)
	}

	token, err := utils.GenerateToken(userID, s.cfg.JWTSecret, s.cfg.JWTExpiry)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return token, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	email = strings.TrimSpace(strings.ToLower(email))

	if !emailRegex.MatchString(email) {
		return "", fmt.Errorf("%w: invalid email format", domain.ErrValidation)
	}
	if password == "" {
		return "", fmt.Errorf("%w: password is required", domain.ErrValidation)
	}

	// Map ErrNotFound → ErrUnauthorized to prevent user enumeration.
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", fmt.Errorf("%w: invalid credentials", domain.ErrUnauthorized)
		}
		return "", fmt.Errorf("lookup user: %w", err)
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", fmt.Errorf("%w: invalid credentials", domain.ErrUnauthorized)
	}

	token, err := utils.GenerateToken(user.ID, s.cfg.JWTSecret, s.cfg.JWTExpiry)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return token, nil
}
