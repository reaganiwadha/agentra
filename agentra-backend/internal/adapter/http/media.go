package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/reaganiwadha/agentra/internal/usecase"
)

type MediaHandler struct {
	uc *usecase.MediaUsecase
}

func NewMediaHandler(uc *usecase.MediaUsecase) *MediaHandler {
	return &MediaHandler{uc: uc}
}

func (h *MediaHandler) ListByProject(c *gin.Context) {
	projectID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	user := CurrentUser(c)
	assets, err := h.uc.ListByProject(c.Request.Context(), user.OrganizationID, projectID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, assets)
}

func (h *MediaHandler) Get(c *gin.Context) {
	mediaID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	user := CurrentUser(c)
	detail, err := h.uc.GetDetail(c.Request.Context(), user.OrganizationID, mediaID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *MediaHandler) ListByOrg(c *gin.Context) {
	user := CurrentUser(c)
	assets, err := h.uc.ListByOrg(c.Request.Context(), user.OrganizationID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, assets)
}

func (h *MediaHandler) Upload(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload"})
		return
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "agentra-upload-*")
	if err != nil {
		httpError(c, err)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		httpError(c, err)
		return
	}
	if err := tmp.Close(); err != nil {
		httpError(c, err)
		return
	}

	user := CurrentUser(c)
	asset, err := h.uc.Upload(c.Request.Context(), user.OrganizationID, usecase.MediaUploadInput{
		Filename:  fileHeader.Filename,
		LocalPath: tmpPath,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusCreated, asset)
}

func (h *MediaHandler) Delete(c *gin.Context) {
	mediaID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	user := CurrentUser(c)
	if err := h.uc.Delete(c.Request.Context(), user.OrganizationID, mediaID); err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *MediaHandler) ClearAll(c *gin.Context) {
	user := CurrentUser(c)
	if err := h.uc.ClearAll(c.Request.Context(), user.OrganizationID); err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cleared": true})
}

func (h *MediaHandler) ResetAnalysis(c *gin.Context) {
	mediaID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	user := CurrentUser(c)
	if err := h.uc.ResetAnalysis(c.Request.Context(), user.OrganizationID, mediaID); err != nil {
		httpError(c, err)
		return
	}
	detail, err := h.uc.GetDetail(c.Request.Context(), user.OrganizationID, mediaID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *MediaHandler) Stream(c *gin.Context) {
	mediaID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	user := CurrentUser(c)
	rc, size, mimeType, filename, err := h.uc.OpenStream(c.Request.Context(), user.OrganizationID, mediaID)
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

func (h *MediaHandler) Thumbnail(c *gin.Context) {
	mediaID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	user := CurrentUser(c)
	reader, size, contentType, err := h.uc.OpenThumbnail(c.Request.Context(), user.OrganizationID, mediaID)
	if err != nil {
		httpError(c, err)
		return
	}
	defer reader.Close()

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "private, max-age=300")
	c.DataFromReader(http.StatusOK, size, contentType, reader, nil)
}

func (h *MediaHandler) SegmentFrame(c *gin.Context) {
	mediaID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	frameNum, err := strconv.Atoi(c.Param("frame_number"))
	if err != nil || frameNum < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid frame_number"})
		return
	}

	user := CurrentUser(c)
	reader, size, contentType, err := h.uc.OpenSegmentFrame(c.Request.Context(), user.OrganizationID, mediaID, frameNum)
	if err != nil {
		httpError(c, err)
		return
	}
	defer reader.Close()

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "private, max-age=3600")
	c.DataFromReader(http.StatusOK, size, contentType, reader, nil)
}

func (h *MediaHandler) SearchEmbeddings(c *gin.Context) {
	projectID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	var req struct {
		Query string `json:"query" binding:"required"`
		Limit int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := CurrentUser(c)
	hits, err := h.uc.SearchEmbeddings(c.Request.Context(), user.OrganizationID, projectID, usecase.EmbeddingSearchInput{
		Query: req.Query,
		Limit: req.Limit,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"hits": hits})
}

func (h *MediaHandler) SearchProjectMoments(c *gin.Context) {
	projectID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	var req struct {
		Query           string  `json:"query" binding:"required"`
		Limit           int     `json:"limit"`
		ContextSegments int     `json:"context_segments"`
		MergeGapSec     float64 `json:"merge_gap_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := CurrentUser(c)
	moments, err := h.uc.SearchProjectMoments(c.Request.Context(), user.OrganizationID, projectID, usecase.EditorMomentQueryInput{
		Query:           req.Query,
		Limit:           req.Limit,
		ContextSegments: req.ContextSegments,
		MergeGapSec:     req.MergeGapSec,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"moments": moments})
}

func (h *MediaHandler) SearchEmbeddingsOrg(c *gin.Context) {
	var req struct {
		Query string `json:"query" binding:"required"`
		Limit int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := CurrentUser(c)
	hits, err := h.uc.SearchEmbeddingsOrg(c.Request.Context(), user.OrganizationID, usecase.EmbeddingSearchInput{
		Query: req.Query,
		Limit: req.Limit,
	})
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"hits": hits})
}
