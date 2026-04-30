package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/httpclient"
	"github.com/instructkr/smartclaw/internal/utils"
)

type Settings struct {
	Model        string         `json:"model"`
	MaxTokens    int            `json:"max_tokens"`
	Permission   string         `json:"permission"`
	CustomConfig map[string]any `json:"custom,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Version      int            `json:"version"`
	Checksum     string         `json:"checksum"`
}

type SettingsVersion struct {
	Version   int       `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Changes   []string  `json:"changes"`
}

type ConflictResolution int

const (
	ConflictLocalWins ConflictResolution = iota
	ConflictRemoteWins
	ConflictMerge
)

type SettingsSync struct {
	settingsPath   string
	settings       *Settings
	mu             sync.RWMutex
	syncEnabled    bool
	remoteURL      string
	authToken      string
	versionHistory []SettingsVersion
	maxVersions    int
	lastSyncTime   time.Time
	syncInterval   time.Duration
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

func NewSettingsSync() (*SettingsSync, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	settingsPath := filepath.Join(home, ".smartclaw", "settings.json")
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	s := &SettingsSync{
		settingsPath:   settingsPath,
		syncEnabled:    false,
		versionHistory: make([]SettingsVersion, 0),
		maxVersions:    10,
		syncInterval:   5 * time.Minute,
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}

	s.load()
	s.loadVersionHistory()
	return s, nil
}

func (s *SettingsSync) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.settings = &Settings{
				Model:      "claude-sonnet-4-5",
				MaxTokens:  4096,
				Permission: "ask",
				UpdatedAt:  time.Now(),
				Version:    1,
			}
			return nil
		}
		return err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}

	s.settings = &settings
	return nil
}

func (s *SettingsSync) loadVersionHistory() error {
	home, _ := os.UserHomeDir()
	historyPath := filepath.Join(home, ".smartclaw", "settings_history.json")

	data, err := os.ReadFile(historyPath)
	if err != nil {
		return nil
	}

	return json.Unmarshal(data, &s.versionHistory)
}

func (s *SettingsSync) save() error {
	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.settingsPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(s.settingsPath, data, 0644)
}

func (s *SettingsSync) saveVersionHistory() error {
	home, _ := os.UserHomeDir()
	historyPath := filepath.Join(home, ".smartclaw", "settings_history.json")

	data, err := json.MarshalIndent(s.versionHistory, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(historyPath, data, 0644)
}

func (s *SettingsSync) calculateChecksum() string {
	data, _ := json.Marshal(s.settings)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8])
}

func (s *SettingsSync) GetSettings() *Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.settings
}

func (s *SettingsSync) UpdateSettings(ctx context.Context, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.settings == nil {
		s.settings = &Settings{}
	}

	changes := make([]string, 0)

	if model, ok := updates["model"].(string); ok {
		changes = append(changes, fmt.Sprintf("model: %s -> %s", s.settings.Model, model))
		s.settings.Model = model
	}
	if maxTokens, ok := updates["max_tokens"].(int); ok {
		changes = append(changes, fmt.Sprintf("max_tokens: %d -> %d", s.settings.MaxTokens, maxTokens))
		s.settings.MaxTokens = maxTokens
	}
	if permission, ok := updates["permission"].(string); ok {
		changes = append(changes, fmt.Sprintf("permission: %s -> %s", s.settings.Permission, permission))
		s.settings.Permission = permission
	}
	if custom, ok := updates["custom"].(map[string]any); ok {
		if s.settings.CustomConfig == nil {
			s.settings.CustomConfig = make(map[string]any)
		}
		for k, v := range custom {
			changes = append(changes, fmt.Sprintf("custom.%s: updated", k))
			s.settings.CustomConfig[k] = v
		}
	}

	s.settings.UpdatedAt = time.Now()
	s.settings.Version++
	s.settings.Checksum = s.calculateChecksum()

	s.addToHistory(s.settings.Version, changes)

	if err := s.save(); err != nil {
		return err
	}

	if s.syncEnabled && s.remoteURL != "" {
		utils.Go(func() { s.Sync(s.shutdownCtx) })
	}

	return nil
}

