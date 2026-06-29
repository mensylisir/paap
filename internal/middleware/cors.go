package middleware

import (
	"os"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowOrigins = allowedOrigins()
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	config.AllowCredentials = true
	return cors.New(config)
}

func allowedOrigins() []string {
	origins := []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:3000",
		"http://127.0.0.1:3000",
	}
	for _, item := range strings.Split(os.Getenv("PAAP_CORS_ALLOWED_ORIGINS"), ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			origins = append(origins, item)
		}
	}
	return origins
}
