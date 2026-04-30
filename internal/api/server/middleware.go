package server

import (
	"encoding/json"
	"net/http"

	"github.com/instructkr/smartclaw/internal/serverauth"
)

type authManager = serverauth.AuthManager

func newAuthManager() (*serverauth.AuthManager, error) {
	return serverauth.NewAuthManager()
}

type rateLimiter = serverauth.RateLimiter

func newRateLimiter() *serverauth.RateLimiter {
	return serverauth.NewRateLimiter()
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return serverauth.CORSMiddleware(next)
}

func (s *APIServer) wrapHandler(next http.HandlerFunc) http.HandlerFunc {
	return serverauth.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		serverauth.AuthMiddleware(next, s.auth, s.noAuth)(w, r)
	})
}

func extractToken(r *http.Request) string {
	return serverauth.ExtractToken(r)
}

func validateAccessToken(token string, am *serverauth.AuthManager) bool {
	return serverauth.ValidateAccessToken(token, am)
}

func getUserID(r *http.Request) string {
	return "default"
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
