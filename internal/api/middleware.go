package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ifrunruhin12/money-manager/internal/utils"
)

const ContextUserIDKey = "user_id"

func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.AbortWithError(c, http.StatusUnauthorized, "unauthorized")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			utils.AbortWithError(c, http.StatusUnauthorized, "unauthorized")
			return
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			utils.AbortWithError(c, http.StatusUnauthorized, "unauthorized")
			return
		}

		userID, err := utils.ParseToken(tokenString, jwtSecret)
		if err != nil {
			utils.AbortWithError(c, http.StatusUnauthorized, "unauthorized")
			return
		}

		c.Set(ContextUserIDKey, userID)
		c.Next()
	}
}

func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		logger.Info("request completed",
			"method", method,
			"path", path,
			"status", status,
			"latency", latency.String(),
		)
	}
}

type tokenBucket struct {
	tokens         int
	capacity       int
	refillRate     int // tokens per minute
	lastRefillTime time.Time
	mu             sync.Mutex
}

func newTokenBucket(rpm int) *tokenBucket {
	return &tokenBucket{
		tokens:         rpm,
		capacity:       rpm,
		refillRate:     rpm,
		lastRefillTime: time.Now(),
	}
}

func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime)
	tokensToAdd := int(elapsed.Minutes() * float64(tb.refillRate))

	if tokensToAdd > 0 {
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefillTime = now
	}

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

func RateLimiter(rpm int) gin.HandlerFunc {
	buckets := make(map[string]*tokenBucket)
	var mu sync.RWMutex

	return func(c *gin.Context) {
		// Extract user_id from context (set by AuthMiddleware)
		userIDVal, exists := c.Get(ContextUserIDKey)
		if !exists {
			// If no user_id in context, allow the request (auth middleware will handle it)
			c.Next()
			return
		}

		userID, ok := userIDVal.(string)
		if !ok || userID == "" {
			c.Next()
			return
		}

		// Get or create bucket for this user
		mu.RLock()
		bucket, exists := buckets[userID]
		mu.RUnlock()

		if !exists {
			mu.Lock()
			// Double-check after acquiring write lock
			bucket, exists = buckets[userID]
			if !exists {
				bucket = newTokenBucket(rpm)
				buckets[userID] = bucket
			}
			mu.Unlock()
		}

		// Check if request is allowed
		if !bucket.allow() {
			utils.AbortWithError(c, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		c.Next()
	}
}

// JSONBodyValidator returns HTTP 400 with descriptive message on malformed JSON
func JSONBodyValidator() gin.HandlerFunc {
	return func(c *gin.Context) {
		// This middleware doesn't need to do anything here
		// Gin's ShouldBindJSON already returns descriptive errors
		// Handlers should use ShouldBindJSON and check for errors
		// This middleware serves as a placeholder for future validation logic
		c.Next()
	}
}
