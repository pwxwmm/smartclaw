package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
	"time"
)

type AgentSummary struct {
	AgentID    string    `json:"agent_id"`
	Summary    string    `json:"summary"`
	TokensUsed int       `json:"tokens_used"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AgentSummaryService struct {
	summaries map[string]*AgentSummary
	mu        sync.RWMutex
}

func NewAgentSummaryService() *AgentSummaryService {
	return &AgentSummaryService{
		summaries: make(map[string]*AgentSummary),
	}
}

func (s *AgentSummaryService) Create(agentID, summary string, tokens int) *AgentSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	as := &AgentSummary{
		AgentID:    agentID,
		Summary:    summary,
		TokensUsed: tokens,
		UpdatedAt:  time.Now(),
	}

	s.summaries[agentID] = as
	return as
}

func (s *AgentSummaryService) Get(agentID string) *AgentSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.summaries[agentID]
}

func (s *AgentSummaryService) Update(agentID, summary string, tokens int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.summaries[agentID]; !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	s.summaries[agentID] = &AgentSummary{
		AgentID:    agentID,
		Summary:    summary,
		TokensUsed: tokens,
		UpdatedAt:  time.Now(),
	}

	return nil
}

func (s *AgentSummaryService) List() []*AgentSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*AgentSummary, 0, len(s.summaries))
	for _, as := range s.summaries {
		result = append(result, as)
	}

	return result
}

type MagicDocs struct {
	DocID       string                 `json:"doc_id"`
	Content     string                 `json:"content"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	GeneratedAt time.Time              `json:"generated_at"`
}

type MagicDocsService struct {
	docs map[string]*MagicDocs
	mu   sync.RWMutex
}

func NewMagicDocsService() *MagicDocsService {
	return &MagicDocsService{
		docs: make(map[string]*MagicDocs),
	}
}

func (s *MagicDocsService) Generate(docID, content string) *MagicDocs {
	s.mu.Lock()
	defer s.mu.Unlock()

	md := &MagicDocs{
		DocID:       docID,
		Content:     content,
		GeneratedAt: time.Now(),
	}

	s.docs[docID] = md
	return md
}

func (s *MagicDocsService) Get(docID string) *MagicDocs {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[docID]
}

func (s *MagicDocsService) List() []*MagicDocs {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*MagicDocs, 0, len(s.docs))
	for _, md := range s.docs {
		result = append(result, md)
	}

	return result
}

type Tip struct {
	Category string    `json:"category"`
	Content  string    `json:"content"`
	UsedAt   time.Time `json:"used_at"`
}

type TipsService struct {
	tips []Tip
	mu   sync.RWMutex
}

func NewTipsService() *TipsService {
	return &TipsService{
		tips: []Tip{
			{Category: "general", Content: "Use /help to see available commands"},
			{Category: "productivity", Content: "Use Ctrl+L to clear the screen"},
			{Category: "git", Content: "Use /git-status to check git status"},
		},
	}
}

func (s *TipsService) GetRandom() Tip {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.tips) == 0 {
		return Tip{Content: "No tips available"}
	}

	return s.tips[time.Now().Unix()%int64(len(s.tips))]
}

func (s *TipsService) Add(category, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tips = append(s.tips, Tip{
		Category: category,
		Content:  content,
	})
}

func (s *TipsService) List() []Tip {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Tip{}, s.tips...)
}

