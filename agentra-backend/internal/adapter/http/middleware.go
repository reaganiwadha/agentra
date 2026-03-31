package handler

import (
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Middleware struct {
	sessions *postgres.SessionRepo
	users    *postgres.UserRepo
}

func NewMiddleware(sessions *postgres.SessionRepo, users *postgres.UserRepo) *Middleware {
	return &Middleware{sessions: sessions, users: users}
}

func (m *Middleware) Require(c *gin.Context) {
	raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if raw == "" {
		raw = c.Query("token") // fallback for media stream endpoints used by <video>/<audio>
	}
	if raw == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	sessionID, err := uuid.Parse(raw)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	session, err := m.sessions.GetByID(c.Request.Context(), sessionID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := m.users.GetByID(c.Request.Context(), session.UserID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.Set("session", session)
	c.Set("user", user)
	c.Next()
}

func (m *Middleware) RequireAdmin(c *gin.Context) {
	user := CurrentUser(c)
	if user.Role != domain.RoleAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	c.Next()
}

func CurrentUser(c *gin.Context) domain.User {
	return c.MustGet("user").(domain.User)
}

func CurrentSession(c *gin.Context) domain.Session {
	return c.MustGet("session").(domain.Session)
}
