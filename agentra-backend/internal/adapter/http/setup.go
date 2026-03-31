package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/runtime"
	"github.com/reaganiwadha/agentra/internal/usecase"
)

type SetupHandler struct {
	setupUC   *usecase.SetupUsecase
	storageUC *usecase.StorageUsecase
	db        *sqlx.DB
}

func NewSetupHandler(setupUC *usecase.SetupUsecase, storageUC *usecase.StorageUsecase, db *sqlx.DB) *SetupHandler {
	return &SetupHandler{setupUC: setupUC, storageUC: storageUC, db: db}
}

func (h *SetupHandler) Status(c *gin.Context) {
	needs, err := h.setupUC.NeedsSetup(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"needs_setup": needs})
}

func (h *SetupHandler) Options(c *gin.Context) {
	needs, err := h.setupUC.NeedsSetup(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"needs_setup":         needs,
		"available_storages":  h.setupUC.AvailableStorageOptions(),
		"recommended_storage": domain.StorageMinIO,
	})
}

func (h *SetupHandler) Validate(c *gin.Context) {
	var req struct {
		Token            string `json:"token" binding:"required"`
		OrganizationName string `json:"organization_name" binding:"required"`
		Email            string `json:"email" binding:"required,email"`
		Password         string `json:"password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.setupUC.Validate(c.Request.Context(), req.Token, req.OrganizationName, req.Email, req.Password); err != nil {
		httpError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"valid": true})
}

func (h *SetupHandler) ValidateStorage(c *gin.Context) {
	var req struct {
		Token            string             `json:"token" binding:"required"`
		StorageType      domain.StorageType `json:"storage_type" binding:"required,eq=minio"`
		Endpoint         string             `json:"endpoint" binding:"required"`
		AccessKey        string             `json:"access_key" binding:"required"`
		SecretKey        string             `json:"secret_key" binding:"required"`
		Bucket           string             `json:"bucket" binding:"required"`
		BasePath         string             `json:"base_path"`
		OutputBasePath   string             `json:"output_base_path"`
		OrganizationName string             `json:"organization_name"`
		Email            string             `json:"email"`
		Password         string             `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Optional identity fields are validated when present to catch stale setup state early.
	if req.OrganizationName != "" || req.Email != "" || req.Password != "" {
		if err := h.setupUC.Validate(c.Request.Context(), req.Token, req.OrganizationName, req.Email, req.Password); err != nil {
			httpError(c, err)
			return
		}
	}

	probe, err := h.storageUC.Validate(c.Request.Context(), usecase.StorageInput{
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
	c.JSON(http.StatusOK, probe)
}

func (h *SetupHandler) Handle(c *gin.Context) {
	var req struct {
		Token            string             `json:"token" binding:"required"`
		OrganizationName string             `json:"organization_name" binding:"required"`
		Email            string             `json:"email" binding:"required,email"`
		Password         string             `json:"password" binding:"required,min=8"`
		StorageType      domain.StorageType `json:"storage_type" binding:"omitempty,eq=minio"`
		Endpoint         string             `json:"endpoint"`
		AccessKey        string             `json:"access_key"`
		SecretKey        string             `json:"secret_key"`
		Bucket           string             `json:"bucket"`
		BasePath         string             `json:"base_path"`
		OutputBasePath   string             `json:"output_base_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	storageIn := usecase.DefaultSetupStorage()
	if req.StorageType != "" {
		storageIn.StorageType = req.StorageType
	}
	if req.Endpoint != "" {
		storageIn.Endpoint = req.Endpoint
	}
	if req.AccessKey != "" {
		storageIn.AccessKey = req.AccessKey
	}
	if req.SecretKey != "" {
		storageIn.SecretKey = req.SecretKey
	}
	if req.Bucket != "" {
		storageIn.Bucket = req.Bucket
	}
	if req.BasePath != "" {
		storageIn.BasePath = req.BasePath
	}
	if req.OutputBasePath != "" {
		storageIn.OutputBasePath = req.OutputBasePath
	}

	if err := h.setupUC.Run(
		c.Request.Context(),
		req.Token,
		req.OrganizationName,
		req.Email,
		req.Password,
		storageIn,
	); err != nil {
		httpError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "setup complete"})
}

func (h *SetupHandler) Reset(c *gin.Context) {
	if err := h.truncateAllTables(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("reset failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "reset complete, restarting backend"})

	go func() {
		time.Sleep(300 * time.Millisecond)
		runtime.RequestRestart()
	}()
}

func (h *SetupHandler) truncateAllTables(ctx context.Context) error {
	var tables []string
	if err := h.db.SelectContext(ctx, &tables, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		  AND tablename <> 'goose_db_version'
		ORDER BY tablename
	`); err != nil {
		return err
	}

	if len(tables) == 0 {
		return nil
	}

	quoted := make([]string, 0, len(tables))
	for _, table := range tables {
		quoted = append(quoted, pq.QuoteIdentifier(table))
	}

	stmt := "TRUNCATE TABLE " + strings.Join(quoted, ", ") + " RESTART IDENTITY CASCADE"
	_, err := h.db.ExecContext(ctx, stmt)
	return err
}