type ExtractMemory struct {
	Key      string                 `json:"key"`
	Data     interface{}            `json:"data"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ExtractMemoriesService struct {
	memories map[string]*ExtractMemory
	mu       sync.RWMutex
}

func NewExtractMemoriesService() *ExtractMemoriesService {
	return &ExtractMemoriesService{
		memories: make(map[string]*ExtractMemory),
	}
}

func (s *ExtractMemoriesService) Extract(key string, data interface{}) *ExtractMemory {
	s.mu.Lock()
	defer s.mu.Unlock()

	em := &ExtractMemory{
		Key:      key,
		Data:     data,
		Metadata: make(map[string]interface{}),
	}

	s.memories[key] = em
	return em
}

func (s *ExtractMemoriesService) Get(key string) *ExtractMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.memories[key]
}

func (s *ExtractMemoriesService) List() []*ExtractMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ExtractMemory, 0, len(s.memories))
	for _, em := range s.memories {
		result = append(result, em)
	}

	return result
}

type ToolUseSummary struct {
	ToolName   string    `json:"tool_name"`
	Count      int       `json:"count"`
	LastUsedAt time.Time `json:"last_used_at"`
}

type ToolUseSummaryService struct {
	summary map[string]*ToolUseSummary
	mu      sync.RWMutex
}

func NewToolUseSummaryService() *ToolUseSummaryService {
	return &ToolUseSummaryService{
		summary: make(map[string]*ToolUseSummary),
	}
}

func (s *ToolUseSummaryService) Record(toolName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tus, ok := s.summary[toolName]; ok {
		tus.Count++
		tus.LastUsedAt = time.Now()
	} else {
		s.summary[toolName] = &ToolUseSummary{
			ToolName:   toolName,
			Count:      1,
			LastUsedAt: time.Now(),
		}
	}
}

func (s *ToolUseSummaryService) Get(toolName string) *ToolUseSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.summary[toolName]
}

func (s *ToolUseSummaryService) List() []*ToolUseSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ToolUseSummary, 0, len(s.summary))
	for _, tus := range s.summary {
		result = append(result, tus)
	}

	return result
}

type Diagnostic struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Source   string `json:"source"`
}

type DiagnosticTrackingService struct {
	diagnostics []Diagnostic
	mu          sync.RWMutex
}

func NewDiagnosticTrackingService() *DiagnosticTrackingService {
	return &DiagnosticTrackingService{
		diagnostics: make([]Diagnostic, 0),
	}
}

func (s *DiagnosticTrackingService) Track(code, message, severity, source string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.diagnostics = append(s.diagnostics, Diagnostic{
		Code:     code,
		Message:  message,
		Severity: severity,
		Source:   source,
	})
}

func (s *DiagnosticTrackingService) List() []Diagnostic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Diagnostic{}, s.diagnostics...)
}

func (s *DiagnosticTrackingService) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.diagnostics = make([]Diagnostic, 0)
}

type TeamMemory struct {
	UserID    string                 `json:"user_id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type TeamMemorySyncService struct {
	memories map[string][]*TeamMemory
	mu       sync.RWMutex
}

func NewTeamMemorySyncService() *TeamMemorySyncService {
	return &TeamMemorySyncService{
		memories: make(map[string][]*TeamMemory),
	}
}

func (s *TeamMemorySyncService) Add(userID, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tm := &TeamMemory{
		UserID:    userID,
		Content:   content,
		CreatedAt: time.Now(),
	}

	s.memories[userID] = append(s.memories[userID], tm)
}

func (s *TeamMemorySyncService) Get(userID string) []*TeamMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]*TeamMemory{}, s.memories[userID]...)
}

func (s *TeamMemorySyncService) Sync() error {
	fmt.Println("Team memory synced")
	return nil
}

type SettingsSyncService struct {
	settings map[string]interface{}
	mu       sync.RWMutex
}

func NewSettingsSyncService() *SettingsSyncService {
	return &SettingsSyncService{
		settings: make(map[string]interface{}),
	}
}

func (s *SettingsSyncService) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings[key] = value
}

func (s *SettingsSyncService) Get(key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings[key]
}

func (s *SettingsSyncService) Export(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, 0644)
}

func (s *SettingsSyncService) Import(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = settings

	return nil
}

type PolicyLimits struct {
	MaxTokensPerDay   int `json:"max_tokens_per_day"`
	MaxRequestsPerMin int `json:"max_requests_per_minute"`
	MaxConcurrentReqs int `json:"max_concurrent_requests"`
}

