package handler

import (
	"github.com/reaganiwadha/agentra/internal/usecase"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	uc *usecase.AuthUsecase
}

func NewAuthHandler(uc *usecase.AuthUsecase) *AuthHandler {
	return &AuthHandler{uc: uc}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.uc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		httpError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": session.ID})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	session := CurrentSession(c)
	if err := h.uc.Logout(c.Request.Context(), session.ID); err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}
