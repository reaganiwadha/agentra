package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/reaganiwadha/agentra/internal/usecase"
)

type ActivityHandler struct {
	uc *usecase.ActivityUsecase
}

func NewActivityHandler(uc *usecase.ActivityUsecase) *ActivityHandler {
	return &ActivityHandler{uc: uc}
}

func (h *ActivityHandler) Status(c *gin.Context) {
	user := CurrentUser(c)
	active, latest, err := h.uc.Status(c.Request.Context(), user.OrganizationID)
	if err != nil {
		httpError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"active": active,
		"latest": latest,
	})
}

func (h *ActivityHandler) List(c *gin.Context) {
	user := CurrentUser(c)
	limit := 40
	if raw := c.Query("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	logs, err := h.uc.ListRecent(c.Request.Context(), user.OrganizationID, limit)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, logs)
}

func (h *ActivityHandler) Stream(c *gin.Context) {
	user := CurrentUser(c)
	snapshot, err := h.uc.Snapshot(c.Request.Context(), user.OrganizationID, 30)
	if err != nil {
		httpError(c, err)
		return
	}

	flusher, ok := startSSE(c)
	if !ok {
		return
	}

	c.SSEvent("snapshot", snapshot)
	flusher.Flush()

	logCh, unsubscribe := h.uc.Subscribe(user.OrganizationID)
	defer unsubscribe()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case log, open := <-logCh:
			if !open {
				return
			}
			active, _, err := h.uc.Status(c.Request.Context(), user.OrganizationID)
			if err != nil {
				continue
			}
			c.SSEvent("log", gin.H{
				"active": active,
				"log":    log,
			})
			flusher.Flush()
		case <-heartbeat.C:
			c.SSEvent("heartbeat", gin.H{"ts": time.Now().UTC().Format(time.RFC3339Nano)})
			flusher.Flush()
		}
	}
}