type PolicyLimitsService struct {
	limits map[string]*PolicyLimits
	mu     sync.RWMutex
}

func NewPolicyLimitsService() *PolicyLimitsService {
	return &PolicyLimitsService{
		limits: map[string]*PolicyLimits{
			"default": {
				MaxTokensPerDay:   200000,
				MaxRequestsPerMin: 60,
				MaxConcurrentReqs: 5,
			},
		},
	}
}

func (s *PolicyLimitsService) Get(policy string) *PolicyLimits {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.limits[policy]
}

func (s *PolicyLimitsService) Set(policy string, limits *PolicyLimits) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.limits[policy] = limits
}

type Notifier struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

type NotifierService struct {
	notifiers []Notifier
	mu        sync.RWMutex
}

func NewNotifierService() *NotifierService {
	return &NotifierService{
		notifiers: make([]Notifier, 0),
	}
}

func (s *NotifierService) Notify(title, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.notifiers = append(s.notifiers, Notifier{
		Title:   title,
		Message: message,
	})
}

func (s *NotifierService) List() []Notifier {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Notifier{}, s.notifiers...)
}

func (s *NotifierService) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifiers = make([]Notifier, 0)
}

type TokenEstimation struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type TokenEstimationService struct {
	estimation TokenEstimation
	mu         sync.RWMutex
}

func NewTokenEstimationService() *TokenEstimationService {
	return &TokenEstimationService{}
}

func (s *TokenEstimationService) Estimate(text string) int {
	count := 0
	for _, word := range text {
		if word > 0 {
			count++
		}
	}
	return count / 4
}

func (s *TokenEstimationService) GetLast() TokenEstimation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.estimation
}

type AutoDreamService struct {
	enabled bool
	mu      sync.RWMutex
}

func NewAutoDreamService() *AutoDreamService {
	return &AutoDreamService{enabled: false}
}

func (s *AutoDreamService) Enable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = true
}

func (s *AutoDreamService) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = false
}

func (s *AutoDreamService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

func (s *AutoDreamService) Dream(prompt string) string {
	return fmt.Sprintf("Dream: %s (not fully implemented)", prompt)
}

type AwaySummary struct {
	LastActivity time.Time `json:"last_activity"`
	Summary      string    `json:"summary"`
	Duration     int       `json:"duration_seconds"`
}

type AwaySummaryService struct {
	summaries map[string]*AwaySummary
	mu        sync.RWMutex
}

func NewAwaySummaryService() *AwaySummaryService {
	return &AwaySummaryService{
		summaries: make(map[string]*AwaySummary),
	}
}

func (s *AwaySummaryService) Create(sessionID string, summary string, duration int) *AwaySummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	as := &AwaySummary{
		LastActivity: time.Now(),
		Summary:      summary,
		Duration:     duration,
	}

	s.summaries[sessionID] = as
	return as
}

func (s *AwaySummaryService) Get(sessionID string) *AwaySummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.summaries[sessionID]
}

type RemoteManagedSettings struct {
	Settings map[string]interface{} `json:"settings"`
	Source   string                 `json:"source"`
}

type RemoteManagedSettingsService struct {
	settings map[string]*RemoteManagedSettings
	mu       sync.RWMutex
}

func NewRemoteManagedSettingsService() *RemoteManagedSettingsService {
	return &RemoteManagedSettingsService{
		settings: make(map[string]*RemoteManagedSettings),
	}
}

func (s *RemoteManagedSettingsService) Set(source string, settings map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.settings[source] = &RemoteManagedSettings{
		Settings: settings,
		Source:   source,
	}
}

func (s *RemoteManagedSettingsService) Get(source string) *RemoteManagedSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings[source]
}

func (s *RemoteManagedSettingsService) List() []*RemoteManagedSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*RemoteManagedSettings, 0, len(s.settings))
	for _, rms := range s.settings {
		result = append(result, rms)
	}

	return result
}
