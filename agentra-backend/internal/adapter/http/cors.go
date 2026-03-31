package handler

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS(originsCSV string) gin.HandlerFunc {
	cfg := cors.Config{
		AllowMethods:  []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:  []string{"Authorization", "Content-Type", "Accept", "Range"},
		ExposeHeaders: []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Content-Disposition", "Cache-Control"},
		MaxAge:        86400 * time.Second,
	}

	origins := parseOrigins(originsCSV)
	if len(origins) == 1 && origins[0] == "*" {
		cfg.AllowAllOrigins = true
	} else {
		cfg.AllowOrigins = origins
	}

	return cors.New(cfg)
}

func parseOrigins(csv string) []string {
	var out []string
	for _, o := range strings.Split(csv, ",") {
		if s := strings.TrimSpace(o); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}
