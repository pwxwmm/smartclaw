package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Tokens    int       `json:"tokens"`
	Keep      bool      `json:"keep"`
}

type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
	Model     string    `json:"model"`
	Tokens    int       `json:"tokens"`
	Cost      float64   `json:"cost"`
	Title     string    `json:"title"`
}

type Manager struct {
	sessionsDir string
}

func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".smartclaw", "sessions")

	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &Manager{
		sessionsDir: sessionsDir,
	}, nil
}

func (m *Manager) NewSession(model string) *Session {
	now := time.Now()
	return &Session{
		ID:        generateSessionID(),
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  make([]Message, 0),
		Model:     model,
		Tokens:    0,
		Cost:      0,
		Title:     "",
	}
}

func (m *Manager) Save(session *Session) error {
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = time.Now()
	}

	filename := fmt.Sprintf("%s.json", session.ID)
	path := filepath.Join(m.sessionsDir, filename)

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

func (m *Manager) Load(sessionID string) (*Session, error) {
	filename := fmt.Sprintf("%s.json", sessionID)
	path := filepath.Join(m.sessionsDir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

type SessionInfo struct {
	ID           string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	MessageCount int
	Model        string
	Title        string
}

func (m *Manager) List() ([]SessionInfo, error) {
	files, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []SessionInfo

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		sessionID := strings.TrimSuffix(file.Name(), ".json")

		session, err := m.Load(sessionID)
		if err != nil {
			continue
		}

		sessions = append(sessions, SessionInfo{
			ID:           session.ID,
			CreatedAt:    session.CreatedAt,
			UpdatedAt:    session.UpdatedAt,
			MessageCount: len(session.Messages),
			Model:        session.Model,
			Title:        session.Title,
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

func (m *Manager) Delete(sessionID string) error {
	filename := fmt.Sprintf("%s.json", sessionID)
	path := filepath.Join(m.sessionsDir, filename)

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	return nil
}

func (m *Manager) Export(sessionID string, format string) (string, error) {
	session, err := m.Load(sessionID)
	if err != nil {
		return "", err
	}

	switch format {
	case "markdown", "md":
		return exportMarkdown(session), nil
	case "json":
		data, err := json.MarshalIndent(session, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	default:
		return "", fmt.Errorf("unsupported export format: %s", format)
	}
}

func exportMarkdown(session *Session) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Session: %s\n\n", session.Title))
	sb.WriteString(fmt.Sprintf("- **Created**: %s\n", session.CreatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("- **Model**: %s\n", session.Model))
	sb.WriteString(fmt.Sprintf("- **Tokens**: %d\n", session.Tokens))
	sb.WriteString(fmt.Sprintf("- **Cost**: $%.4f\n\n", session.Cost))
	sb.WriteString("---\n\n")

	for _, msg := range session.Messages {
		if msg.Role == "user" {
			sb.WriteString(fmt.Sprintf("## You (%s)\n\n", msg.Timestamp.Format("15:04:05")))
		} else {
			sb.WriteString(fmt.Sprintf("## SmartClaw (%s)\n\n", msg.Timestamp.Format("15:04:05")))
		}
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

func generateSessionID() string {
	timestamp := time.Now().Format("20060102_150405")
	uniqueID := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%s", timestamp, uniqueID)
}

func (m *Manager) GetSessionsDir() string {
	return m.sessionsDir
}

func (s *Session) AddMessage(role, content string) {
	s.Messages = append(s.Messages, Message{
		ID:        generateMessageID(),
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		Tokens:    0,
		Keep:      false,
	})
	s.UpdatedAt = time.Now()

	if s.Title == "" && role == "user" && len(s.Messages) == 1 {
		title := content
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		s.Title = title
	}
}

func generateMessageID() string {
	return uuid.New().String()[:8]
}
