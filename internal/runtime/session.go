package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Session struct {
	ID        string                 `json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Messages  []Message              `json:"messages"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type SessionManager struct {
	basePath string
}

func NewSessionManager() (*SessionManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	basePath := filepath.Join(home, ".smartclaw", "sessions")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	return &SessionManager{basePath: basePath}, nil
}

func (sm *SessionManager) NewSession() *Session {
	return &Session{
		ID:        generateSessionID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  make([]Message, 0),
		Metadata:  make(map[string]any),
	}
}

func (sm *SessionManager) Save(session *Session) error {
	session.UpdatedAt = time.Now()

	path := filepath.Join(sm.basePath, session.ID+".jsonl")

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, msg := range session.Messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if _, err := writer.WriteString(string(data) + "\n"); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func (sm *SessionManager) Load(id string) (*Session, error) {
	path := filepath.Join(sm.basePath, id+".jsonl")

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	session := &Session{
		ID:       id,
		Messages: make([]Message, 0),
		Metadata: make(map[string]any),
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	session.CreatedAt = stat.ModTime()
	session.UpdatedAt = stat.ModTime()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var msg Message
		if err := json.Unmarshal([]byte(scanner.Text()), &msg); err != nil {
			continue
		}
		session.Messages = append(session.Messages, msg)
	}

	return session, scanner.Err()
}

func (sm *SessionManager) List() ([]*Session, error) {
	entries, err := os.ReadDir(sm.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, err
	}

	sessions := make([]*Session, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		info, err := entry.Info()
		if err != nil {
			continue
		}
		session := &Session{
			ID:        id,
			UpdatedAt: info.ModTime(),
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (sm *SessionManager) Delete(id string) error {
	path := filepath.Join(sm.basePath, id+".jsonl")
	return os.Remove(path)
}

func (sm *SessionManager) GetPath(id string) string {
	return filepath.Join(sm.basePath, id+".jsonl")
}

func generateSessionID() string {
	return fmt.Sprintf("ses_%d", time.Now().UnixNano())
}

func (s *Session) AddMessage(msg Message) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

func (s *Session) Clear() {
	s.Messages = make([]Message, 0)
	s.UpdatedAt = time.Now()
}

func (s *Session) MessageCount() int {
	return len(s.Messages)
}
