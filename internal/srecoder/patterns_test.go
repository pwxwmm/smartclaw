package srecoder

import (
	"strings"
	"testing"
)

func TestGetPatterns_ReturnsNonEmpty(t *testing.T) {
	patterns := GetPatterns()
	if len(patterns) == 0 {
		t.Error("GetPatterns() should return at least one pattern")
	}
}

func TestGetPatterns_RequiredFields(t *testing.T) {
	patterns := GetPatterns()
	for _, p := range patterns {
		if p.Name == "" {
			t.Error("pattern should have a non-empty Name")
		}
		if p.Description == "" {
			t.Errorf("pattern %q should have a non-empty Description", p.Name)
		}
		if p.GoTemplate == "" {
			t.Errorf("pattern %q should have a non-empty GoTemplate", p.Name)
		}
		if p.Category == "" {
			t.Errorf("pattern %q should have a non-empty Category", p.Name)
		}
	}
}

func TestGetPatterns_KnownPatterns(t *testing.T) {
	patterns := GetPatterns()
	names := make(map[string]bool)
	for _, p := range patterns {
		names[p.Name] = true
	}
	expected := []string{"circuit_breaker", "retry_with_backoff", "rate_limiter", "bulkhead"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected pattern %q not found", name)
		}
	}
}

func TestGetPatternForCode_HTTPCode(t *testing.T) {
	code := `package main
import "net/http"
func handler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }`
	patterns := GetPatternForCode(code)
	if len(patterns) == 0 {
		t.Error("HTTP code should match at least one pattern")
	}
	names := make(map[string]bool)
	for _, p := range patterns {
		names[p.Name] = true
	}
	if !names["health_check_handler"] {
		t.Error("HTTP code should recommend health_check_handler")
	}
}

func TestGetPatternForCode_ExternalCalls(t *testing.T) {
	code := `package main
func callDB() error {
	db, _ := sql.Open("postgres", dsn)
	return db.Ping()
}`
	patterns := GetPatternForCode(code)
	names := make(map[string]bool)
	for _, p := range patterns {
		names[p.Name] = true
	}
	if !names["circuit_breaker"] {
		t.Error("code with external calls should recommend circuit_breaker")
	}
	if !names["retry_with_backoff"] {
		t.Error("code with external calls should recommend retry_with_backoff")
	}
}

func TestGetPatternForCode_ContextUsage(t *testing.T) {
	code := `package main
import "context"
func process(ctx context.Context) error { return nil }`
	patterns := GetPatternForCode(code)
	names := make(map[string]bool)
	for _, p := range patterns {
		names[p.Name] = true
	}
	if !names["timeout_with_context"] {
		t.Error("code with context should recommend timeout_with_context")
	}
}

func TestGetPatternForCode_Goroutines(t *testing.T) {
	code := `package main
func run() {
	go func() { doWork() }()
}`
	patterns := GetPatternForCode(code)
	names := make(map[string]bool)
	for _, p := range patterns {
		names[p.Name] = true
	}
	if !names["bulkhead"] {
		t.Error("code with goroutines should recommend bulkhead")
	}
}

func TestGetPatternForCode_EmptyCode(t *testing.T) {
	patterns := GetPatternForCode("")
	if len(patterns) != 0 {
		t.Errorf("empty code should match no patterns, got %d", len(patterns))
	}
}

func TestGetPatternForCode_MainFunction(t *testing.T) {
	code := `package main
func main() {
	http.ListenAndServe(":8080", nil)
}`
	patterns := GetPatternForCode(code)
	names := make(map[string]bool)
	for _, p := range patterns {
		names[p.Name] = true
	}
	if !names["graceful_shutdown"] {
		t.Error("code with main and http should recommend graceful_shutdown")
	}
}

func TestApplyPattern_InsertsAfterImports(t *testing.T) {
	code := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}`
	pattern := GetPatterns()[0]
	result, err := ApplyPattern(pattern, code)
	if err != nil {
		t.Fatalf("ApplyPattern returned error: %v", err)
	}
	if !strings.Contains(result, pattern.GoTemplate[:20]) {
		t.Error("result should contain the pattern template")
	}
	if !strings.Contains(result, "func main()") {
		t.Error("result should still contain original code")
	}
}

func TestApplyPattern_NoImports(t *testing.T) {
	code := `package main

func main() {}`
	pattern := GetPatterns()[0]
	result, err := ApplyPattern(pattern, code)
	if err != nil {
		t.Fatalf("ApplyPattern returned error: %v", err)
	}
	if !strings.Contains(result, pattern.GoTemplate[:20]) {
		t.Error("result should contain the pattern template even without imports")
	}
}

func TestFindInsertionPoint_WithImportBlock(t *testing.T) {
	code := `package main

