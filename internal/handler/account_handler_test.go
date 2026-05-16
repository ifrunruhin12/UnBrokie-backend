package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifrunruhin12/money-manager/internal/domain"
	"github.com/ifrunruhin12/money-manager/internal/handler"
	"github.com/ifrunruhin12/money-manager/internal/repository"
)

// Mock BalanceService
type mockBalanceService struct {
	balance int
	err     error
}

func (m *mockBalanceService) GetBalance(ctx context.Context, userID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.balance, nil
}

// Mock AccountRepository
type mockAccountRepository struct {
	account                  *domain.Account
	getErr                   error
	updateStartingBalanceErr error
	updateTimezoneErr        error
}

func (m *mockAccountRepository) Insert(ctx context.Context, db repository.DBTX, account domain.Account) error {
	return nil
}

func (m *mockAccountRepository) GetByUserID(ctx context.Context, userID string) (*domain.Account, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.account, nil
}

func (m *mockAccountRepository) UpdateStartingBalance(ctx context.Context, userID string, balance int) error {
	return m.updateStartingBalanceErr
}

func (m *mockAccountRepository) UpdateTimezone(ctx context.Context, userID string, tz string) error {
	return m.updateTimezoneErr
}

func (m *mockAccountRepository) AdjustBalance(ctx context.Context, db repository.DBTX, userID string, delta int) error {
	return nil
}

func (m *mockAccountRepository) SetDirty(ctx context.Context, db repository.DBTX, userID string, dirty bool) error {
	return nil
}

func (m *mockAccountRepository) SetReconciled(ctx context.Context, userID string) error {
	return nil
}

func (m *mockAccountRepository) ReconcileBalance(ctx context.Context, userID string, balance int) error {
	return nil
}

func setupTestRouter(balanceSvc *mockBalanceService, accountRepo *mockAccountRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Middleware to inject user_id into context
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user-id")
		c.Next()
	})

	h := handler.NewAccountHandler(balanceSvc, accountRepo)

	r.GET("/balance", h.GetBalance)
	r.PATCH("/account/balance", h.UpdateStartingBalance)
	r.PATCH("/account/timezone", h.UpdateTimezone)
	r.POST("/account/reconcile", h.Reconcile)

	return r
}

func TestGetBalance_Success(t *testing.T) {
	balanceSvc := &mockBalanceService{balance: 5000}
	accountRepo := &mockAccountRepository{}
	router := setupTestRouter(balanceSvc, accountRepo)

	req := httptest.NewRequest(http.MethodGet, "/balance", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["ok"].(bool))
	data := resp["data"].(map[string]any)
	assert.Equal(t, float64(5000), data["balance"].(float64))
}

func TestGetBalance_ServiceError(t *testing.T) {
	balanceSvc := &mockBalanceService{err: domain.ErrNotFound}
	accountRepo := &mockAccountRepository{}
	router := setupTestRouter(balanceSvc, accountRepo)

	req := httptest.NewRequest(http.MethodGet, "/balance", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.False(t, resp["ok"].(bool))
	assert.Contains(t, resp["error"].(string), "not found")
}

func TestUpdateStartingBalance_Success(t *testing.T) {
	balanceSvc := &mockBalanceService{balance: 10000}
	accountRepo := &mockAccountRepository{}
	router := setupTestRouter(balanceSvc, accountRepo)

	body := map[string]int{"balance": 10000}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/account/balance", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["ok"].(bool))
	data := resp["data"].(map[string]any)
	assert.Equal(t, float64(10000), data["balance"].(float64))
}

func TestUpdateStartingBalance_InvalidJSON(t *testing.T) {
	balanceSvc := &mockBalanceService{}
	accountRepo := &mockAccountRepository{}
	router := setupTestRouter(balanceSvc, accountRepo)

	req := httptest.NewRequest(http.MethodPatch, "/account/balance", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.False(t, resp["ok"].(bool))
}

func TestUpdateTimezone_Success(t *testing.T) {
	balanceSvc := &mockBalanceService{}
	accountRepo := &mockAccountRepository{}
	router := setupTestRouter(balanceSvc, accountRepo)

	body := map[string]string{"timezone": "Asia/Dhaka"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/account/timezone", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["ok"].(bool))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "Asia/Dhaka", data["timezone"].(string))
}

func TestUpdateTimezone_InvalidTimezone(t *testing.T) {
	balanceSvc := &mockBalanceService{}
	accountRepo := &mockAccountRepository{}
	router := setupTestRouter(balanceSvc, accountRepo)

	body := map[string]string{"timezone": "Invalid/Timezone"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/account/timezone", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.False(t, resp["ok"].(bool))
	assert.Contains(t, resp["error"].(string), "invalid timezone")
}

func TestReconcile_Success(t *testing.T) {
	now := time.Now()
	balanceSvc := &mockBalanceService{balance: 5000}
	accountRepo := &mockAccountRepository{
		account: &domain.Account{
			ID:               "acc-1",
			UserID:           "test-user-id",
			CurrentBalance:   4900, // Mismatch with true balance
			LastReconciledAt: &now,
		},
	}
	router := setupTestRouter(balanceSvc, accountRepo)

	req := httptest.NewRequest(http.MethodPost, "/account/reconcile", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["ok"].(bool))
	data := resp["data"].(map[string]any)
	assert.True(t, data["reconciled"].(bool))
	assert.True(t, data["mismatch"].(bool)) // Should detect mismatch
	assert.Equal(t, float64(5000), data["balance"].(float64))
}

func TestReconcile_NoMismatch(t *testing.T) {
	now := time.Now()
	balanceSvc := &mockBalanceService{balance: 5000}
	accountRepo := &mockAccountRepository{
		account: &domain.Account{
			ID:               "acc-1",
			UserID:           "test-user-id",
			CurrentBalance:   5000, // Matches true balance
			LastReconciledAt: &now,
		},
	}
	router := setupTestRouter(balanceSvc, accountRepo)

	req := httptest.NewRequest(http.MethodPost, "/account/reconcile", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["ok"].(bool))
	data := resp["data"].(map[string]any)
	assert.True(t, data["reconciled"].(bool))
	assert.False(t, data["mismatch"].(bool)) // No mismatch
	assert.Equal(t, float64(5000), data["balance"].(float64))
}
