package http

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/levskiy0/go-laravel-long-polling/internal/auth"
	"github.com/levskiy0/go-laravel-long-polling/internal/core"
	"github.com/levskiy0/go-laravel-long-polling/internal/redis"
)

type Handlers struct {
	jwtService      *auth.JWTService
	upstreamPool    *core.LaravelUpstreamPool
	subscriber      *redis.Subscriber
	accessSecret    string
	pollTimeout     time.Duration
	maxLimit        int
	logger          *slog.Logger
}

func NewHandlers(
	jwtService *auth.JWTService,
	upstreamPool *core.LaravelUpstreamPool,
	subscriber *redis.Subscriber,
	accessSecret string,
	pollTimeout time.Duration,
	maxLimit int,
	logger *slog.Logger,
) *Handlers {
	return &Handlers{
		jwtService:   jwtService,
		upstreamPool: upstreamPool,
		subscriber:   subscriber,
		accessSecret: accessSecret,
		pollTimeout:  pollTimeout,
		maxLimit:     maxLimit,
		logger:       logger,
	}
}

// GetAccessToken handles the /getAccessToken endpoint
// POST /getAccessToken?channel_id=...&secret=...
func (h *Handlers) GetAccessToken(c *gin.Context) {
	channelID := c.Query("channel_id")
	secret := c.Query("secret")

	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "channel_id is required",
		})
		return
	}

	if secret != h.accessSecret {
		h.logger.Warn("invalid access secret", "channel_id", channelID)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	token, err := h.jwtService.GenerateToken(channelID)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err, "channel_id", channelID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate token",
		})
		return
	}

	h.logger.Info("token generated", "channel_id", channelID)

	c.JSON(http.StatusOK, gin.H{
		"token": token,
	})
}

// GetUpdates handles the /getUpdates endpoint
// GET /getUpdates?token=...&offset=...&limit=...
func (h *Handlers) GetUpdates(c *gin.Context) {
	tokenString := c.Query("token")
	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "100")

	// Validate token
	if tokenString == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "token is required",
		})
		return
	}

	channelID, err := h.jwtService.ValidateToken(tokenString)
	if err != nil {
		h.logger.Warn("invalid token", "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid or expired token",
		})
		return
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		offset = 0
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 100
	}
	if limit > h.maxLimit {
		limit = h.maxLimit
	}

	h.logger.Debug("getUpdates request",
		"channel_id", channelID,
		"offset", offset,
		"limit", limit,
	)

	ctx := c.Request.Context()
	events, err := h.upstreamPool.GetEvents(ctx, channelID, offset, limit)
	if err != nil {
		h.logger.Error("failed to fetch events from Laravel", "error", err, "channel_id", channelID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch events",
		})
		return
	}

	if len(events) > 0 {
		h.logger.Debug("returning immediate events",
			"channel_id", channelID,
			"count", len(events),
		)
		c.JSON(http.StatusOK, gin.H{
			"events": events,
		})
		return
	}

	notifyCh := h.subscriber.Subscribe(channelID)
	defer h.subscriber.Unsubscribe(channelID, notifyCh)

	pollCtx, cancel := context.WithTimeout(ctx, h.pollTimeout)
	defer cancel()

	select {
	case <-pollCtx.Done():
		// Timeout - return empty response
		h.logger.Debug("poll timeout", "channel_id", channelID)
		c.JSON(http.StatusOK, gin.H{
			"events": []interface{}{},
		})
		return

	case notification := <-notifyCh:
		// New event notification received, fetch events again
		h.logger.Debug("notification received",
			"channel_id", channelID,
			"event_id", notification.EventID,
		)

		events, err := h.upstreamPool.GetEvents(c.Request.Context(), channelID, offset, limit)
		if err != nil {
			h.logger.Error("failed to fetch events after notification",
				"error", err,
				"channel_id", channelID,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to fetch events",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"events": events,
		})
		return
	}
}

// Health check endpoint
func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
