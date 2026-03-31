package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/usecase"
)

type ProviderHandler struct {
	uc *usecase.ProviderUsecase
}

func NewProviderHandler(uc *usecase.ProviderUsecase) *ProviderHandler {
	return &ProviderHandler{uc: uc}
}

type providerTypeMeta struct {
	Type           domain.ProviderType `json:"type"`
	Label          string              `json:"label"`
	DefaultBaseURL string              `json:"default_base_url"`
	BaseURLFixed   bool                `json:"base_url_fixed"`
	APIKeyRequired bool                `json:"api_key_required"`
}

var knownProviderTypes = []providerTypeMeta{
	{domain.ProviderOpenAICompat, "OpenAI Compatible", "", false, false},
	{domain.ProviderDeepgram, "Deepgram", usecase.DeepgramBaseURL, true, true},
	{domain.ProviderOther, "Other", "", false, false},
}

func (h *ProviderHandler) Types(c *gin.Context) {
	c.JSON(http.StatusOK, knownProviderTypes)
}

func (h *ProviderHandler) List(c *gin.Context) {
	user := CurrentUser(c)
	items, err := h.uc.List(c.Request.Context(), user.OrganizationID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *ProviderHandler) Create(c *gin.Context) {
	var req struct {
		Name         string              `json:"name" binding:"required"`
		ProviderType domain.ProviderType `json:"provider_type" binding:"required,oneof=openai_compat deepgram other"`
		BaseURL      string              `json:"base_url"`
		APIKey       string              `json:"api_key"`
		IsActive     *bool               `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	user := CurrentUser(c)
	item, err := h.uc.Create(c.Request.Context(), user.OrganizationID, usecase.ProviderInput{
		Name:         req.Name,
		ProviderType: req.ProviderType,
		BaseURL:      req.BaseURL,
		APIKey:       req.APIKey,
		IsActive:     isActive,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *ProviderHandler) Update(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name         string              `json:"name" binding:"required"`
		ProviderType domain.ProviderType `json:"provider_type" binding:"required,oneof=openai_compat deepgram other"`
		BaseURL      string              `json:"base_url"`
		APIKey       string              `json:"api_key"`
		IsActive     *bool               `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	user := CurrentUser(c)
	item, err := h.uc.Update(c.Request.Context(), user.OrganizationID, id, usecase.ProviderInput{
		Name:         req.Name,
		ProviderType: req.ProviderType,
		BaseURL:      req.BaseURL,
		APIKey:       req.APIKey,
		IsActive:     isActive,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *ProviderHandler) Delete(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	user := CurrentUser(c)
	if err := h.uc.Delete(c.Request.Context(), user.OrganizationID, id); err != nil {
		httpError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ProviderHandler) TestGet(c *gin.Context) {
	var req struct {
		ProviderType domain.ProviderType `json:"provider_type" binding:"required,oneof=openai_compat deepgram other"`
		BaseURL      string              `json:"base_url" binding:"required"`
		APIKey       string              `json:"api_key"`
		TestPath     string              `json:"test_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	baseURL := strings.TrimSpace(req.BaseURL)
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}

	testPath := strings.TrimSpace(req.TestPath)
	if testPath == "" {
		testPath = "/models"
	}
	if !strings.HasPrefix(testPath, "/") {
		testPath = "/" + testPath
	}

	target := strings.TrimRight(baseURL, "/") + testPath
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid URL"})
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	httpReq.Header.Set("Accept", "application/json")

	start := time.Now()
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		curl := fmt.Sprintf("curl -i -X GET '%s' -H 'Accept: application/json'%s",
			target,
			func() string {
				if apiKey == "" {
					return ""
				}
				return " -H 'Authorization: Bearer ***'"
			}(),
		)
		c.JSON(http.StatusOK, gin.H{
			"ok":          false,
			"target_url":  target,
			"duration_ms": time.Since(start).Milliseconds(),
			"error":       err.Error(),
			"curl":        curl,
		})
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	body := strings.TrimSpace(string(bodyBytes))
	if len(body) > 1200 {
		body = body[:1200] + "...(truncated)"
	}

	curl := fmt.Sprintf("curl -i -X GET '%s' -H 'Accept: application/json'%s",
		target,
		func() string {
			if apiKey == "" {
				return ""
			}
			return " -H 'Authorization: Bearer ***'"
		}(),
	)

	c.JSON(http.StatusOK, gin.H{
		"ok":          resp.StatusCode >= 200 && resp.StatusCode < 300,
		"status_code": resp.StatusCode,
		"status":      resp.Status,
		"target_url":  target,
		"duration_ms": time.Since(start).Milliseconds(),
		"body":        body,
		"curl":        curl,
	})
}
