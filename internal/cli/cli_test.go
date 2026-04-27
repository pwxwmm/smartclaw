package cli

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/instructkr/smartclaw/internal/runtime"
)

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"short", 10, "short"},
		{"exact", 5, "exact"},
		{"", 5, ""},
		{"a", 1, "a"},
		{"ab", 1, "a..."},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestTruncate_ExactBoundary(t *testing.T) {
	t.Parallel()

	got := truncate("12345", 5)
	if got != "12345" {
		t.Errorf("truncate at exact boundary = %q, want %q", got, "12345")
	}
}

func TestTruncate_OneOver(t *testing.T) {
	t.Parallel()

	got := truncate("123456", 5)
	if got != "12345..." {
		t.Errorf("truncate one over boundary = %q, want %q", got, "12345...")
	}
}

func TestJsonMarshal(t *testing.T) {
	t.Parallel()

	data := map[string]string{"key": "value"}
	result, err := jsonMarshal(data)
	if err != nil {
		t.Fatalf("jsonMarshal() returned error: %v", err)
	}

	var decoded map[string]string
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() returned error: %v", err)
	}
	if decoded["key"] != "value" {
		t.Errorf("decoded[\"key\"] = %q, want %q", decoded["key"], "value")
	}
}

func TestJsonMarshal_NilInput(t *testing.T) {
	t.Parallel()

	result, err := jsonMarshal(nil)
	if err != nil {
		t.Fatalf("jsonMarshal(nil) returned error: %v", err)
	}
	if string(result) != "null" {
		t.Errorf("jsonMarshal(nil) = %q, want %q", string(result), "null")
	}
}

func TestClientConfig_Struct(t *testing.T) {
	t.Parallel()

	cfg := ClientConfig{
		APIKey:       "test-key",
		Model:        "claude-sonnet-4-5",
		MaxTokens:    4096,
		SystemPrompt: "You are helpful",
		BaseURL:      "https://api.anthropic.com",
		IsOpenAI:     false,
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "test-key")
	}
	if cfg.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q, want %q", cfg.Model, "claude-sonnet-4-5")
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096", cfg.MaxTokens)
	}
	if cfg.IsOpenAI {
		t.Error("IsOpenAI should be false")
	}
}

func TestOutputJSON_Structure(t *testing.T) {
	t.Parallel()

	output := map[string]any{
		"content":       "response text",
		"input_tokens":  100,
		"output_tokens": 50,
		"cost":          0.001,
		"stop_reason":   "end_turn",
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() returned error: %v", err)
	}

	if decoded["stop_reason"] != "end_turn" {
		t.Errorf("stop_reason = %v, want %q", decoded["stop_reason"], "end_turn")
	}
}

func TestUpdateCost(t *testing.T) {
	s := &REPLSession{}

	s.updateCost(1000, 500)
	expected := float64(1000)*0.000003 + float64(500)*0.000015
	if s.totalCost != expected {
		t.Errorf("totalCost after first update = %f, want %f", s.totalCost, expected)
	}

	s.updateCost(1000, 500)
	if s.totalCost != expected*2 {
		t.Errorf("totalCost after second update = %f, want %f", s.totalCost, expected*2)
	}
}

func TestUpdateCost_ZeroTokens(t *testing.T) {
	s := &REPLSession{}
	s.updateCost(0, 0)
	if s.totalCost != 0 {
		t.Errorf("totalCost with zero tokens = %f, want 0", s.totalCost)
	}
}

func TestUpdateCost_InputOnly(t *testing.T) {
	s := &REPLSession{}
	s.updateCost(1000, 0)
	expected := float64(1000) * 0.000003
	if s.totalCost != expected {
		t.Errorf("totalCost with input only = %f, want %f", s.totalCost, expected)
	}
}

func TestUpdateCost_OutputOnly(t *testing.T) {
	s := &REPLSession{}
	s.updateCost(0, 1000)
	expected := float64(1000) * 0.000015
	if s.totalCost != expected {
		t.Errorf("totalCost with output only = %f, want %f", s.totalCost, expected)
	}
}

func TestBuildAPIMessages(t *testing.T) {
	sess := &runtime.Session{
		Messages: []runtime.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "world"},
		},
	}
	s := &REPLSession{session: sess}

	messages := s.buildAPIMessages()
	if len(messages) != 2 {
		t.Fatalf("buildAPIMessages() returned %d messages, want 2", len(messages))
	}
	if messages[0].Role != "user" {
		t.Errorf("messages[0].Role = %q, want %q", messages[0].Role, "user")
	}
	if messages[0].Content != "hello" {
		t.Errorf("messages[0].Content = %q, want %q", messages[0].Content, "hello")
	}
}

func TestBuildAPIMessages_Empty(t *testing.T) {
	sess := &runtime.Session{Messages: []runtime.Message{}}
	s := &REPLSession{session: sess}

	messages := s.buildAPIMessages()
	if len(messages) != 0 {
		t.Errorf("buildAPIMessages() on empty session returned %d messages, want 0", len(messages))
	}
}

