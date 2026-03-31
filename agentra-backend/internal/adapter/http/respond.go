package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/reaganiwadha/agentra/internal/usecase"
	"github.com/sirupsen/logrus"
)

func parseUUIDParam(c *gin.Context, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + param})
		return uuid.Nil, false
	}
	return id, true
}

func startSSE(c *gin.Context) (http.Flusher, bool) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming is not supported"})
		return nil, false
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()
	return flusher, true
}

func httpError(c *gin.Context, err error) {
	switch {
	case errorx.IsOfType(err, usecase.ErrBadRequest):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errorx.IsOfType(err, usecase.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
	case errorx.IsOfType(err, usecase.ErrInvalidSetupToken),
		errorx.IsOfType(err, usecase.ErrSetupDone),
		errorx.IsOfType(err, usecase.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errorx.IsOfType(err, usecase.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errorx.IsOfType(err, usecase.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		if l, ok := c.Get("logger"); ok {
			l.(*logrus.Logger).WithError(err).Error("internal error")
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
