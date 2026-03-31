package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/usecase"
)

type EditorHandler struct {
	uc *usecase.EditorUsecase
}

func NewEditorHandler(uc *usecase.EditorUsecase) *EditorHandler {
	return &EditorHandler{uc: uc}
}

func (h *EditorHandler) Get(c *gin.Context) {
	user := CurrentUser(c)
	cfg, err := h.uc.Get(c.Request.Context(), user.OrganizationID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, cfg)
}

func (h *EditorHandler) Set(c *gin.Context) {
	var req struct {
		ProviderID          string `json:"provider_id"`
		ModelName           string `json:"model_name"`
		BasePrompt          string `json:"base_prompt"`
		MaxDurationSec      int    `json:"max_duration_sec" binding:"required,min=1"`
		IsAutonomousEnabled bool   `json:"is_autonomous_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	providerID := uuid.Nil
	if req.ProviderID != "" {
		parsed, err := uuid.Parse(req.ProviderID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider_id"})
			return
		}
		providerID = parsed
	}

	user := CurrentUser(c)
	cfg, err := h.uc.Set(c.Request.Context(), user.OrganizationID, usecase.EditorInput{
		ProviderID:          providerID,
		ModelName:           req.ModelName,
		BasePrompt:          req.BasePrompt,
		MaxDurationSec:      req.MaxDurationSec,
		IsAutonomousEnabled: req.IsAutonomousEnabled,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, cfg)
}

func (h *EditorHandler) StreamAgentTest(c *gin.Context) {
	user := CurrentUser(c)
	events, err := h.uc.StartAgentTest(c.Request.Context(), user.OrganizationID)
	if err != nil {
		httpError(c, err)
		return
	}

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
