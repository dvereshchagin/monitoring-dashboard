package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	gatewaymetrics "github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/metrics"
	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter applies global and per-client request limits.
type Limiter struct {
	global *rate.Limiter
	perIP  map[string]*clientLimiter
	mu     sync.Mutex

	rps   rate.Limit
	burst int
}

func New(rps float64, burst int) *Limiter {
	return &Limiter{
		global: rate.NewLimiter(rate.Limit(rps), burst),
		perIP:  make(map[string]*clientLimiter),
		rps:    rate.Limit(rps),
		burst:  burst,
	}
}

func (l *Limiter) Middleware(metrics *gatewaymetrics.Metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !l.allow(ip) {
			metrics.RateLimitDropped.Inc()
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *Limiter) allow(ip string) bool {
	if !l.global.Allow() {
		return false
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	item, ok := l.perIP[ip]
	if !ok {
		item = &clientLimiter{limiter: rate.NewLimiter(l.rps, l.burst), lastSeen: time.Now()}
		l.perIP[ip] = item
	}

	item.lastSeen = time.Now()
	if len(l.perIP) > 10_000 {
		l.cleanupLocked(time.Now().Add(-10 * time.Minute))
	}

	return item.limiter.Allow()
}

func (l *Limiter) cleanupLocked(threshold time.Time) {
	for ip, entry := range l.perIP {
		if entry.lastSeen.Before(threshold) {
			delete(l.perIP, ip)
		}
	}
}

func clientIP(r *http.Request) string {
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
