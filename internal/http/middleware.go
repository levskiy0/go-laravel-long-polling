package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/levskiy0/go-laravel-long-polling/internal/auth"
	"github.com/levskiy0/go-laravel-long-polling/internal/ratelimit"
)

const (
	ContextKeyChannelID = "channel_id"
	ContextKeyToken     = "token"
)

// JWTMiddleware validates JWT token and puts channel_id and token into context
func JWTMiddleware(jwtService *auth.JWTService, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.Query("token")
		if tokenString == "" {
			logger.Warn("missing token")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "token is required",
			})
			c.Abort()
			return
		}

		channelID, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			logger.Warn("invalid token", "error", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Put channel_id and token into context
		c.Set(ContextKeyChannelID, channelID)
		c.Set(ContextKeyToken, tokenString)

		c.Next()
	}
}

// RateLimitMiddleware checks both IP and token-based rate limits
// Expects token and channel_id to be already validated and in context (via JWTMiddleware)
func RateLimitMiddleware(rateLimiter *ratelimit.Limiter, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		allowed, err := rateLimiter.CheckIP(c.Request.Context(), clientIP)
		if err != nil {
			logger.Error("rate limiter error", "error", err, "ip", clientIP)
		}
		if !allowed {
			logger.Warn("rate limit exceeded", "ip", clientIP, "endpoint", c.Request.URL.Path)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please try again later.",
			})
			c.Abort()
			return
		}

		// Check token-based rate limit (token is already validated by JWTMiddleware)
		tokenString, exists := c.Get(ContextKeyToken)
		if exists {
			token := tokenString.(string)
			channelID, _ := c.Get(ContextKeyChannelID)

			allowed, err := rateLimiter.CheckToken(c.Request.Context(), token)
			if err != nil {
				logger.Error("rate limiter error for token", "error", err, "channel_id", channelID)
			}
			if !allowed {
				logger.Warn("token rate limit exceeded", "channel_id", channelID, "endpoint", c.Request.URL.Path)
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "Too many requests. Please try again later.",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
