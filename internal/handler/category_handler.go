package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ifrunruhin12/money-manager/internal/service"
	"github.com/ifrunruhin12/money-manager/internal/utils"
)

type CategoryHandler struct {
	service service.CategoryService
}

func NewCategoryHandler(service service.CategoryService) *CategoryHandler {
	return &CategoryHandler{
		service: service,
	}
}

func (h *CategoryHandler) Create(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if !utils.BindJSON(c, &req) {
		return
	}

	cat, err := h.service.Create(c.Request.Context(), userID, req.Name)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusCreated, cat)
}

func (h *CategoryHandler) List(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	categories, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"categories": categories})
}

func (h *CategoryHandler) Update(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	id := c.Param("id")
	if id == "" {
		utils.WriteError(c, http.StatusBadRequest, "category id is required")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if !utils.BindJSON(c, &req) {
		return
	}

	cat, err := h.service.Update(c.Request.Context(), id, userID, req.Name)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, cat)
}

func (h *CategoryHandler) Delete(c *gin.Context) {
	userID, ok := utils.GetUserID(c)
	if !ok {
		return
	}

	id := c.Param("id")
	if id == "" {
		utils.WriteError(c, http.StatusBadRequest, "category id is required")
		return
	}

	err := h.service.Delete(c.Request.Context(), id, userID)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"deleted": true})
}
