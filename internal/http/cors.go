package http

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/levskiy0/go-laravel-long-polling/internal/config"
)

// CORSMiddleware creates CORS middleware based on config
func CORSMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set CORS headers
		c.Writer.Header().Set("Access-Control-Allow-Origin", cfg.CORSAllowedOrigins)
		c.Writer.Header().Set("Access-Control-Allow-Methods", cfg.CORSAllowedMethods)
		c.Writer.Header().Set("Access-Control-Allow-Headers", cfg.CORSAllowedHeaders)
		c.Writer.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.CORSMaxAge))

		if cfg.CORSAllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
