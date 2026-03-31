package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/usecase"
)

type ProjectHandler struct {
	uc *usecase.ProjectUsecase
}

func NewProjectHandler(uc *usecase.ProjectUsecase) *ProjectHandler {
	return &ProjectHandler{uc: uc}
}

func (h *ProjectHandler) List(c *gin.Context) {
	user := CurrentUser(c)
	projects, err := h.uc.List(c.Request.Context(), user.OrganizationID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, projects)
}

func (h *ProjectHandler) Get(c *gin.Context) {
	projectID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	user := CurrentUser(c)
	project, err := h.uc.Get(c.Request.Context(), user.OrganizationID, projectID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req struct {
		Name           string `json:"name" binding:"required"`
		Description    string `json:"description"`
		StorageSubpath string `json:"storage_subpath"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := CurrentUser(c)
	project, err := h.uc.Create(c.Request.Context(), user.OrganizationID, user.ID, usecase.ProjectInput{
		Name:           req.Name,
		Description:    req.Description,
		StorageSubpath: req.StorageSubpath,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusCreated, project)
}

func (h *ProjectHandler) SetMediaScope(c *gin.Context) {
	projectID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	var req struct {
		Mode     domain.MediaScopeMode `json:"mode" binding:"required,oneof=global date_range selected"`
		StartAt  string                `json:"start_at"`
		EndAt    string                `json:"end_at"`
		MediaIDs []string              `json:"media_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var startAt *time.Time
	if req.StartAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.StartAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_at"})
			return
		}
		startAt = &parsed
	}

	var endAt *time.Time
	if req.EndAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.EndAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_at"})
			return
		}
		endAt = &parsed
	}

	mediaIDs := make([]uuid.UUID, 0, len(req.MediaIDs))
	for _, raw := range req.MediaIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid media id in media_ids"})
			return
		}
		mediaIDs = append(mediaIDs, id)
	}

	user := CurrentUser(c)
	project, err := h.uc.SetMediaScope(c.Request.Context(), user.OrganizationID, projectID, usecase.ProjectMediaScopeInput{
		Mode:     req.Mode,
		StartAt:  startAt,
		EndAt:    endAt,
		MediaIDs: mediaIDs,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (h *ProjectHandler) UpdateDraft(c *gin.Context) {
	projectID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	var req struct {
		Name           string `json:"name" binding:"required"`
		BasePrompt     string `json:"base_prompt"`
		VariantCount   int    `json:"variant_count" binding:"required,min=1,max=12"`
		MinDurationSec int    `json:"min_duration_sec" binding:"required,min=1"`
		MaxDurationSec int    `json:"max_duration_sec" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := CurrentUser(c)
	project, err := h.uc.UpdateDraft(c.Request.Context(), user.OrganizationID, projectID, usecase.ProjectDraftInput{
		Name:           req.Name,
		BasePrompt:     req.BasePrompt,
		VariantCount:   req.VariantCount,
		MinDurationSec: req.MinDurationSec,
		MaxDurationSec: req.MaxDurationSec,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (h *ProjectHandler) QueueRuns(c *gin.Context) {
	projectID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	var req struct {
		Name           string `json:"name" binding:"required"`
		BasePrompt     string `json:"base_prompt"`
		VariantCount   int    `json:"variant_count" binding:"required,min=1,max=12"`
		MinDurationSec int    `json:"min_duration_sec" binding:"required,min=1"`
		MaxDurationSec int    `json:"max_duration_sec" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := CurrentUser(c)
	jobs, err := h.uc.QueueRuns(c.Request.Context(), user.OrganizationID, projectID, user.ID, usecase.QueueProjectRunsInput{
		Name:           req.Name,
		BasePrompt:     req.BasePrompt,
		VariantCount:   req.VariantCount,
		MinDurationSec: req.MinDurationSec,
		MaxDurationSec: req.MaxDurationSec,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"runs": jobs})
}

func (h *ProjectHandler) ListRuns(c *gin.Context) {
	projectID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	user := CurrentUser(c)
	jobs, err := h.uc.ListRuns(c.Request.Context(), user.OrganizationID, projectID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"runs": jobs})
}

func (h *ProjectHandler) CancelRun(c *gin.Context) {
	runID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	user := CurrentUser(c)
	job, err := h.uc.CancelRun(c.Request.Context(), user.OrganizationID, runID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *ProjectHandler) GetRun(c *gin.Context) {
	runID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	user := CurrentUser(c)
	run, err := h.uc.GetRun(c.Request.Context(), user.OrganizationID, runID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, run)
}

func (h *ProjectHandler) StreamRender(c *gin.Context) {
	renderID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	user := CurrentUser(c)
	rc, size, mimeType, filename, err := h.uc.OpenRenderStream(c.Request.Context(), user.OrganizationID, renderID)
	if err != nil {
		httpError(c, err)
		return
	}
	defer rc.Close()

	safeName := strings.ReplaceAll(filename, `"`, `'`)
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, safeName))
	c.Header("Cache-Control", "private, max-age=60")
	if mimeType != "" {
		c.Header("Content-Type", mimeType)
	}

	if rs, ok := rc.(io.ReadSeeker); ok {
		http.ServeContent(c.Writer, c.Request, filename, time.Time{}, rs)
	} else {
		c.DataFromReader(http.StatusOK, size, mimeType, rc, nil)
	}
}
