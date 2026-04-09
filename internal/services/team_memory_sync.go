package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Secret struct {
	Type    string
	Value   string
	Line    int
	Context string
}

type MemoryVisibility int

const (
	VisibilityPrivate MemoryVisibility = iota
	VisibilityTeam
	VisibilityPublic
)

type MemoryType int

const (
	MemoryTypeCode MemoryType = iota
	MemoryTypeConversation
	MemoryTypeDecision
	MemoryTypePattern
	MemoryTypePreference
)

type Memory struct {
	ID          string                 `json:"id"`
	TeamID      string                 `json:"team_id"`
	UserID      string                 `json:"user_id"`
	Type        MemoryType             `json:"type"`
	Visibility  MemoryVisibility       `json:"visibility"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Tags        []string               `json:"tags,omitempty"`
	ProjectID   string                 `json:"project_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	AccessCount int                    `json:"access_count"`
	Version     int                    `json:"version"`
}

type TeamMember struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type Team struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Members     []TeamMember `json:"members"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Settings    TeamSettings `json:"settings"`
}

type TeamSettings struct {
	AutoSync         bool `json:"auto_sync"`
	SyncInterval     int  `json:"sync_interval"`
	MaxMemories      int  `json:"max_memories"`
	EnableEncryption bool `json:"enable_encryption"`
	AllowPublicShare bool `json:"allow_public_share"`
}

type TeamMemorySync struct {
	teamID       string
	memories     []*Memory
	team         *Team
	apiEndpoint  string
	apiKey       string
	encryptKey   []byte
	storagePath  string
	mu           sync.RWMutex
	httpClient   *http.Client
	lastSyncTime time.Time
	syncEnabled  bool
}

func NewTeamMemorySync(teamID string) *TeamMemorySync {
	home, _ := os.UserHomeDir()
	storagePath := filepath.Join(home, ".smartclaw", "teams", teamID)
	os.MkdirAll(storagePath, 0755)

	tms := &TeamMemorySync{
		teamID:      teamID,
		memories:    make([]*Memory, 0),
		storagePath: storagePath,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		syncEnabled: false,
	}

	tms.loadLocal()
	return tms
}

func (tms *TeamMemorySync) loadLocal() error {
	data, err := ioutil.ReadFile(filepath.Join(tms.storagePath, "memories.json"))
	if err != nil {
		return nil
	}

	return json.Unmarshal(data, &tms.memories)
}

func (tms *TeamMemorySync) saveLocal() error {
	data, err := json.MarshalIndent(tms.memories, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(tms.storagePath, "memories.json"), data, 0644)
}

func (tms *TeamMemorySync) Configure(apiEndpoint, apiKey string, encryptKey []byte) {
	tms.mu.Lock()
	defer tms.mu.Unlock()

	tms.apiEndpoint = apiEndpoint
	tms.apiKey = apiKey
	tms.encryptKey = encryptKey
	tms.syncEnabled = apiEndpoint != ""
}

func (tms *TeamMemorySync) ShareMemory(ctx context.Context, memory *Memory) error {
	tms.mu.Lock()
	defer tms.mu.Unlock()

	memory.TeamID = tms.teamID
	memory.ID = fmt.Sprintf("mem_%d", time.Now().UnixNano())
	memory.CreatedAt = time.Now()
	memory.UpdatedAt = time.Now()
	memory.Version = 1

	if tms.encryptKey != nil && tms.team.Settings.EnableEncryption {
		encrypted, err := tms.encrypt(memory.Content)
		if err != nil {
			return err
		}
		memory.Content = encrypted
		memory.Metadata["encrypted"] = true
	}

	tms.memories = append(tms.memories, memory)
	tms.saveLocal()

	if tms.syncEnabled {
		go tms.syncMemory(ctx, memory)
	}

	return nil
}

func (tms *TeamMemorySync) GetTeamMemories(ctx context.Context) ([]*Memory, error) {
	tms.mu.RLock()
	defer tms.mu.RUnlock()

	result := make([]*Memory, len(tms.memories))
	copy(result, tms.memories)
	return result, nil
}

func (tms *TeamMemorySync) GetMemoriesByType(ctx context.Context, memType MemoryType) ([]*Memory, error) {
	tms.mu.RLock()
	defer tms.mu.RUnlock()

	var result []*Memory
	for _, m := range tms.memories {
		if m.Type == memType {
			result = append(result, m)
		}
	}
	return result, nil
}

func (tms *TeamMemorySync) GetMemoriesByTag(ctx context.Context, tag string) ([]*Memory, error) {
	tms.mu.RLock()
	defer tms.mu.RUnlock()

	var result []*Memory
	for _, m := range tms.memories {
		for _, t := range m.Tags {
			if t == tag {
				result = append(result, m)
				break
			}
		}
	}
	return result, nil
}

func (tms *TeamMemorySync) SearchMemories(ctx context.Context, query string) ([]*Memory, error) {
	tms.mu.RLock()
	defer tms.mu.RUnlock()

	var result []*Memory
	query = query // strings.ToLower(query)

	for _, m := range tms.memories {
		if strings.Contains(m.Title, query) || strings.Contains(m.Content, query) {
			result = append(result, m)
			continue
		}

		for _, tag := range m.Tags {
			if strings.Contains(tag, query) {
				result = append(result, m)
				break
			}
		}
	}

	return result, nil
}

func (tms *TeamMemorySync) UpdateMemory(ctx context.Context, memoryID string, updates map[string]interface{}) error {
	tms.mu.Lock()
	defer tms.mu.Unlock()

	var target *Memory
	for _, m := range tms.memories {
		if m.ID == memoryID {
			target = m
			break
		}
	}

	if target == nil {
		return fmt.Errorf("memory not found: %s", memoryID)
	}

	if content, ok := updates["content"].(string); ok {
		if tms.encryptKey != nil && tms.team.Settings.EnableEncryption {
			encrypted, err := tms.encrypt(content)
			if err != nil {
				return err
			}
			target.Content = encrypted
		} else {
			target.Content = content
		}
	}

	if title, ok := updates["title"].(string); ok {
		target.Title = title
	}
	if tags, ok := updates["tags"].([]string); ok {
		target.Tags = tags
	}
	if visibility, ok := updates["visibility"].(MemoryVisibility); ok {
		target.Visibility = visibility
	}

	target.UpdatedAt = time.Now()
	target.Version++

	tms.saveLocal()

	if tms.syncEnabled {
		go tms.syncMemory(ctx, target)
	}

	return nil
}

func (tms *TeamMemorySync) DeleteMemory(ctx context.Context, memoryID string) error {
	tms.mu.Lock()
	defer tms.mu.Unlock()

	for i, m := range tms.memories {
		if m.ID == memoryID {
			tms.memories = append(tms.memories[:i], tms.memories[i+1:]...)
			tms.saveLocal()

			if tms.syncEnabled {
				go tms.deleteRemoteMemory(ctx, memoryID)
			}

			return nil
		}
	}

	return fmt.Errorf("memory not found: %s", memoryID)
}

func (tms *TeamMemorySync) Sync(ctx context.Context) error {
	if !tms.syncEnabled {
		return nil
	}

	remoteMemories, err := tms.fetchRemoteMemories(ctx)
	if err != nil {
		return err
	}

	tms.mu.Lock()
	defer tms.mu.Unlock()

	localMap := make(map[string]*Memory)
	for _, m := range tms.memories {
		localMap[m.ID] = m
	}

	for _, remote := range remoteMemories {
		local, exists := localMap[remote.ID]
		if !exists {
			tms.memories = append(tms.memories, remote)
		} else if remote.Version > local.Version {
			*tms.memories[len(tms.memories)-1] = *remote
		}
	}

	tms.saveLocal()
	tms.lastSyncTime = time.Now()

	return nil
}

func (tms *TeamMemorySync) syncMemory(ctx context.Context, memory *Memory) error {
	if !tms.syncEnabled || tms.apiEndpoint == "" {
		return nil
	}

	data, _ := json.Marshal(memory)
	url := fmt.Sprintf("%s/teams/%s/memories", tms.apiEndpoint, tms.teamID)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(data)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tms.apiKey)

	resp, err := tms.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("sync failed with status %d", resp.StatusCode)
	}

	return nil
}

func (tms *TeamMemorySync) fetchRemoteMemories(ctx context.Context) ([]*Memory, error) {
	if !tms.syncEnabled || tms.apiEndpoint == "" {
		return nil, nil
	}

	url := fmt.Sprintf("%s/teams/%s/memories", tms.apiEndpoint, tms.teamID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+tms.apiKey)

	resp, err := tms.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var memories []*Memory
	if err := json.NewDecoder(resp.Body).Decode(&memories); err != nil {
		return nil, err
	}

	return memories, nil
}

func (tms *TeamMemorySync) deleteRemoteMemory(ctx context.Context, memoryID string) error {
	if !tms.syncEnabled || tms.apiEndpoint == "" {
		return nil
	}

	url := fmt.Sprintf("%s/teams/%s/memories/%s", tms.apiEndpoint, tms.teamID, memoryID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+tms.apiKey)

	resp, err := tms.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("delete failed with status %d", resp.StatusCode)
	}

	return nil
}

func (tms *TeamMemorySync) encrypt(plaintext string) (string, error) {
	if len(tms.encryptKey) == 0 {
		return "", fmt.Errorf("encryption key not set")
	}

	block, err := aes.NewCipher(tms.encryptKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return string(ciphertext), nil
}

func (tms *TeamMemorySync) decrypt(ciphertext string) (string, error) {
	if len(tms.encryptKey) == 0 {
		return "", fmt.Errorf("encryption key not set")
	}

	block, err := aes.NewCipher(tms.encryptKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	data := []byte(ciphertext)
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, cipherData := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func (tms *TeamMemorySync) ScanForSecrets(content string) []Secret {
	secrets := []Secret{}

	patterns := []struct {
		name  string
		regex string
	}{
		{"api_key", `(?i)api[_-]?key\s*[=:]\s*['"][^'"]+['"]`},
		{"password", `(?i)password\s*[=:]\s*['"][^'"]+['"]`},
		{"token", `(?i)token\s*[=:]\s*['"][^'"]+['"]`},
		{"secret", `(?i)secret\s*[=:]\s*['"][^'"]+['"]`},
	}

	for _, p := range patterns {
		if strings.Contains(content, p.name) {
			secrets = append(secrets, Secret{
				Type:    p.name,
				Context: p.regex,
			})
		}
	}

	return secrets
}

func (tms *TeamMemorySync) GetLastSyncTime() time.Time {
	tms.mu.RLock()
	defer tms.mu.RUnlock()
	return tms.lastSyncTime
}

func (tms *TeamMemorySync) SetTeam(team *Team) {
	tms.mu.Lock()
	defer tms.mu.Unlock()
	tms.team = team
}

func (tms *TeamMemorySync) GetTeam() *Team {
	tms.mu.RLock()
	defer tms.mu.RUnlock()
	return tms.team
}

func (tms *TeamMemorySync) GetStats() map[string]interface{} {
	tms.mu.RLock()
	defer tms.mu.RUnlock()

	stats := map[string]interface{}{
		"total_memories": len(tms.memories),
		"by_type":        make(map[MemoryType]int),
		"by_visibility":  make(map[MemoryVisibility]int),
		"sync_enabled":   tms.syncEnabled,
		"last_sync":      tms.lastSyncTime,
	}

	byType := stats["by_type"].(map[MemoryType]int)
	byVisibility := stats["by_visibility"].(map[MemoryVisibility]int)

	for _, m := range tms.memories {
		byType[m.Type]++
		byVisibility[m.Visibility]++
	}

	return stats
}
