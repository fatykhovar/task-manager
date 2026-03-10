package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatykhovar/task-manager/internal/config"
	"github.com/fatykhovar/task-manager/internal/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// Prometheus metrics
var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

// Logger middleware
func Logger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(ww, r)
			logger.Info("request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.status),
				zap.Duration("duration", time.Since(start)),
			)
		})
	}
}

// Metrics middleware
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(ww, r)

			status := strconv.Itoa(ww.status)
			path := r.URL.Path
			httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
			httpRequestDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
		})
	}
}

// Auth middleware validates JWT and sets user ID in context
func Auth(jwtCfg config.JWTConfig) func(http.Handler) http.Handler {
	authSvc := &service.AuthService{}
	_ = authSvc // replaced by closure

	// Mini inline validator to avoid circular imports
	validate := func(tokenStr string) (int64, error) {
		tmp := service.AuthService{}
		_ = tmp
		// Use the real auth service via a package-level helper
		return authServiceValidator(tokenStr, jwtCfg.Secret)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			userID, err := validate(tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RateLimit implements per-user rate limiting (token bucket, in-memory)
func RateLimit(cfg config.RateLimitConfig) func(http.Handler) http.Handler {
	type bucket struct {
		tokens    float64
		lastCheck time.Time
		mu        sync.Mutex
	}

	var (
		buckets sync.Map
		rate    = float64(cfg.RequestsPerMinute) / 60.0 // per second
		max     = float64(cfg.RequestsPerMinute)
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr // could also use JWT user ID

			val, _ := buckets.LoadOrStore(key, &bucket{tokens: max, lastCheck: time.Now()})
			b := val.(*bucket)

			b.mu.Lock()
			now := time.Now()
			elapsed := now.Sub(b.lastCheck).Seconds()
			b.tokens += elapsed * rate
			if b.tokens > max {
				b.tokens = max
			}
			b.lastCheck = now

			if b.tokens < 1 {
				b.mu.Unlock()
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			b.tokens--
			b.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// GetUserID extracts the authenticated user ID from context
func GetUserID(r *http.Request) (int64, bool) {
	id, ok := r.Context().Value(UserIDKey).(int64)
	return id, ok
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
