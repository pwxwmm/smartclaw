package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/instructkr/smartclaw/internal/config"
)

type Config struct {
	APIKey       string `json:"api_key,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
	Model        string `json:"model,omitempty"`
	BaseURL      string `json:"base_url,omitempty"`
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".smartclaw", "config.json"), nil
}

func LoadConfig() (*Config, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	cfg, err := config.LoadJSON[Config](path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}

	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return config.SaveJSON(path, cfg)
}

func GetAPIKey() string {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("ANTHROPIC_AUTH_TOKEN"); key != "" {
		return key
	}

	config, err := LoadConfig()
	if err != nil {
		return ""
	}

	return config.APIKey
}

func GetBaseURL() string {
	if url := os.Getenv("ANTHROPIC_BASE_URL"); url != "" {
		return url
	}

	config, err := LoadConfig()
	if err != nil {
		return ""
	}

	return config.BaseURL
}

func SetAPIKey(apiKey string) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	var existing map[string]any
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &existing)
	}
	if existing == nil {
		existing = make(map[string]any)
	}

	existing["api_key"] = apiKey

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
