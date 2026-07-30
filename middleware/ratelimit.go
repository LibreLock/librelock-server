package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	tokens   float64
	lastFill time.Time
}

// RateLimit is a per-client token bucket: burst requests may arrive at once, then one more every 1/perSecond
// It fronts the auth endpoints that hash, where each request can cost a 64 MiB argon2 hash
// Clients are keyed on gin's ClientIP, which honours X-Forwarded-For only for trusted proxies (TRUSTED_PROXIES)
// Behind an untrusted-by-default proxy every request looks like the proxy, so the whole deployment shares one bucket - set TRUSTED_PROXIES when running behind one
func RateLimit(burst int, perSecond float64) gin.HandlerFunc {
	var (
		mu        sync.Mutex
		buckets   = map[string]*bucket{}
		lastSweep = time.Now()
	)

	// A bucket that has been full this long is indistinguishable from a fresh one, so it can go
	idle := time.Duration(float64(burst)/perSecond) * time.Second
	if idle < time.Minute {
		idle = time.Minute
	}

	return func(c *gin.Context) {
		key := c.ClientIP()
		now := time.Now()

		mu.Lock()
		if now.Sub(lastSweep) > idle {
			for k, b := range buckets {
				if now.Sub(b.lastFill) > idle {
					delete(buckets, k)
				}
			}
			lastSweep = now
		}

		b, ok := buckets[key]
		if !ok {
			b = &bucket{tokens: float64(burst), lastFill: now}
			buckets[key] = b
		}

		b.tokens += now.Sub(b.lastFill).Seconds() * perSecond
		if b.tokens > float64(burst) {
			b.tokens = float64(burst)
		}
		b.lastFill = now

		allowed := b.tokens >= 1
		if allowed {
			b.tokens--
		}
		retryAfter := 0
		if !allowed {
			retryAfter = int((1-b.tokens)/perSecond) + 1
		}
		mu.Unlock()

		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please wait a moment and try again.",
			})
			return
		}

		c.Next()
	}
}
