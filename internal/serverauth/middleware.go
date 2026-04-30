package serverauth

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter provides IP-based request rate limiting.
type RateLimiter struct {
	visitors map[string]*visitorInfo
	mu       sync.Mutex
}

type visitorInfo struct {
	count    int
	lastSeen time.Time
}

// NewRateLimiter creates a new RateLimiter that allows 100 requests per minute per IP.
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitorInfo),
	}
	go rl.cleanup()
	return rl
}

// Middleware wraps an http.HandlerFunc with rate limiting.
func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := strings.Split(r.RemoteAddr, ":")[0]
		rl.mu.Lock()
		v, ok := rl.visitors[ip]
		if !ok {
			v = &visitorInfo{}
			rl.visitors[ip] = v
		}
		v.count++
		v.lastSeen = time.Now()
		if v.count > 100 {
			rl.mu.Unlock()
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		rl.mu.Unlock()
		next(w, r)
	}
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > time.Minute {
				delete(rl.visitors, ip)
			} else {
				v.count = 0
			}
		}
		rl.mu.Unlock()
	}
}

// VisitorCount returns the request count for a given IP address.
func (rl *RateLimiter) VisitorCount(ip string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if v, ok := rl.visitors[ip]; ok {
		return v.count
	}
	return 0
}

// CORSMiddleware wraps an http.HandlerFunc with CORS headers.
func CORSMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

// AuthMiddleware wraps an http.HandlerFunc with authentication checks.
// It skips auth for WebSocket upgrade requests and when noAuth is true.
// Token extraction order: cookie "smartclaw-token" → Authorization: Bearer → ?token= query param.
func AuthMiddleware(next http.HandlerFunc, authMgr *AuthManager, noAuth bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" {
			next(w, r)
			return
		}

		if noAuth {
			next(w, r)
			return
		}

		if authMgr != nil && !authMgr.IsAuthRequired() {
			next(w, r)
			return
		}

		token := ExtractToken(r)
		if !ValidateAccessToken(token, authMgr) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// ExtractToken extracts a token from the request using the priority:
// 1. cookie "smartclaw-token"  2. Authorization: Bearer header  3. ?token= query param
func ExtractToken(r *http.Request) string {
	if cookie, err := r.Cookie("smartclaw-token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	token := r.Header.Get("Authorization")
	if strings.HasPrefix(token, "Bearer ") {
		return strings.TrimPrefix(token, "Bearer ")
	}
	if token != "" {
		return token
	}

	return r.URL.Query().Get("token")
}

// ValidateAccessToken checks whether a token is valid against the AuthManager.
func ValidateAccessToken(token string, authMgr *AuthManager) bool {
	if token == "" {
		return false
	}

	if authMgr == nil {
		return false
	}

	if session, err := authMgr.ValidateToken(token); err == nil && session != nil {
		return true
	}

	if authMgr.ValidateLegacyToken(token) {
		return true
	}

	return false
}