import (
	"fmt"
	"os"
)

func main() {}`
	point := findInsertionPoint(code)
	if point <= 0 {
		t.Errorf("expected positive insertion point for code with import block, got %d", point)
	}
}

func TestFindInsertionPoint_SingleImport(t *testing.T) {
	code := `package main

import "fmt"

func main() {}`
	point := findInsertionPoint(code)
	if point <= 0 {
		t.Errorf("expected positive insertion point for code with single import, got %d", point)
	}
}

func TestFindInsertionPoint_NoImports(t *testing.T) {
	code := `package main

func main() {}`
	point := findInsertionPoint(code)
	if point != 1 {
		t.Errorf("expected insertion point at line after package, got %d", point)
	}
}

func TestDetectChangeType_ConfigFiles(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"config.yaml", "config_change"},
		{"settings.json", "config_change"},
		{"app.toml", "config_change"},
		{".env", "config_change"},
		{"migration.sql", "migration"},
		{"Dockerfile", "deployment"},
		{"main.go", "code_change"},
	}
	for _, tt := range tests {
		result := detectChangeType(tt.path, "")
		if result != tt.expected {
			t.Errorf("detectChangeType(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestMapChangeType_AllTypes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"deployment", "deployment"},
		{"config_change", "config_change"},
		{"scaling", "scaling"},
		{"rollback", "rollback"},
		{"hotfix", "hotfix"},
		{"migration", "migration"},
		{"unknown", "deployment"},
	}
	for _, tt := range tests {
		result := mapChangeType(tt.input)
		if string(result) != tt.expected {
			t.Errorf("mapChangeType(%q) = %q, want %q", tt.input, string(result), tt.expected)
		}
	}
}

func TestFileToService_InternalPath(t *testing.T) {
	result := fileToService("internal/api/handler.go")
	if result != "api" {
		t.Errorf("fileToService('internal/api/handler.go') = %q, want %q", result, "api")
	}
}

func TestFileToService_CmdPath(t *testing.T) {
	result := fileToService("cmd/server/main.go")
	if result != "server" {
		t.Errorf("fileToService('cmd/server/main.go') = %q, want %q", result, "server")
	}
}

func TestFileToService_PkgPath(t *testing.T) {
	result := fileToService("pkg/utils/helper.go")
	if result != "utils" {
		t.Errorf("fileToService('pkg/utils/helper.go') = %q, want %q", result, "utils")
	}
}

func TestFileToService_Fallback(t *testing.T) {
	result := fileToService("handler.go")
	if result != "." {
		t.Errorf("fileToService('handler.go') = %q, want '.' (parent dir)", result)
	}
}

func TestGenerateSuggestions_CriticalRisk(t *testing.T) {
	m := &SRECodingMode{}
	analysis := &ImpactAnalysis{
		RiskScore:   0.8,
		BlastRadius: []string{"svc1"},
	}
	m.generateSuggestions(analysis)
	found := false
	for _, s := range analysis.Suggestions {
		if strings.Contains(s, "CRITICAL RISK") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected CRITICAL RISK suggestion for risk score >= 0.75")
	}
}

func TestGenerateSuggestions_LowRisk(t *testing.T) {
	m := &SRECodingMode{}
	analysis := &ImpactAnalysis{
		RiskScore:   0.1,
		BlastRadius: []string{"svc1"},
	}
	m.generateSuggestions(analysis)
	found := false
	for _, s := range analysis.Suggestions {
		if strings.Contains(s, "LOW RISK") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected LOW RISK suggestion for risk score < 0.25")
	}
}

func TestGenerateSuggestions_LargeBlastRadius(t *testing.T) {
	m := &SRECodingMode{}
	services := make([]string, 8)
	for i := range services {
		services[i] = "svc" + string(rune('A'+i))
	}
	analysis := &ImpactAnalysis{
		RiskScore:   0.1,
		BlastRadius: services,
	}
	m.generateSuggestions(analysis)
	found := false
	for _, s := range analysis.Suggestions {
		if strings.Contains(s, "Large blast radius") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected large blast radius warning with >5 services")
	}
}

func TestGenerateSuggestions_ConfigChange(t *testing.T) {
	m := &SRECodingMode{}
	analysis := &ImpactAnalysis{
		RiskScore:   0.1,
		ChangeType:  "config_change",
		BlastRadius: []string{"svc1"},
	}
	m.generateSuggestions(analysis)
	found := false
	for _, s := range analysis.Suggestions {
		if strings.Contains(s, "Config changes") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected config change suggestion for config_change type")
	}
}
