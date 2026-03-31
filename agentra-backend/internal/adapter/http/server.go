package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/reaganiwadha/agentra/internal/config"
	"github.com/sirupsen/logrus"
	"go.uber.org/fx"
)

func NewEngine(cfg config.Config, log *logrus.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(CORS(cfg.CORSOrigins))
	r.Use(gin.Recovery())
	r.Use(requestLogger(log))
	return r
}

func requestLogger(log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Set("logger", log)
		c.Next()
		log.WithFields(logrus.Fields{
			"method":  c.Request.Method,
			"path":    c.Request.URL.Path,
			"status":  c.Writer.Status(),
			"latency": time.Since(start).String(),
			"ip":      c.ClientIP(),
		}).Info("request")
	}
}

func RegisterRoutes(
	r *gin.Engine,
	mid *Middleware,
	setup *SetupHandler,
	auth *AuthHandler,
	users *UserHandler,
	storage *StorageHandler,
	providers *ProviderHandler,
	analyzers *AnalyzerAdminHandler,
	editor *EditorHandler,
	projects *ProjectHandler,
	media *MediaHandler,
	activity *ActivityHandler,
) {
	r.GET("/setup", setup.Status)
	r.GET("/setup/options", setup.Options)
	r.POST("/setup/validate", setup.Validate)
	r.POST("/setup/storage/validate", setup.ValidateStorage)
	r.POST("/setup", setup.Handle)
	r.POST("/login", auth.Login)

	authed := r.Group("/")
	authed.Use(mid.Require)
	authed.POST("/logout", auth.Logout)
	authed.GET("/me", users.Me)
	authed.GET("/storage", storage.Get)
	authed.GET("/storage/status", storage.Status)
	authed.GET("/editor-config", editor.Get)
	authed.GET("/projects", projects.List)
	authed.POST("/projects", projects.Create)
	authed.GET("/projects/:id", projects.Get)
	authed.PUT("/projects/:id", projects.UpdateDraft)
	authed.PUT("/projects/:id/media-scope", projects.SetMediaScope)
	authed.GET("/projects/:id/runs", projects.ListRuns)
	authed.POST("/projects/:id/runs", projects.QueueRuns)
	authed.GET("/runs/:id", projects.GetRun)
	authed.POST("/runs/:id/cancel", projects.CancelRun)
	authed.GET("/renders/:id/stream", projects.StreamRender)
	authed.GET("/media", media.ListByOrg)
	authed.POST("/media/upload", media.Upload)
	authed.POST("/media/clear-all", media.ClearAll)
	authed.POST("/media/search", media.SearchEmbeddingsOrg)
	authed.GET("/projects/:id/media", media.ListByProject)
	authed.POST("/projects/:id/embeddings/search", media.SearchEmbeddings)
	authed.POST("/projects/:id/editor/query", media.SearchProjectMoments)
	authed.GET("/media-assets/:id", media.Get)
	authed.DELETE("/media-assets/:id", media.Delete)
	authed.POST("/media-assets/:id/reset-analysis", media.ResetAnalysis)
	authed.GET("/media-assets/:id/stream", media.Stream)
	authed.GET("/media/:id/thumbnail", media.Thumbnail)
	authed.GET("/media-assets/:id/thumbnail", media.Thumbnail)
	authed.GET("/media-assets/:id/segment-frame/:frame_number", media.SegmentFrame)
	authed.GET("/activity/status", activity.Status)
	authed.GET("/activity/logs", activity.List)
	authed.GET("/activity/stream", activity.Stream)

	admin := authed.Group("/")
	admin.Use(mid.RequireAdmin)
	admin.POST("/users", users.Create)
	admin.GET("/admin/providers/types", providers.Types)
	admin.GET("/admin/providers", providers.List)
	admin.POST("/admin/providers", providers.Create)
	admin.PUT("/admin/providers/:id", providers.Update)
	admin.DELETE("/admin/providers/:id", providers.Delete)
	admin.POST("/admin/providers/test-get", providers.TestGet)
	admin.GET("/admin/analyzers", analyzers.List)
	admin.POST("/admin/analyzers", analyzers.Create)
	admin.PUT("/admin/analyzers/:id", analyzers.Update)
	admin.DELETE("/admin/analyzers/:id", analyzers.Delete)
	admin.GET("/admin/analyzers/:id/test/stream", analyzers.StreamTest)
	admin.GET("/admin/editor-agent/test/stream", editor.StreamAgentTest)
	admin.PUT("/storage", storage.Set)
	admin.PUT("/editor-config", editor.Set)
	admin.POST("/setup/reset", setup.Reset)
}

func StartServer(lc fx.Lifecycle, r *gin.Engine, cfg config.Config, log *logrus.Logger) {
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				log.Infof("listening on :%s", cfg.Port)
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.WithError(err).Fatal("server error")
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
}
