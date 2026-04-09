package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	TokenType    string
}

type Profile struct {
	ID    string
	Email string
	Name  string
}

type OAuthService struct {
	clientID      string
	redirectURI   string
	authURL       string
	tokenURL      string
	profileURL    string
	port          int
	tokenStore    string
	codeVerifier  string
	codeChallenge string
	httpClient    *http.Client
}

func NewOAuthService(clientID, redirectURI string) *OAuthService {
	home, _ := os.UserHomeDir()
	tokenStore := filepath.Join(home, ".smartclaw", "oauth_token.json")

	return &OAuthService{
		clientID:    clientID,
		redirectURI: redirectURI,
		authURL:     "https://claude.ai/oauth/authorize",
		tokenURL:    "https://claude.ai/oauth/token",
		profileURL:  "https://claude.ai/api/profile",
		port:        8765,
		tokenStore:  tokenStore,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *OAuthService) generatePKCE() error {
	verifier := make([]byte, 32)
	if _, err := rand.Read(verifier); err != nil {
		return err
	}

	s.codeVerifier = base64URLEncode(verifier)

	hash := sha256.Sum256([]byte(s.codeVerifier))
	s.codeChallenge = base64URLEncode(hash[:])

	return nil
}

func base64URLEncode(data []byte) string {
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(data)
}

func (s *OAuthService) StartAuthFlow(ctx context.Context) (string, error) {
	if err := s.generatePKCE(); err != nil {
		return "", err
	}

	params := url.Values{
		"client_id":             {s.clientID},
		"redirect_uri":          {s.redirectURI},
		"response_type":         {"code"},
		"code_challenge":        {s.codeChallenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid profile email"},
	}

	authURL := fmt.Sprintf("%s?%s", s.authURL, params.Encode())

	return authURL, nil
}

func (s *OAuthService) WaitForCallback(ctx context.Context) (*Token, error) {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback")
			http.Error(w, "No code", http.StatusBadRequest)
			return
		}

		codeChan <- code
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Authentication successful! You can close this window."))
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	defer server.Shutdown(ctx)

	select {
	case code := <-codeChan:
		return s.exchangeCode(ctx, code)
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *OAuthService) exchangeCode(ctx context.Context, code string) (*Token, error) {
	data := url.Values{
		"client_id":     {s.clientID},
		"redirect_uri":  {s.redirectURI},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {s.codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	token := &Token{
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second),
		TokenType:    tokenResponse.TokenType,
	}

	return token, nil
}

func (s *OAuthService) RefreshToken(ctx context.Context, token *Token) (*Token, error) {
	if token.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	data := url.Values{
		"client_id":     {s.clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {token.RefreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	newToken := &Token{
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second),
		TokenType:    tokenResponse.TokenType,
	}

	if newToken.RefreshToken == "" {
		newToken.RefreshToken = token.RefreshToken
	}

	return newToken, nil
}

func (s *OAuthService) GetProfile(ctx context.Context, token *Token) (*Profile, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.profileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("%s %s", token.TokenType, token.AccessToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("profile request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("profile request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var profileResponse struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}

	if err := json.Unmarshal(body, &profileResponse); err != nil {
		return nil, fmt.Errorf("failed to parse profile response: %w", err)
	}

	return &Profile{
		ID:    profileResponse.ID,
		Email: profileResponse.Email,
		Name:  profileResponse.Name,
	}, nil
}

func (s *OAuthService) SaveToken(token *Token) error {
	if err := os.MkdirAll(filepath.Dir(s.tokenStore), 0755); err != nil {
		return err
	}

	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	return os.WriteFile(s.tokenStore, data, 0600)
}

func (s *OAuthService) LoadToken() (*Token, error) {
	data, err := os.ReadFile(s.tokenStore)
	if err != nil {
		return nil, err
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

func (s *OAuthService) ClearToken() error {
	return os.Remove(s.tokenStore)
}

func (s *OAuthService) IsTokenExpired(token *Token) bool {
	return time.Now().After(token.ExpiresAt)
}

func (s *OAuthService) IsTokenExpiringSoon(token *Token, duration time.Duration) bool {
	return time.Now().Add(duration).After(token.ExpiresAt)
}

func (s *OAuthService) EnsureValidToken(ctx context.Context, token *Token) (*Token, error) {
	if s.IsTokenExpiringSoon(token, 5*time.Minute) {
		newToken, err := s.RefreshToken(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}
		return newToken, nil
	}
	return token, nil
}

func (s *OAuthService) SetCustomEndpoints(authURL, tokenURL, profileURL string) {
	s.authURL = authURL
	s.tokenURL = tokenURL
	s.profileURL = profileURL
}
