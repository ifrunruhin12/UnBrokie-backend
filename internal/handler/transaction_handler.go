package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ifrunruhin12/money-manager/internal/domain"
	"github.com/ifrunruhin12/money-manager/internal/service"
	"github.com/ifrunruhin12/money-manager/internal/utils"
)

type TransactionHandler struct {
	transactionService service.TransactionService
}

func NewTransactionHandler(transactionService service.TransactionService) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
	}
}

func (h *TransactionHandler) Create(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	var req struct {
		CategoryID string    `json:"category_id" binding:"required"`
		Amount     int       `json:"amount" binding:"required"`
		Date       time.Time `json:"date" binding:"required"`
		Note       string    `json:"note"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.WriteError(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	tx := domain.Transaction{
		UserID:     userID,
		CategoryID: req.CategoryID,
		Amount:     req.Amount,
		Date:       req.Date,
		Note:       req.Note,
		Type:       domain.TransactionTypeManual,
	}

	created, err := h.transactionService.Create(c.Request.Context(), tx)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusCreated, gin.H{"transaction": created})
}

func (h *TransactionHandler) List(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")
	limitStr := c.DefaultQuery("limit", "50")
	cursorDateStr := c.Query("cursor_date")
	cursorID := c.Query("cursor_id")

	if fromStr == "" {
		utils.WriteError(c, http.StatusBadRequest, "from query parameter is required")
		return
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		utils.WriteError(c, http.StatusBadRequest, "invalid from date format (expected RFC3339)")
		return
	}

	if toStr == "" {
		utils.WriteError(c, http.StatusBadRequest, "to query parameter is required")
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		utils.WriteError(c, http.StatusBadRequest, "invalid to date format (expected RFC3339)")
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		utils.WriteError(c, http.StatusBadRequest, "invalid limit (must be positive integer)")
		return
	}

	var cursorDate time.Time
	if cursorDateStr != "" {
		cursorDate, err = time.Parse(time.RFC3339, cursorDateStr)
		if err != nil {
			utils.WriteError(c, http.StatusBadRequest, "invalid cursor_date format (expected RFC3339)")
			return
		}
	}

	transactions, nextCursor, err := h.transactionService.ListByDateRange(
		c.Request.Context(),
		userID,
		from,
		to,
		limit,
		cursorDate,
		cursorID,
	)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	response := gin.H{
		"transactions": transactions,
	}
	if nextCursor != nil {
		response["next_cursor"] = gin.H{
			"date": nextCursor.Date.Format(time.RFC3339),
			"id":   nextCursor.ID,
		}
	}

	utils.WriteOK(c, http.StatusOK, response)
}

func (h *TransactionHandler) Override(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	txID := c.Param("id")
	if txID == "" {
		utils.WriteError(c, http.StatusBadRequest, "transaction id is required")
		return
	}

	var req struct {
		Amount int    `json:"amount" binding:"required"`
		Note   string `json:"note"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.WriteError(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	overrideTx, err := h.transactionService.Override(c.Request.Context(), txID, userID, req.Amount, req.Note)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"transaction": overrideTx})
}

func (h *TransactionHandler) Skip(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	txID := c.Param("id")
	if txID == "" {
		utils.WriteError(c, http.StatusBadRequest, "transaction id is required")
		return
	}

	if err := h.transactionService.Skip(c.Request.Context(), txID, userID); err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"skipped": true})
}

func (h *TransactionHandler) Restore(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	txID := c.Param("id")
	if txID == "" {
		utils.WriteError(c, http.StatusBadRequest, "transaction id is required")
		return
	}

	if err := h.transactionService.Restore(c.Request.Context(), txID, userID); err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"restored": true})
}

func (h *TransactionHandler) GetHistory(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	txID := c.Param("id")
	if txID == "" {
		utils.WriteError(c, http.StatusBadRequest, "transaction id is required")
		return
	}

	history, err := h.transactionService.GetHistory(c.Request.Context(), txID, userID)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"history": history})
}
