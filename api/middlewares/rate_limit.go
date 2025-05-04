package middlewares

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/medinapdr/world-gen/config"
	"github.com/redis/go-redis/v9"
)

// RateLimiter implements a request rate limiting middleware
type RateLimiter struct {
	redisClient *redis.Client
	appConfig   *config.AppConfig
}

// NewRateLimiter creates a new instance of the rate limiting middleware
func NewRateLimiter(redisClient *redis.Client, appConfig *config.AppConfig) *RateLimiter {
	return &RateLimiter{
		redisClient: redisClient,
		appConfig:   appConfig,
	}
}

// Middleware returns an Echo middleware for rate limiting
func (r *RateLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip rate limiting for health checks
			if isExemptFromRateLimit(c.Path()) {
				return next(c)
			}

			// Skip if Redis is not available
			if r.redisClient == nil {
				return next(c)
			}

			// Check and enforce rate limits
			if exceeded, err := r.isRateLimitExceeded(c); err != nil {
				// Continue on Redis errors to avoid blocking requests
				return next(c)
			} else if exceeded {
				return respondWithRateLimitExceeded(c)
			}

			return next(c)
		}
	}
}

// isExemptFromRateLimit checks if a path should be excluded from rate limiting
func isExemptFromRateLimit(path string) bool {
	// Add more exempt paths here as needed
	return path == "/health"
}

// isRateLimitExceeded checks if the client has exceeded their rate limit
func (r *RateLimiter) isRateLimitExceeded(c echo.Context) (bool, error) {
	userIP := c.RealIP()
	key := "rate-limit:" + userIP
	ctx := context.Background()

	// Increment request counter for this IP
	count, err := r.redisClient.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}

	// Set expiration on new keys
	if count == 1 {
		windowDuration := time.Duration(r.appConfig.RateWindow) * time.Second
		r.redisClient.Expire(ctx, key, windowDuration)
	}

	// Check if limit exceeded
	return count > int64(r.appConfig.RateLimit), nil
}

// respondWithRateLimitExceeded returns a standard rate limit exceeded response
func respondWithRateLimitExceeded(c echo.Context) error {
	return c.JSON(http.StatusTooManyRequests, map[string]string{
		"error": "Request limit exceeded. Try again later.",
	})
}
