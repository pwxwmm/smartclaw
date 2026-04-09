package upstreamproxy

import (
	"net/http"
)

type RelayMiddleware func(next http.Handler) http.Handler

type RequestRelay struct {
	proxy      *UpstreamProxy
	middleware []RelayMiddleware
}

func NewRequestRelay(proxy *UpstreamProxy) *RequestRelay {
	return &RequestRelay{
		proxy:      proxy,
		middleware: make([]RelayMiddleware, 0),
	}
}

func (r *RequestRelay) AddMiddleware(m RelayMiddleware) {
	r.middleware = append(r.middleware, m)
}

func (r *RequestRelay) Handle(w http.ResponseWriter, req *http.Request) {
	handler := http.Handler(r.proxy)

	for i := len(r.middleware) - 1; i >= 0; i-- {
		handler = r.middleware[i](handler)
	}

	handler.ServeHTTP(w, req)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		next.ServeHTTP(w, req)
	})
}

func AuthMiddleware(token string) RelayMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			authHeader := req.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

func RateLimitMiddleware(requestsPerSecond int) RelayMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req)
		})
	}
}

func CORSMiddleware(allowedOrigins []string) RelayMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			origin := req.Header.Get("Origin")
			for _, allowed := range allowedOrigins {
				if origin == allowed || allowed == "*" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					break
				}
			}

			if req.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		next.ServeHTTP(w, req)
	})
}
