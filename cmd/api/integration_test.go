package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ifrunruhin12/money-manager/internal/api"
	"github.com/ifrunruhin12/money-manager/internal/db"
	"github.com/ifrunruhin12/money-manager/internal/handler"
	"github.com/ifrunruhin12/money-manager/internal/repository"
	"github.com/ifrunruhin12/money-manager/internal/service"
	"github.com/ifrunruhin12/money-manager/internal/config"
	"github.com/ifrunruhin12/money-manager/pkg/logger"
)

// repoRoot returns the absolute path to the repository root so migrations can
// be found regardless of which directory the test binary runs from.
func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	// file is cmd/api/integration_test.go — go up two levels
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	t.Skip("set TEST_DATABASE_URL or DATABASE_URL to run integration tests")
	return ""
}

// buildTestServer wires the full dependency graph and returns a *httptest.Server.
// The caller is responsible for closing it.
func buildTestServer(t *testing.T, dbURL, jwtSecret string) *httptest.Server {
	t.Helper()

	gin.SetMode(gin.TestMode)

	log := logger.New("error") // suppress noise in test output

	ctx := context.Background()

	cfg := &config.Config{
		DatabaseURL:         dbURL,
		JWTSecret:           jwtSecret,
		JWTExpiry:           24 * time.Hour,
		DBMaxConns:                20,
		DBMinConns:                2,
		DBMaxConnLifetime:         30 * time.Minute,
		DBMaxConnIdleTime:         5 * time.Minute,
		DBHealthCheckPeriod:       1 * time.Minute,
		DBMaxConnLifetimeJitter:   5 * time.Minute,
		DBPingTimeout:             5 * time.Second,
	}

	pool, err := db.Connect(ctx, cfg)
	require.NoError(t, err, "connect to test database")
	t.Cleanup(pool.Close)

	migrationsDir := filepath.Join(repoRoot(), "migrations")
	err = db.RunMigrations(dbURL, migrationsDir, log)
	require.NoError(t, err, "run migrations")

	userRepo := repository.NewUserRepository(pool)
	accountRepo := repository.NewAccountRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	transactionRepo := repository.NewTransactionRepository(pool)
	bigBuyRepo := repository.NewBigBuyRepository(pool)

	authSvc := service.NewAuthService(pool, userRepo, accountRepo, categoryRepo, cfg)
	balanceSvc := service.NewBalanceService(accountRepo, transactionRepo, bigBuyRepo, 5*time.Minute, log)
	transactionSvc := service.NewTransactionService(pool, transactionRepo, accountRepo, false, log)

	authH := handler.NewAuthHandler(authSvc)
	accountH := handler.NewAccountHandler(balanceSvc, accountRepo)
	transactionH := handler.NewTransactionHandler(transactionSvc)
	categoryH := handler.NewCategoryHandler()
	bigBuyH := handler.NewBigBuyHandler()

	router := api.NewRouter(jwtSecret, authH, accountH, transactionH, categoryH, bigBuyH)

	srv := httptest.NewServer(router)
	return srv
}

// postJSON sends a POST request with a JSON body and returns the response.
func postJSON(t *testing.T, srv *httptest.Server, path string, body any, token string) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, srv.URL+path, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// getJSON sends a GET request and returns the response.
func getJSON(t *testing.T, srv *httptest.Server, path string, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// decodeBody decodes a JSON response body into a map.
func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var m map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
	return m
}

// TestAuthSmokeTest is the Phase 0 checkpoint integration test.
//
// It verifies the full auth flow end-to-end:
//  1. POST /auth/register  → 201 + non-empty token
//  2. POST /auth/login     → 200 + valid JWT
//  3. GET  /balance        → 401 (no token)
//  4. GET  /balance        → 200 (with token)
func TestAuthSmokeTest(t *testing.T) {
	dbURL := testDatabaseURL(t)
	jwtSecret := "test-secret-for-smoke-test"

	srv := buildTestServer(t, dbURL, jwtSecret)
	defer srv.Close()

	email := fmt.Sprintf("smoketest+%d@example.com", time.Now().UnixNano())
	password := "securepassword123"

	// ── Step 1: Register ────────────────────────────────────────────────────
	resp := postJSON(t, srv, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": password,
	}, "")
	body := decodeBody(t, resp)

	require.Equal(t, http.StatusCreated, resp.StatusCode,
		"register should return 201; body: %v", body)

	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "response should have a 'data' object; body: %v", body)

	registerToken, ok := data["token"].(string)
	require.True(t, ok && registerToken != "", "register response should contain a non-empty token; data: %v", data)

	// ── Step 2: Login ────────────────────────────────────────────────────────
	resp = postJSON(t, srv, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, "")
	body = decodeBody(t, resp)

	require.Equal(t, http.StatusOK, resp.StatusCode,
		"login should return 200; body: %v", body)

	data, ok = body["data"].(map[string]any)
	require.True(t, ok, "login response should have a 'data' object; body: %v", body)

	loginToken, ok := data["token"].(string)
	require.True(t, ok && loginToken != "", "login response should contain a non-empty token; data: %v", data)

	// ── Step 3: GET /balance without token → 401 ────────────────────────────
	resp = getJSON(t, srv, "/api/v1/balance", "")
	resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"GET /balance without token should return 401")

	// ── Step 4: GET /balance with token → 200 ───────────────────────────────
	resp = getJSON(t, srv, "/api/v1/balance", loginToken)
	body = decodeBody(t, resp)

	require.Equal(t, http.StatusOK, resp.StatusCode,
		"GET /balance with valid token should return 200; body: %v", body)
}
