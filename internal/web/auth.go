package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	sessionDuration = 24 * time.Hour
	cleanupInterval = 1 * time.Hour
)

// Session represents an authenticated user session.
type Session struct {
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	UserID    string    `json:"user_id"`
}

// AuthManager manages HMAC-SHA256 session tokens.
type AuthManager struct {
	secretKey  []byte
	sessions   map[string]*Session
	mu         sync.Mutex
	apiKey     string // configured API key for login validation
	legacyToken string // SMARTCLAW_AUTH_TOKEN for backward compatibility
}

// NewAuthManager creates a new AuthManager with a random secret key.
func NewAuthManager() *AuthManager {
	secretKey := make([]byte, 32)
	if _, err := rand.Read(secretKey); err != nil {
		panic(fmt.Sprintf("failed to generate secret key: %v", err))
	}

	am := &AuthManager{
		secretKey:   secretKey,
		sessions:    make(map[string]*Session),
		legacyToken: os.Getenv("SMARTCLAW_AUTH_TOKEN"),
	}

	am.apiKey = am.loadAPIKeyFromConfig()

	go am.cleanupExpired()

	return am
}

// loadAPIKeyFromConfig reads the API key from ~/.smartclaw/config.json.
func (am *AuthManager) loadAPIKeyFromConfig() string {
	// If SMARTCLAW_AUTH_TOKEN is set, use it as the API key for login
	if am.legacyToken != "" {
		return am.legacyToken
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	configPath := filepath.Join(homeDir, ".smartclaw", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var cfg struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}

	return cfg.APIKey
}

// GenerateToken creates a new HMAC-SHA256 signed session token.
// The token format is: base64(userID:timestamp:signature)
func (am *AuthManager) GenerateToken(userID string) (string, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	timestamp := now.Unix()

	// Compute HMAC-SHA256 signature over "userID:timestamp"
	message := fmt.Sprintf("%s:%d", userID, timestamp)
	mac := hmac.New(sha256.New, am.secretKey)
	mac.Write([]byte(message))
	signature := mac.Sum(nil)

	// Token payload: userID:timestamp:base64(signature)
	sigEncoded := base64.RawURLEncoding.EncodeToString(signature)
	payload := fmt.Sprintf("%s:%d:%s", userID, timestamp, sigEncoded)

	// Full token: base64(payload)
	token := base64.RawURLEncoding.EncodeToString([]byte(payload))

	session := &Session{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(sessionDuration),
		UserID:    userID,
	}

	am.sessions[token] = session

	return token, nil
}

// ValidateToken parses and verifies a session token.
// It checks the HMAC signature and session expiry.
func (am *AuthManager) ValidateToken(token string) (*Session, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check if it's a direct session token
	if session, ok := am.sessions[token]; ok {
		if time.Now().After(session.ExpiresAt) {
			delete(am.sessions, token)
			return nil, errors.New("session expired")
		}
		return session, nil
	}

	// Try to validate as HMAC-signed token (may have been created by another instance)
	session, err := am.verifyHMACToken(token)
	if err != nil {
		return nil, err
	}

	// Cache the verified session
	am.sessions[token] = session

	return session, nil
}

// verifyHMACToken verifies the HMAC signature of a token without requiring it in the session map.
func (am *AuthManager) verifyHMACToken(token string) (*Session, error) {
	payload, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, errors.New("invalid token encoding")
	}

	parts := strings.SplitN(string(payload), ":", 3)
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	userID := parts[0]
	timestampStr := parts[1]
	signatureEncoded := parts[2]

	var timestamp int64
	if _, err := fmt.Sscanf(timestampStr, "%d", &timestamp); err != nil {
		return nil, errors.New("invalid timestamp in token")
	}

	// Verify signature
	message := fmt.Sprintf("%s:%s", userID, timestampStr)
	mac := hmac.New(sha256.New, am.secretKey)
	mac.Write([]byte(message))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(signatureEncoded)
	if err != nil {
		return nil, errors.New("invalid signature encoding")
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return nil, errors.New("invalid token signature")
	}

	// Check expiry
	createdAt := time.Unix(timestamp, 0)
	expiresAt := createdAt.Add(sessionDuration)
	if time.Now().After(expiresAt) {
		return nil, errors.New("token expired")
	}

	return &Session{
		Token:     token,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
		UserID:    userID,
	}, nil
}

// Login validates the API key and returns a session token.
func (am *AuthManager) Login(apiKey string) (string, error) {
	// If no API key is configured, allow any key
	if am.apiKey == "" {
		return am.GenerateToken("default")
	}

	// Validate against configured API key
	if apiKey != am.apiKey {
		return "", errors.New("invalid API key")
	}

	return am.GenerateToken("default")
}

// ValidateLegacyToken checks if a token matches the SMARTCLAW_AUTH_TOKEN env var.
// This provides backward compatibility with the old direct token comparison.
func (am *AuthManager) ValidateLegacyToken(token string) bool {
	if am.legacyToken == "" {
		return false
	}
	return token == am.legacyToken
}

// IsAuthRequired returns true if authentication is required.
func (am *AuthManager) IsAuthRequired() bool {
	return am.apiKey != "" || am.legacyToken != ""
}

// cleanupExpired removes expired sessions periodically.
func (am *AuthManager) cleanupExpired() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		am.mu.Lock()
		now := time.Now()
		for token, session := range am.sessions {
			if now.After(session.ExpiresAt) {
				delete(am.sessions, token)
			}
		}
		am.mu.Unlock()
	}
}

// SetAPIKey sets the API key used for login validation.
func (am *AuthManager) SetAPIKey(key string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.apiKey = key
}
