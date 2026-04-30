package dap

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type DAPSession struct {
	ID               string
	Client           *DAPClient
	ProgramPath      string
	CreatedAt        time.Time
	ActiveBreakpoints map[int]Breakpoint
	CurrentFrame     *StackFrame
}

type SessionManager struct {
	sessions sync.Map
	mu       sync.Mutex
}

var DefaultSessionManager = &SessionManager{}

func (sm *SessionManager) CreateSession(programPath string) (*DAPSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	client, err := Launch(programPath)
	if err != nil {
		return nil, fmt.Errorf("failed to launch delve: %w", err)
	}

	if err := client.Initialize(); err != nil {
		client.Disconnect()
		return nil, fmt.Errorf("failed to initialize DAP session: %w", err)
	}

	if err := client.LaunchRequest(programPath); err != nil {
		client.Disconnect()
		return nil, fmt.Errorf("failed to launch program: %w", err)
	}

	sessionID := uuid.New().String()[:8]
	session := &DAPSession{
		ID:                sessionID,
		Client:            client,
		ProgramPath:       programPath,
		CreatedAt:         time.Now(),
		ActiveBreakpoints: make(map[int]Breakpoint),
	}

	sm.sessions.Store(sessionID, session)
	return session, nil
}

func (sm *SessionManager) GetSession(id string) (*DAPSession, bool) {
	val, ok := sm.sessions.Load(id)
	if !ok {
		return nil, false
	}
	session, ok := val.(*DAPSession)
	if !ok {
		return nil, false
	}
	return session, true
}

func (sm *SessionManager) CloseSession(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	val, ok := sm.sessions.Load(id)
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}

	session, ok := val.(*DAPSession)
	if !ok {
		return fmt.Errorf("invalid session type for %s", id)
	}

	if err := session.Client.Disconnect(); err != nil {
		return fmt.Errorf("failed to disconnect session %s: %w", id, err)
	}

	sm.sessions.Delete(id)
	return nil
}

func (sm *SessionManager) ListSessions() []DAPSession {
	var result []DAPSession
	sm.sessions.Range(func(key, value any) bool {
		if session, ok := value.(*DAPSession); ok {
			result = append(result, *session)
		}
		return true
	})
	return result
}
