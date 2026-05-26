package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ifrunruhin12/money-manager/internal/service"
	"github.com/ifrunruhin12/money-manager/internal/utils"
)

type AuthHandler struct {
	auth service.AuthService
}

func NewAuthHandler(auth service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type authRequest struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req authRequest
	if !utils.BindJSON(c, &req) {
		return
	}

	token, err := h.auth.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusCreated, gin.H{"token": token})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req authRequest
	if !utils.BindJSON(c, &req) {
		return
	}

	token, err := h.auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		status, msg := utils.MapError(err)
		utils.WriteError(c, status, msg)
		return
	}

	utils.WriteOK(c, http.StatusOK, gin.H{"token": token})
}
