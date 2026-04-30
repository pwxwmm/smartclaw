package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type OAuthConfig struct {
	ClientID    string
	RedirectURI string
	AuthURL     string
	TokenURL    string
}

type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func DefaultOAuthConfig() *OAuthConfig {
	redirectURI := os.Getenv("SMARTCLAW_OAUTH_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://localhost:8765/callback"
	}
	return &OAuthConfig{
		ClientID:    "claude-code",
		RedirectURI: redirectURI,
		AuthURL:     "https://claude.ai/oauth/authorize",
		TokenURL:    "https://claude.ai/oauth/token",
	}
}

func GeneratePKCE() (verifier, challenge string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}

	verifier = base64URLEncode(bytes)

	hash := sha256.Sum256([]byte(verifier))
	challenge = base64URLEncode(hash[:])

	return verifier, challenge, nil
}

func base64URLEncode(data []byte) string {
	encoded := base64.URLEncoding.EncodeToString(data)
	return strings.TrimRight(encoded, "=")
}

func BuildAuthURL(config *OAuthConfig, challenge string) string {
	params := url.Values{
		"client_id":             {config.ClientID},
		"redirect_uri":          {config.RedirectURI},
		"response_type":         {"code"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid profile email"},
	}

	return fmt.Sprintf("%s?%s", config.AuthURL, params.Encode())
}

func ExchangeCode(config *OAuthConfig, code, verifier string) (*Token, error) {
	data := url.Values{
		"client_id":     {config.ClientID},
		"redirect_uri":  {config.RedirectURI},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(config.TokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

func RefreshToken(config *OAuthConfig, refreshToken string) (*Token, error) {
	data := url.Values{
		"client_id":     {config.ClientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(config.TokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token refresh failed: %s", string(body))
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

func (t *Token) IsExpired() bool {
	return t.ExpiresIn <= 0
}

func (t *Token) ExpiresAt() time.Time {
	return time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
}
