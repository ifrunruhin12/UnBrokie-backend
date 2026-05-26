package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ifrunruhin12/money-manager/internal/domain"
	"github.com/ifrunruhin12/money-manager/internal/service"
	"github.com/ifrunruhin12/money-manager/internal/utils"
)

type BigBuyHandler struct {
	bigBuyService service.BigBuyService
}

func NewBigBuyHandler(bigBuyService service.BigBuyService) *BigBuyHandler {
	return &BigBuyHandler{
		bigBuyService: bigBuyService,
	}
}

func (h *BigBuyHandler) Create(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	var req struct {
		Title      string    `json:"title" binding:"required"`
		Amount     int       `json:"amount" binding:"required"`
		CategoryID string    `json:"category_id" binding:"required"`
		Date       time.Time `json:"date" binding:"required"`
		Note       string    `json:"note"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.WriteError(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	bigBuy := domain.BigBuy{
		UserID:     userID,
		Title:      req.Title,
		Amount:     req.Amount,
		CategoryID: req.CategoryID,
		Date:       req.Date,
		Note:       req.Note,
	}

	created, err := h.bigBuyService.Create(c.Request.Context(), bigBuy)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusCreated, gin.H{"big_buy": created})
}

func (h *BigBuyHandler) List(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	monthStr := c.Query("month")
	if monthStr == "" {
		utils.WriteError(c, http.StatusBadRequest, "month query parameter is required (format: YYYY-MM)")
		return
	}

	monthTime, err := time.Parse("2006-01", monthStr)
	if err != nil {
		utils.WriteError(c, http.StatusBadRequest, "invalid month format (expected YYYY-MM)")
		return
	}

	year := monthTime.Year()
	month := int(monthTime.Month())

	// Call service
	bigBuys, err := h.bigBuyService.ListByMonth(c.Request.Context(), userID, year, month)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"big_buys": bigBuys})
}

func (h *BigBuyHandler) Update(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	bigBuyID := c.Param("id")
	if bigBuyID == "" {
		utils.WriteError(c, http.StatusBadRequest, "big buy id is required")
		return
	}

	var req struct {
		Title      string    `json:"title" binding:"required"`
		Amount     int       `json:"amount" binding:"required"`
		CategoryID string    `json:"category_id" binding:"required"`
		Date       time.Time `json:"date" binding:"required"`
		Note       string    `json:"note"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.WriteError(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	bigBuy := domain.BigBuy{
		ID:         bigBuyID,
		UserID:     userID,
		Title:      req.Title,
		Amount:     req.Amount,
		CategoryID: req.CategoryID,
		Date:       req.Date,
		Note:       req.Note,
	}

	updated, err := h.bigBuyService.Update(c.Request.Context(), bigBuy)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"big_buy": updated})
}

func (h *BigBuyHandler) Delete(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	bigBuyID := c.Param("id")
	if bigBuyID == "" {
		utils.WriteError(c, http.StatusBadRequest, "big buy id is required")
		return
	}

	if err := h.bigBuyService.Delete(c.Request.Context(), bigBuyID, userID); err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"deleted": true})
}
