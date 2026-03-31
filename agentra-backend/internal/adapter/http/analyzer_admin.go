package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/usecase"
)

type AnalyzerAdminHandler struct {
	uc *usecase.AnalyzerAdminUsecase
}

func NewAnalyzerAdminHandler(uc *usecase.AnalyzerAdminUsecase) *AnalyzerAdminHandler {
	return &AnalyzerAdminHandler{uc: uc}
}

func (h *AnalyzerAdminHandler) List(c *gin.Context) {
	user := CurrentUser(c)
	items, err := h.uc.List(c.Request.Context(), user.OrganizationID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *AnalyzerAdminHandler) Create(c *gin.Context) {
	var req struct {
		Name         string              `json:"name"`
		AnalyzerType domain.AnalyzerType `json:"analyzer_type" binding:"required,oneof=transcription vision_tags embedding"`
		ProviderID   string              `json:"provider_id" binding:"required"`
		ModelName    string              `json:"model_name"`
		ConfigJSON   string              `json:"config_json"`
		IsEnabled    *bool               `json:"is_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	providerID, err := uuid.Parse(req.ProviderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider_id"})
		return
	}
	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}

	user := CurrentUser(c)
	item, err := h.uc.Create(c.Request.Context(), user.OrganizationID, usecase.AnalyzerInput{
		Name:         req.Name,
		AnalyzerType: req.AnalyzerType,
		ProviderID:   providerID,
		ModelName:    req.ModelName,
		ConfigJSON:   req.ConfigJSON,
		IsEnabled:    enabled,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *AnalyzerAdminHandler) Update(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name         string              `json:"name"`
		AnalyzerType domain.AnalyzerType `json:"analyzer_type" binding:"required,oneof=transcription vision_tags embedding"`
		ProviderID   string              `json:"provider_id" binding:"required"`
		ModelName    string              `json:"model_name"`
		ConfigJSON   string              `json:"config_json"`
		IsEnabled    *bool               `json:"is_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	providerID, err := uuid.Parse(req.ProviderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider_id"})
		return
	}
	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}

	user := CurrentUser(c)
	item, err := h.uc.Update(c.Request.Context(), user.OrganizationID, id, usecase.AnalyzerInput{
		Name:         req.Name,
		AnalyzerType: req.AnalyzerType,
		ProviderID:   providerID,
		ModelName:    req.ModelName,
		ConfigJSON:   req.ConfigJSON,
		IsEnabled:    enabled,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *AnalyzerAdminHandler) Delete(c *gin.Context) {
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

func (h *AnalyzerAdminHandler) StreamTest(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	user := CurrentUser(c)
	events, err := h.uc.StartAnalyzerTest(c.Request.Context(), user.OrganizationID, id)
	if err != nil {
		httpError(c, err)
		return
	}
	h.streamAnalyzerTestEvents(c, events)
}

func (h *AnalyzerAdminHandler) streamAnalyzerTestEvents(c *gin.Context, events <-chan usecase.TranscriptionTestEvent) {
	flusher, ok := startSSE(c)
	if !ok {
		return
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case ev, open := <-events:
			if !open {
				return
			}
			c.SSEvent(ev.Event, ev)
			flusher.Flush()
		case <-heartbeat.C:
			c.SSEvent("heartbeat", gin.H{"ts": time.Now().UTC().Format(time.RFC3339Nano)})
			flusher.Flush()
		}
	}
}
