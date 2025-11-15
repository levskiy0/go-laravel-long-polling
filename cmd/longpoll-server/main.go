package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/levskiy0/go-laravel-long-polling/internal/auth"
	"github.com/levskiy0/go-laravel-long-polling/internal/config"
	"github.com/levskiy0/go-laravel-long-polling/internal/core"
	"github.com/levskiy0/go-laravel-long-polling/internal/http"
	"github.com/levskiy0/go-laravel-long-polling/internal/redis"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		fx.Provide(config.Load),
		fx.Provide(provideLogger),
		fx.Provide(provideRedisClient),
		fx.Provide(provideJWTService),
		fx.Provide(provideLaravelUpstreamPool),
		fx.Provide(provideRedisSubscriber),
		fx.Provide(provideHTTPHandlers),
		fx.Provide(provideHTTPServer),
		fx.Invoke(registerHooks),
	)

	app.Run()
}

func provideLogger(cfg *config.Config) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: cfg.GetLogLevel(),
	}

	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func provideRedisClient(cfg *config.Config, logger *slog.Logger) *goredis.Client {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	logger.Info("Redis client created", "addr", cfg.RedisAddr)
	return client
}

func provideJWTService(cfg *config.Config, logger *slog.Logger) (*auth.JWTService, error) {
	service, err := auth.NewJWTService(cfg.JWTSecret, cfg.JWTExpiresIn, cfg.JWTAlgo)
	if err != nil {
		return nil, err
	}
	logger.Info("JWT service created")
	return service, nil
}

func provideLaravelUpstreamPool(cfg *config.Config, logger *slog.Logger) *core.LaravelUpstreamPool {
	pool := core.NewLaravelUpstreamPool(
		cfg.LaravelAddr,
		cfg.AccessTokenSecret,
		cfg.MaxLimit,
		cfg.LaravelUpstreamWorkers,
		logger,
	)
	logger.Info("Laravel upstream pool created",
		"addr", cfg.LaravelAddr,
		"workers", cfg.LaravelUpstreamWorkers,
	)
	return pool
}

func provideRedisSubscriber(client *goredis.Client, cfg *config.Config, logger *slog.Logger) *redis.Subscriber {
	subscriber := redis.NewSubscriber(client, cfg.RedisChannel, logger)
	logger.Info("Redis subscriber created", "channel", cfg.RedisChannel)
	return subscriber
}

func provideHTTPHandlers(
	jwtService *auth.JWTService,
	upstreamPool *core.LaravelUpstreamPool,
	subscriber *redis.Subscriber,
	cfg *config.Config,
	logger *slog.Logger,
) *http.Handlers {
	return http.NewHandlers(
		jwtService,
		upstreamPool,
		subscriber,
		cfg.AccessTokenSecret,
		cfg.PollTimeout,
		cfg.MaxLimit,
		logger,
	)
}

func provideHTTPServer(
	cfg *config.Config,
	handlers *http.Handlers,
	logger *slog.Logger,
) *http.Server {
	return http.NewServer(
		cfg.HTTPAddr,
		cfg.HTTPReadTimeout,
		cfg.HTTPWriteTimeout,
		handlers,
		cfg,
		logger,
	)
}

func registerHooks(
	lc fx.Lifecycle,
	server *http.Server,
	subscriber *redis.Subscriber,
	redisClient *goredis.Client,
	logger *slog.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("starting long-polling service")

			// Start Redis subscriber with auto-reconnect
			go func() {
				backoff := time.Second
				maxBackoff := time.Minute
				for {
					err := subscriber.Start(context.Background())
					if err != nil && err != context.Canceled {
						logger.Error("Redis subscriber stopped, reconnecting",
							"error", err,
							"retry_in", backoff)
						time.Sleep(backoff)
						// Exponential backoff
						backoff *= 2
						if backoff > maxBackoff {
							backoff = maxBackoff
						}
					} else {
						// Reset backoff on successful connection or graceful shutdown
						backoff = time.Second
						if err == context.Canceled {
							logger.Info("Redis subscriber shutdown gracefully")
							return
						}
					}
				}
			}()

			go func() {
				if err := server.Start(); err != nil {
					logger.Error("HTTP server stopped", "error", err)
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("stopping long-polling service")

			subscriber.Stop()

			if err := server.Stop(ctx); err != nil {
				logger.Error("failed to stop HTTP server", "error", err)
			}

			if err := redisClient.Close(); err != nil {
				logger.Error("failed to close Redis client", "error", err)
			}

			return nil
		},
	})
}
