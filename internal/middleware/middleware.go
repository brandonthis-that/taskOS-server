package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/brandonthis-that/taskOS-server/internal/auth"
)

// Logger logs each request with method, path, and status.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Recover catches panics and returns 500.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "error", rec)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Authenticate validates Bearer API keys.
func Authenticate(svc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := auth.ParseBearer(r.Header.Get("Authorization"))
			if err != nil {
				http.Error(w, `{"error":"missing or invalid authorization","code":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			userID, username, err := svc.ValidateAPIKey(r.Context(), token)
			if err != nil {
				http.Error(w, `{"error":"invalid api key","code":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			ctx := auth.WithUser(r.Context(), userID, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CORS allows configured origins.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowAll || originAllowed(origin, allowedOrigins) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func originAllowed(origin string, allowed []string) bool {
	for _, o := range allowed {
		if strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}

// RateLimit applies a simple per-IP token bucket (requests per minute).
func RateLimit(perMinute int) func(http.Handler) http.Handler {
	var mu sync.Mutex
	buckets := map[string]*rateBucket{}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !allow(&mu, buckets, ip, perMinute) {
				http.Error(w, `{"error":"rate limit exceeded","code":"rate_limited"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type rateBucket struct {
	count   int
	resetAt time.Time
}

func allow(mu *sync.Mutex, buckets map[string]*rateBucket, key string, limit int) bool {
	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	b, ok := buckets[key]
	if !ok || now.After(b.resetAt) {
		buckets[key] = &rateBucket{count: 1, resetAt: now.Add(time.Minute)}
		return true
	}
	if b.count >= limit {
		return false
	}
	b.count++
	return true
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		return host[:i]
	}
	return host
}
