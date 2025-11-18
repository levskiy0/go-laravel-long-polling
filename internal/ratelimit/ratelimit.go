package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Limiter provides rate limiting functionality using Redis
type Limiter struct {
	redis  *goredis.Client
	logger *slog.Logger

	limitPerIP    int
	limitPerToken int

	window time.Duration
}

// NewLimiter creates a new rate limiter
func NewLimiter(redis *goredis.Client, limitPerIP, limitPerToken int, logger *slog.Logger) *Limiter {
	return &Limiter{
		redis:         redis,
		logger:        logger,
		limitPerIP:    limitPerIP,
		limitPerToken: limitPerToken,
		window:        time.Second,
	}
}

// CheckIP checks if the IP address is within rate limits
func (l *Limiter) CheckIP(ctx context.Context, ip string) (bool, error) {
	if l.limitPerIP <= 0 {
		return true, nil
	}

	key := fmt.Sprintf("ratelimit:ip:%s", ip)
	return l.checkLimit(ctx, key, l.limitPerIP)
}

// CheckToken checks if the token is within rate limits
func (l *Limiter) CheckToken(ctx context.Context, token string) (bool, error) {
	if l.limitPerToken <= 0 {
		return true, nil
	}

	key := fmt.Sprintf("ratelimit:token:%s", token)
	return l.checkLimit(ctx, key, l.limitPerToken)
}

// checkLimit performs the actual rate limit check using Redis with fixed window
func (l *Limiter) checkLimit(ctx context.Context, key string, limit int) (bool, error) {
	added := l.redis.SetNX(ctx, key, 0, l.window).Val()

	count, err := l.redis.Incr(ctx, key).Result()
	if err != nil {
		l.logger.Error("failed to increment rate limit counter", "error", err, "key", key)
		return true, err
	}

	if added && count != 1 {
		l.redis.Expire(ctx, key, l.window)
	}

	if count > int64(limit) {
		l.logger.Warn("rate limit exceeded",
			"key", key,
			"count", count,
			"limit", limit,
		)
		return false, nil
	}

	return true, nil
}

// Reset resets the rate limit for a specific key (useful for testing)
func (l *Limiter) Reset(ctx context.Context, ip, token string) error {
	keys := []string{}
	if ip != "" {
		keys = append(keys, fmt.Sprintf("ratelimit:ip:%s", ip))
	}
	if token != "" {
		keys = append(keys, fmt.Sprintf("ratelimit:token:%s", token))
	}

	if len(keys) == 0 {
		return nil
	}

	return l.redis.Del(ctx, keys...).Err()
}
