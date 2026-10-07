package middleware

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateBucket struct {
	count    int
	resetAt  time.Time
	lastSeen time.Time
}

type IPRateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*rateBucket
	limit      int
	window     time.Duration
	staleAfter time.Duration
	maxEntries int
	now        func() time.Time
}

func NewIPRateLimiter(limit int, window, staleAfter time.Duration, maxEntries int) (*IPRateLimiter, error) {
	if limit <= 0 {
		return nil, errors.New("rate limit must be positive")
	}
	if window <= 0 || staleAfter <= 0 {
		return nil, errors.New("rate-limit durations must be positive")
	}
	if maxEntries <= 0 {
		return nil, errors.New("rate-limit bucket bound must be positive")
	}
	return &IPRateLimiter{
		buckets:    make(map[string]*rateBucket),
		limit:      limit,
		window:     window,
		staleAfter: staleAfter,
		maxEntries: maxEntries,
		now:        time.Now,
	}, nil
}

func (l *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := l.now()
		allowed, retryAfter := l.allow(c.ClientIP(), now)
		if !allowed {
			seconds := int(math.Ceil(retryAfter.Seconds()))
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(seconds))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			return
		}
		c.Next()
	}
}

func (l *IPRateLimiter) allow(ip string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.removeStale(now)
	bucket, ok := l.buckets[ip]
	if !ok {
		if len(l.buckets) >= l.maxEntries {
			l.evictOldest()
		}
		bucket = &rateBucket{resetAt: now.Add(l.window)}
		l.buckets[ip] = bucket
	}
	if !now.Before(bucket.resetAt) {
		bucket.count = 0
		bucket.resetAt = now.Add(l.window)
	}
	bucket.lastSeen = now
	if bucket.count >= l.limit {
		return false, bucket.resetAt.Sub(now)
	}
	bucket.count++
	return true, 0
}

func (l *IPRateLimiter) removeStale(now time.Time) {
	for ip, bucket := range l.buckets {
		if !bucket.lastSeen.IsZero() && now.Sub(bucket.lastSeen) >= l.staleAfter {
			delete(l.buckets, ip)
		}
	}
}

func (l *IPRateLimiter) evictOldest() {
	var oldestIP string
	var oldest time.Time
	for ip, bucket := range l.buckets {
		if oldestIP == "" || bucket.lastSeen.Before(oldest) {
			oldestIP = ip
			oldest = bucket.lastSeen
		}
	}
	if oldestIP != "" {
		delete(l.buckets, oldestIP)
	}
}

func (l *IPRateLimiter) bucketCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
