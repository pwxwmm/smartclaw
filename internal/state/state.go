package state

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/types"
)

type AppState struct {
	mu            sync.RWMutex
	Config        *types.Config
	CurrentModel  string
	Sessions      map[string]*types.Session
	ActiveSession *types.Session
	Cache         map[string]*types.CacheEntry
}

func NewAppState() *AppState {
	return &AppState{
		Config:       &types.Config{},
		CurrentModel: "claude-sonnet-4-5",
		Sessions:     make(map[string]*types.Session),
		Cache:        make(map[string]*types.CacheEntry),
	}
}

func (s *AppState) SetConfig(config *types.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Config = config
}

func (s *AppState) GetConfig() *types.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Config
}

func (s *AppState) SetModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentModel = model
}

func (s *AppState) GetModel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentModel
}

func (s *AppState) CreateSession(id string) *types.Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := types.NewSession(id)
	session.Model = s.CurrentModel
	s.Sessions[id] = session
	s.ActiveSession = session

	return session
}

func (s *AppState) GetSession(id string) *types.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Sessions[id]
}

func (s *AppState) GetActiveSession() *types.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ActiveSession
}

func (s *AppState) SetActiveSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.Sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	s.ActiveSession = session
	return nil
}

func (s *AppState) AddMessage(sessionID string, msg types.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.Sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.AddMessage(msg)
	return nil
}

func (s *AppState) SaveSession(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.ActiveSession, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *AppState) LoadSession(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var session types.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Sessions[session.ID] = &session
	return nil
}

func (s *AppState) ListSessions() []*types.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*types.Session, 0, len(s.Sessions))
	for _, session := range s.Sessions {
		result = append(result, session)
	}

	return result
}

func (s *AppState) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Sessions[id]; !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	delete(s.Sessions, id)

	if s.ActiveSession != nil && s.ActiveSession.ID == id {
		s.ActiveSession = nil
	}

	return nil
}

func (s *AppState) SetCache(key string, value interface{}, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := &types.CacheEntry{
		Key:       key,
		Value:     value,
		CreatedAt: time.Now(),
	}

	if ttl > 0 {
		expiresAt := time.Now().Add(ttl)
		entry.ExpiresAt = &expiresAt
	}

	s.Cache[key] = entry
}

func (s *AppState) GetCache(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.Cache[key]
	if !ok {
		return nil, false
	}

	if entry.IsExpired() {
		return nil, false
	}

	return entry.Value, true
}

func (s *AppState) DeleteCache(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Cache, key)
}

func (s *AppState) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Cache = make(map[string]*types.CacheEntry)
}

type Context struct {
	mu     sync.RWMutex
	Values map[string]interface{}
	Stack  []map[string]interface{}
}

func NewContext() *Context {
	return &Context{
		Values: make(map[string]interface{}),
		Stack:  make([]map[string]interface{}, 0),
	}
}

func (c *Context) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Values[key] = value
}

func (c *Context) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.Values[key]
	return val, ok
}

func (c *Context) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Values, key)
}

func (c *Context) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.Values))
	for k := range c.Values {
		keys = append(keys, k)
	}
	return keys
}

func (c *Context) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Values = make(map[string]interface{})
}

func (c *Context) Push() {
	c.mu.Lock()
	defer c.mu.Unlock()

	snapshot := make(map[string]interface{})
	for k, v := range c.Values {
		snapshot[k] = v
	}
	c.Stack = append(c.Stack, snapshot)
}

func (c *Context) Pop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.Stack) == 0 {
		return
	}

	snapshot := c.Stack[len(c.Stack)-1]
	c.Stack = c.Stack[:len(c.Stack)-1]
	c.Values = snapshot
}

func (c *Context) Snapshot() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := make(map[string]interface{})
	for k, v := range c.Values {
		snapshot[k] = v
	}
	return snapshot
}

type StateManager struct {
	state   *AppState
	context *Context
	mu      sync.RWMutex
}

func NewStateManager() *StateManager {
	return &StateManager{
		state:   NewAppState(),
		context: NewContext(),
	}
}

func (sm *StateManager) GetState() *AppState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

func (sm *StateManager) GetContext() *Context {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.context
}

var globalStateManager *StateManager

func Init() {
	globalStateManager = NewStateManager()
}

func Get() *StateManager {
	if globalStateManager == nil {
		globalStateManager = NewStateManager()
	}
	return globalStateManager
}
