package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter holds rate limiters for each IP address
type IPRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rps      rate.Limit
	burst    int
	cleanup  time.Duration
}

// NewIPRateLimiter creates a new IP-based rate limiter
// rps: requests per second allowed per IP
// burst: maximum burst size
func NewIPRateLimiter(rps float64, burst int) *IPRateLimiter {
	limiter := &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		cleanup:  5 * time.Minute,
	}

	// Start cleanup goroutine to remove old limiters
	go limiter.cleanupRoutine()

	return limiter
}

// getLimiter returns the rate limiter for an IP address
func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(i.rps, i.burst)
		i.limiters[ip] = limiter
	}

	return limiter
}

// cleanupRoutine periodically removes old rate limiters to prevent memory leaks
func (i *IPRateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(i.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		i.mu.Lock()
		// Remove limiters that haven't been used recently
		// In a production system, you might want to track last access time
		if len(i.limiters) > 10000 {
			// If we have too many limiters, clear half of them
			newMap := make(map[string]*rate.Limiter, len(i.limiters)/2)
			count := 0
			for k, v := range i.limiters {
				if count < len(i.limiters)/2 {
					newMap[k] = v
					count++
				}
			}
			i.limiters = newMap
		}
		i.mu.Unlock()
	}
}

// RateLimit middleware limits requests per IP address
func RateLimit(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP from X-Forwarded-For or RemoteAddr
			ip := r.Header.Get("X-Forwarded-For")
			if ip == "" {
				ip = r.Header.Get("X-Real-IP")
			}
			if ip == "" {
				ip = r.RemoteAddr
			}

			limiter := limiter.getLimiter(ip)

			if !limiter.Allow() {
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
