package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/levskiy0/go-laravel-long-polling/internal/config"
)

type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

func NewServer(
	addr string,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	handlers *Handlers,
	cfg *config.Config,
	logger *slog.Logger,
) *Server {
	// Set Gin mode based on log level
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(CORSMiddleware(cfg))

	// Add custom logger middleware
	router.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		logger.Info("request completed",
			"method", c.Request.Method,
			"path", path,
			"status", statusCode,
			"latency", latency.String(),
		)
	})

	// Register routes
	router.GET("/health", handlers.Health)
	router.POST("/getAccessToken", handlers.GetAccessToken)
	router.GET("/getUpdates", handlers.GetUpdates)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	return &Server{
		httpServer: httpServer,
		logger:     logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("stopping HTTP server")
	return s.httpServer.Shutdown(ctx)
}
