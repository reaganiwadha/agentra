package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/usecase"
)

type StorageHandler struct {
	uc *usecase.StorageUsecase
}

func NewStorageHandler(uc *usecase.StorageUsecase) *StorageHandler {
	return &StorageHandler{uc: uc}
}

func (h *StorageHandler) Get(c *gin.Context) {
	user := CurrentUser(c)
	cfg, err := h.uc.Get(c.Request.Context(), user.OrganizationID)
	if err != nil {
		httpError(c, err)
		return
	}
	cfg.HasSecret = cfg.SecretKey.String != ""
	c.JSON(http.StatusOK, cfg)
}

func (h *StorageHandler) Status(c *gin.Context) {
	user := CurrentUser(c)
	status, err := h.uc.Status(c.Request.Context(), user.OrganizationID)
	if err != nil {
		httpError(c, err)
		return
	}
	if status.Config != nil {
		status.Config.HasSecret = status.Config.SecretKey.String != ""
	}
	c.JSON(http.StatusOK, status)
}

func (h *StorageHandler) Set(c *gin.Context) {
	var req struct {
		StorageType    domain.StorageType `json:"storage_type"     binding:"required,oneof=smb minio"`
		Endpoint       string             `json:"endpoint"`
		AccessKey      string             `json:"access_key"`
		SecretKey      string             `json:"secret_key"`
		Bucket         string             `json:"bucket"`
		BasePath       string             `json:"base_path"`
		OutputBasePath string             `json:"output_base_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := CurrentUser(c)

	// Preserve existing secret key if not provided
	if req.SecretKey == "" {
		existing, err := h.uc.Get(c.Request.Context(), user.OrganizationID)
		if err == nil {
			req.SecretKey = existing.SecretKey.String
		}
	}

	cfg, err := h.uc.Set(c.Request.Context(), user.OrganizationID, usecase.StorageInput{
		StorageType:    req.StorageType,
		Endpoint:       req.Endpoint,
		AccessKey:      req.AccessKey,
		SecretKey:      req.SecretKey,
		Bucket:         req.Bucket,
		BasePath:       req.BasePath,
		OutputBasePath: req.OutputBasePath,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	cfg.HasSecret = cfg.SecretKey.String != ""
	c.JSON(http.StatusOK, cfg)
}