func (s *SettingsSync) addToHistory(version int, changes []string) {
	s.versionHistory = append(s.versionHistory, SettingsVersion{
		Version:   version,
		Timestamp: time.Now(),
		Changes:   changes,
	})

	if len(s.versionHistory) > s.maxVersions {
		s.versionHistory = s.versionHistory[1:]
	}
}

func (s *SettingsSync) Sync(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.syncEnabled || s.remoteURL == "" {
		return nil
	}

	remoteSettings, err := s.fetchRemote(ctx)
	if err != nil {
		return err
	}

	if remoteSettings.Version > s.settings.Version {
		return s.mergeRemote(remoteSettings, ConflictRemoteWins)
	} else if remoteSettings.Version < s.settings.Version {
		return s.pushRemote(ctx)
	}

	s.lastSyncTime = time.Now()
	return nil
}

func (s *SettingsSync) fetchRemote(ctx context.Context) (*Settings, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.remoteURL, nil)
	if err != nil {
		return nil, err
	}

	if s.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.authToken)
	}

	resp, err := httpclient.NewClient(30 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var remoteSettings Settings
	if err := json.NewDecoder(resp.Body).Decode(&remoteSettings); err != nil {
		return nil, err
	}

	return &remoteSettings, nil
}

func (s *SettingsSync) pushRemote(ctx context.Context) error {
	data, err := json.Marshal(s.settings)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", s.remoteURL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	if s.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.authToken)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.NewClient(30 * time.Second).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to push settings: status %d", resp.StatusCode)
	}

	s.lastSyncTime = time.Now()
	return nil
}

func (s *SettingsSync) mergeRemote(remote *Settings, resolution ConflictResolution) error {
	switch resolution {
	case ConflictRemoteWins:
		s.settings = remote
	case ConflictLocalWins:
	case ConflictMerge:
		if remote.Model != s.settings.Model {
			s.settings.Model = remote.Model
		}
		if remote.MaxTokens > s.settings.MaxTokens {
			s.settings.MaxTokens = remote.MaxTokens
		}
		for k, v := range remote.CustomConfig {
			if s.settings.CustomConfig == nil {
				s.settings.CustomConfig = make(map[string]any)
			}
			if _, exists := s.settings.CustomConfig[k]; !exists {
				s.settings.CustomConfig[k] = v
			}
		}
	}

	s.settings.Version = max(s.settings.Version, remote.Version) + 1
	s.settings.UpdatedAt = time.Now()
	s.settings.Checksum = s.calculateChecksum()

	return s.save()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *SettingsSync) ConfigureRemote(url, authToken string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remoteURL = url
	s.authToken = authToken
	s.syncEnabled = url != ""
}

func (s *SettingsSync) SetSyncEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncEnabled = enabled
}

func (s *SettingsSync) GetVersionHistory() []SettingsVersion {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]SettingsVersion{}, s.versionHistory...)
}

func (s *SettingsSync) Rollback(version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var target *SettingsVersion
	for _, v := range s.versionHistory {
		if v.Version == version {
			target = &v
			break
		}
	}

	if target == nil {
		return fmt.Errorf("version %d not found in history", version)
	}

	s.settings.Version = version
	s.settings.UpdatedAt = time.Now()
	s.settings.Checksum = s.calculateChecksum()

	return s.save()
}

func (s *SettingsSync) GetLastSyncTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSyncTime
}

func (s *SettingsSync) StartAutoSync(ctx context.Context) {
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdownCtx.Done():
			return
		case <-ticker.C:
			s.Sync(ctx)
		}
	}
}

func (s *SettingsSync) Stop() {
	if s.shutdownCancel != nil {
		s.shutdownCancel()
	}
}
