package handler

import (
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/usecase"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	uc *usecase.UserUsecase
}

func NewUserHandler(uc *usecase.UserUsecase) *UserHandler {
	return &UserHandler{uc: uc}
}

func (h *UserHandler) Me(c *gin.Context) {
	user := CurrentUser(c)
	c.JSON(http.StatusOK, gin.H{
		"id":              user.ID,
		"organization_id": user.OrganizationID,
		"email":           user.Email,
		"role":            user.Role,
		"is_active":       user.IsActive,
	})
}

func (h *UserHandler) Create(c *gin.Context) {
	var req struct {
		Email    string          `json:"email" binding:"required,email"`
		Password string          `json:"password" binding:"required,min=8"`
		Role     domain.UserRole `json:"role" binding:"required,oneof=admin editor viewer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	requester := CurrentUser(c)
	user, err := h.uc.Create(c.Request.Context(), requester.OrganizationID, req.Email, req.Password, req.Role)
	if err != nil {
		httpError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       user.ID,
		"email":    user.Email,
		"role":     user.Role,
		"is_active": user.IsActive,
	})
}
