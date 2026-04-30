package server

import (
	"net/http"

	"github.com/instructkr/smartclaw/internal/serverauth"
)

type rateLimiter = serverauth.RateLimiter

func newRateLimiter() *serverauth.RateLimiter {
	return serverauth.NewRateLimiter()
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return serverauth.CORSMiddleware(next)
}

func authMiddleware(next http.HandlerFunc, authMgr *AuthManager, noAuth bool) http.HandlerFunc {
	return serverauth.AuthMiddleware(next, authMgr, noAuth)
}

func extractToken(r *http.Request) string {
	return serverauth.ExtractToken(r)
}

func validateAccessToken(token string, authMgr *AuthManager) bool {
	return serverauth.ValidateAccessToken(token, authMgr)
}
