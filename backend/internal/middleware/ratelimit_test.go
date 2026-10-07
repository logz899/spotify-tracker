package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestIPRateLimiterExhaustionAndWindowRecovery(t *testing.T) {
	limiter, err := NewIPRateLimiter(2, time.Minute, 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("NewIPRateLimiter() error = %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.now = func() time.Time { return now }
	router := rateLimitedRouter(limiter)

	if code := performLimitedRequest(router, "192.0.2.1:1000"); code != http.StatusNoContent {
		t.Fatalf("first status = %d", code)
	}
	if code := performLimitedRequest(router, "192.0.2.1:1001"); code != http.StatusNoContent {
		t.Fatalf("second status = %d", code)
	}
	if code := performLimitedRequest(router, "192.0.2.1:1002"); code != http.StatusTooManyRequests {
		t.Fatalf("exhausted status = %d, want 429", code)
	}
	if code := performLimitedRequest(router, "198.51.100.2:1000"); code != http.StatusNoContent {
		t.Fatalf("separate IP status = %d", code)
	}

	now = now.Add(time.Minute)
	if code := performLimitedRequest(router, "192.0.2.1:1003"); code != http.StatusNoContent {
		t.Fatalf("recovered status = %d", code)
	}
}

func TestIPRateLimiterBoundsAndCleansBuckets(t *testing.T) {
	limiter, err := NewIPRateLimiter(1, time.Minute, 2*time.Minute, 2)
	if err != nil {
		t.Fatalf("NewIPRateLimiter() error = %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.now = func() time.Time { return now }
	router := rateLimitedRouter(limiter)

	performLimitedRequest(router, "192.0.2.1:1000")
	now = now.Add(time.Second)
	performLimitedRequest(router, "192.0.2.2:1000")
	now = now.Add(time.Second)
	performLimitedRequest(router, "192.0.2.3:1000")
	if got := limiter.bucketCount(); got != 2 {
		t.Fatalf("bucket count = %d, want bounded count 2", got)
	}

	now = now.Add(3 * time.Minute)
	performLimitedRequest(router, "192.0.2.4:1000")
	if got := limiter.bucketCount(); got != 1 {
		t.Fatalf("bucket count after stale cleanup = %d, want 1", got)
	}
}

func rateLimitedRouter(limiter *IPRateLimiter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(limiter.Middleware())
	router.POST("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return router
}

func performLimitedRequest(router http.Handler, remoteAddress string) int {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.RemoteAddr = remoteAddress
	router.ServeHTTP(response, request)
	return response.Code
}
