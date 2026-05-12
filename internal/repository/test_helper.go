package repository

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupTestDB creates a connection pool to the test database
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://test_user:supersecretpassword@localhost:5432/money_manager_test?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	// Ping to verify connection
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("failed to ping test database: %v", err)
	}

	return pool
}

// cleanupTestDB closes the connection pool
func cleanupTestDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	pool.Close()
}

// truncateTables cleans up all tables for a fresh test state
func truncateTables(t *testing.T, pool *pgxpool.Pool) {
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