func TestLastUserInput(t *testing.T) {
	sess := &runtime.Session{
		Messages: []runtime.Message{
			{Role: "assistant", Content: "hi"},
			{Role: "user", Content: "what is Go?"},
			{Role: "assistant", Content: "Go is a language"},
		},
	}
	s := &REPLSession{session: sess}

	result := s.lastUserInput()
	if result != "what is Go?" {
		t.Errorf("lastUserInput() = %q, want %q", result, "what is Go?")
	}
}

func TestLastUserInput_Empty(t *testing.T) {
	sess := &runtime.Session{Messages: []runtime.Message{}}
	s := &REPLSession{session: sess}

	result := s.lastUserInput()
	if result != "" {
		t.Errorf("lastUserInput() on empty session = %q, want empty", result)
	}
}

func TestVersion_Variables(t *testing.T) {
	t.Parallel()

	if Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestGithubRelease_JSON(t *testing.T) {
	t.Parallel()

	data := `{"tag_name": "v1.0.0", "html_url": "https://github.com/test/release"}`
	var release githubRelease
	if err := json.Unmarshal([]byte(data), &release); err != nil {
		t.Fatalf("json.Unmarshal() returned error: %v", err)
	}
	if release.TagName != "v1.0.0" {
		t.Errorf("TagName = %q, want %q", release.TagName, "v1.0.0")
	}
	if release.HTMLURL != "https://github.com/test/release" {
		t.Errorf("HTMLURL = %q, want %q", release.HTMLURL, "https://github.com/test/release")
	}
}

func TestOutputWriter_Struct(t *testing.T) {
	t.Parallel()

	w := NewOutputWriter(true, false)
	if !w.useColor {
		t.Error("useColor should be true")
	}
	if w.useJSON {
		t.Error("useJSON should be false")
	}
}

func TestOutputWriter_NoColor(t *testing.T) {
	t.Parallel()

	w := NewOutputWriter(false, true)
	if w.useColor {
		t.Error("useColor should be false")
	}
	if !w.useJSON {
		t.Error("useJSON should be true")
	}
}

func TestNewREPL(t *testing.T) {
	t.Parallel()

	r := NewREPL("test>")
	if r.prompt != "test>" {
		t.Errorf("prompt = %q, want %q", r.prompt, "test>")
	}
	if r.running {
		t.Error("New REPL should not be running")
	}
	if len(r.history) != 0 {
		t.Errorf("history should be empty, got %d items", len(r.history))
	}
}

func TestREPL_GetHistory(t *testing.T) {
	t.Parallel()

	r := NewREPL(">")
	r.history = []string{"cmd1", "cmd2"}
	history := r.GetHistory()
	if len(history) != 2 {
		t.Errorf("GetHistory() returned %d items, want 2", len(history))
	}
}

func TestREPL_ClearHistory(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewREPL(">")
	histPath := tmpDir + "/history"
	r.historyFile = histPath
	r.history = []string{"cmd1"}
	os.WriteFile(histPath, []byte("cmd1\n"), 0644)

	err := r.ClearHistory()
	if err != nil {
		t.Errorf("ClearHistory() returned error: %v", err)
	}
	if len(r.history) != 0 {
		t.Errorf("history after ClearHistory = %d items, want 0", len(r.history))
	}
}

func TestNewPromptInput(t *testing.T) {
	t.Parallel()

	p := NewPromptInput()
	if p == nil {
		t.Fatal("NewPromptInput() returned nil")
	}
	if p.promptColor != "\033[36m" {
		t.Errorf("promptColor = %q, want cyan ANSI", p.promptColor)
	}
}

func TestHandleSlashCommand_Exit(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
	}
	result := s.handleSlashCommand("/exit")
	if !result {
		t.Error("handleSlashCommand(\"/exit\") should return true")
	}
}

func TestHandleSlashCommand_Quit(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
	}
	result := s.handleSlashCommand("/quit")
	if !result {
		t.Error("handleSlashCommand(\"/quit\") should return true")
	}
}

func TestHandleSlashCommand_EmptyInput(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
	}
	result := s.handleSlashCommand("")
	if result {
		t.Error("handleSlashCommand(\"\") should return false")
	}
}

func TestHandleSlashCommand_UnknownCommand(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
	}
	result := s.handleSlashCommand("/nonexistent_command")
	if result {
		t.Error("handleSlashCommand with unknown command should return false")
	}
}

func TestHandleSlashCommand_Help(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
	}
	result := s.handleSlashCommand("/help")
	if result {
		t.Error("handleSlashCommand(\"/help\") should return false (not exit)")
	}
}

func TestHandleSlashCommand_Model_Set(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
		model:   "old-model",
	}
	s.handleSlashCommand("/model new-model")
	if s.model != "new-model" {
		t.Errorf("model after /model command = %q, want %q", s.model, "new-model")
	}
}

func TestHandleSlashCommand_Model_Show(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
		model:   "claude-sonnet-4-5",
	}
	result := s.handleSlashCommand("/model")
	if result {
		t.Error("handleSlashCommand(\"/model\") should return false (not exit)")
	}
}
