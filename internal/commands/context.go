package commands

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/voice"
)

type Session struct {
	ID           string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	MessageCount int
	Model        string
}

type CommandContext struct {
	Session      *Session
	WorkDir      string
	Model        string
	APIKey       string
	Permission   string
	LogLevel     string
	StartTime    time.Time
	TokenCount   int64
	InputTokens  int64
	OutputTokens int64
	VoiceManager *voice.VoiceManager
	mu           sync.RWMutex
}

func NewCommandContext() *CommandContext {
	workDir, _ := os.Getwd()

	// Initialize voice manager with defaults
	voiceConfig := voice.VoiceConfig{
		Mode:             voice.VoiceModeDisabled,
		Language:         "en",
		Model:            "whisper-1",
		SampleRate:       16000,
		RecordingTimeout: 30,
		SilenceThreshold: 3,
	}
	vm := voice.NewVoiceManager(voiceConfig)

	return &CommandContext{
		WorkDir:      workDir,
		Model:        "claude-sonnet-4-5",
		Permission:   "ask",
		LogLevel:     "info",
		StartTime:    time.Now(),
		VoiceManager: vm,
	}
}

func (c *CommandContext) SetModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Model = model
}

func (c *CommandContext) GetModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Model
}

func (c *CommandContext) SetAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.APIKey = key
}

func (c *CommandContext) GetAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.APIKey
}

func (c *CommandContext) AddTokens(input, output int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.InputTokens += input
	c.OutputTokens += output
	c.TokenCount = c.InputTokens + c.OutputTokens
}

func (c *CommandContext) GetTokenStats() (int64, int64, int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.InputTokens, c.OutputTokens, c.TokenCount
}

func (c *CommandContext) NewSession() *Session {
	session := &Session{
		ID:        generateSessionID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Model:     c.Model,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Session = session
	return session
}

func (c *CommandContext) GetSession() *Session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Session
}

func generateSessionID() string {
	return fmt.Sprintf("ses_%d", time.Now().UnixNano())
}
