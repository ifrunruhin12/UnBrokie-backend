package repository

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifrunruhin12/money-manager/internal/db"
)

// SetupTestDB creates a connection pool to the test database.
// If the test database or user doesn't exist, it creates them automatically.
// It also runs all pending migrations.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgres://test_user:supersecretpassword@localhost:5432/money_manager_test?sslmode=disable"
	}

	// Try to connect to the test database
	pool, err := pgxpool.New(context.Background(), testDBURL)
	if err == nil {
		// Connection successful, verify with ping
		if err := pool.Ping(context.Background()); err == nil {
			// Run migrations to ensure schema is up to date
			if err := runTestMigrations(t, testDBURL); err != nil {
				t.Fatalf("failed to run migrations: %v", err)
			}
			return pool
		}
		pool.Close()
	}

	// Connection failed - try to create the test database and user
	t.Log("Test database not found, attempting to create it...")
	if err := ensureTestDatabaseExists(t); err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Retry connection after creating database
	pool, err = pgxpool.New(context.Background(), testDBURL)
	if err != nil {
		t.Fatalf("failed to connect to test database after creation: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("failed to ping test database: %v", err)
	}

	// Run migrations on the newly created database
	if err := runTestMigrations(t, testDBURL); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return pool
}

// runTestMigrations runs all pending migrations on the test database
func runTestMigrations(t *testing.T, dbURL string) error {
	t.Helper()

	// Find migrations directory relative to the test file
	// Assuming tests are in internal/repository or internal/service
	// and migrations are in the project root
	migrationsDir := findMigrationsDir(t)

	// Create a silent logger for migrations (to avoid cluttering test output)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors during tests
	}))

	return db.RunMigrations(dbURL, migrationsDir, logger)
}

// findMigrationsDir locates the migrations directory relative to the test
func findMigrationsDir(t *testing.T) string {
	t.Helper()

	// Try to find migrations directory by walking up from current directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Walk up the directory tree to find the migrations folder
	dir := cwd
	for {
		migrationsPath := filepath.Join(dir, "migrations")
		if _, err := os.Stat(migrationsPath); err == nil {
			return migrationsPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding migrations
			t.Fatalf("migrations directory not found (searched from %s)", cwd)
		}
		dir = parent
	}
}

// ensureTestDatabaseExists connects as the main database user and creates
// the test database and user if they don't exist.
func ensureTestDatabaseExists(t *testing.T) error {
	t.Helper()

	// Connect to the main database (from .env) as the main user to create test DB
	// The main user should have CREATEDB privilege
	mainDBURL := os.Getenv("DATABASE_URL")
	if mainDBURL == "" {
		// Fallback: try localhost with default credentials from .env
		mainDBURL = "postgres://money_manager:changeme@localhost:5432/money_manager?sslmode=disable"
	}

	// Replace 'db' hostname with 'localhost' for tests running outside Docker
	// Tests run on host machine, not inside Docker network
	if os.Getenv("DATABASE_URL") != "" {
		mainDBURL = "postgres://money_manager:changeme@localhost:5432/money_manager?sslmode=disable"
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, mainDBURL)
	if err != nil {
		return fmt.Errorf("failed to connect to main database: %w (make sure DATABASE_URL is set correctly or database is running)", err)
	}
	defer conn.Close(ctx)

	// Create test user if not exists
	_, err = conn.Exec(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT FROM pg_user WHERE usename = 'test_user') THEN
				CREATE USER test_user WITH PASSWORD 'supersecretpassword';
			END IF;
		END
		$$;
	`)
	if err != nil {
		return fmt.Errorf("failed to create test user: %w", err)
	}

	// Create test database if not exists
	var exists bool
	err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = 'money_manager_test')").Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if test database exists: %w", err)
	}
	
	if !exists {
		_, err = conn.Exec(ctx, "CREATE DATABASE money_manager_test OWNER test_user")
		if err != nil {
			return fmt.Errorf("failed to create test database: %w", err)
		}
	}

	// Grant privileges
	_, err = conn.Exec(ctx, "GRANT ALL PRIVILEGES ON DATABASE money_manager_test TO test_user")
	if err != nil {
		return fmt.Errorf("failed to grant privileges: %w", err)
	}

	t.Log("Test database and user created successfully")
	return nil
}

// CleanupTestDB closes the connection pool
func CleanupTestDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	pool.Close()
}

// TruncateTables cleans up all tables for a fresh test state
func TruncateTables(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()

	tables := []string{
		"events",
		"transactions",
		"big_buys",
		"consumables",
		"rules",
		"categories",
		"accounts",
		"users",
	}

	for _, table := range tables {
		_, err := pool.Exec(context.Background(), fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			t.Fatalf("failed to truncate table %s: %v", table, err)
		}
	}
}
