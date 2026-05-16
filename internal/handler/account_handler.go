package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ifrunruhin12/money-manager/internal/repository"
	"github.com/ifrunruhin12/money-manager/internal/service"
	"github.com/ifrunruhin12/money-manager/internal/utils"
)

// AccountHandler handles account and balance endpoints.
type AccountHandler struct {
	balanceService service.BalanceService
	accountRepo    repository.AccountRepository
}

// NewAccountHandler creates a new AccountHandler.
func NewAccountHandler(balanceService service.BalanceService, accountRepo repository.AccountRepository) *AccountHandler {
	return &AccountHandler{
		balanceService: balanceService,
		accountRepo:    accountRepo,
	}
}

// GetBalance handles GET /balance.
func (h *AccountHandler) GetBalance(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	balance, err := h.balanceService.GetBalance(c.Request.Context(), userID)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"balance": balance})
}

// UpdateStartingBalance handles PATCH /account/balance.
func (h *AccountHandler) UpdateStartingBalance(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	var req struct {
		Balance int `json:"balance" binding:"required"`
	}
	if !utils.BindJSON(c, &req) {
		return
	}

	if err := h.accountRepo.UpdateStartingBalance(c.Request.Context(), userID, req.Balance); err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	// Trigger reconcile after updating starting balance
	balance, err := h.balanceService.GetBalance(c.Request.Context(), userID)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"balance": balance})
}

// UpdateTimezone handles PATCH /account/timezone.
func (h *AccountHandler) UpdateTimezone(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	var req struct {
		Timezone string `json:"timezone" binding:"required"`
	}
	if !utils.BindJSON(c, &req) {
		return
	}

	// Validate IANA timezone string
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		utils.WriteError(c, http.StatusBadRequest, "invalid timezone: "+err.Error())
		return
	}

	if err := h.accountRepo.UpdateTimezone(c.Request.Context(), userID, req.Timezone); err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"timezone": req.Timezone})
}

// Reconcile handles POST /account/reconcile.
func (h *AccountHandler) Reconcile(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	// Get account to check current cached balance
	account, err := h.accountRepo.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	cachedBalance := account.CurrentBalance

	// Force reconciliation by calling GetBalance (which will recompute if needed)
	trueBalance, err := h.balanceService.GetBalance(c.Request.Context(), userID)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	mismatch := cachedBalance != trueBalance

	utils.WriteOK(c, http.StatusOK, gin.H{
		"reconciled": true,
		"mismatch":   mismatch,
		"balance":    trueBalance,
	})
}
