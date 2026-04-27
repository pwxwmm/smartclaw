package services

import (
	"context"
	"testing"
)

func TestDiagnosticRegistry_AddAndGet(t *testing.T) {
	r := NewDiagnosticRegistry()
	r.Add("main.go", Diagnostic{Code: "E1", Message: "err", Severity: "error", Source: "test"})
	r.Add("main.go", Diagnostic{Code: "W1", Message: "warn", Severity: "warning", Source: "test"})

	diags := r.Get("main.go")
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}
	if diags[0].Code != "E1" {
		t.Errorf("first diag code = %q, want %q", diags[0].Code, "E1")
	}
	if diags[1].Severity != "warning" {
		t.Errorf("second diag severity = %q, want %q", diags[1].Severity, "warning")
	}
}

func TestDiagnosticRegistry_Get_NonExistent(t *testing.T) {
	r := NewDiagnosticRegistry()
	diags := r.Get("nonexistent.go")
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for missing file, got %d", len(diags))
	}
}

func TestDiagnosticRegistry_GetAll(t *testing.T) {
	r := NewDiagnosticRegistry()
	r.Add("a.go", Diagnostic{Code: "E1", Message: "err1", Severity: "error", Source: "test"})
	r.Add("b.go", Diagnostic{Code: "W1", Message: "warn1", Severity: "warning", Source: "test"})

	all := r.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 files in GetAll, got %d", len(all))
	}
	if len(all["a.go"]) != 1 || len(all["b.go"]) != 1 {
		t.Error("each file should have exactly 1 diagnostic")
	}
}

func TestDiagnosticRegistry_Clear(t *testing.T) {
	r := NewDiagnosticRegistry()
	r.Add("main.go", Diagnostic{Code: "E1", Message: "err", Severity: "error", Source: "test"})
	r.Clear("main.go")
	diags := r.Get("main.go")
	if len(diags) != 0 {
		t.Error("diagnostics should be cleared")
	}
}

func TestDiagnosticRegistry_ClearAll(t *testing.T) {
	r := NewDiagnosticRegistry()
	r.Add("a.go", Diagnostic{Code: "E1", Message: "err", Severity: "error", Source: "test"})
	r.Add("b.go", Diagnostic{Code: "E2", Message: "err2", Severity: "error", Source: "test"})
	r.ClearAll()
	all := r.GetAll()
	if len(all) != 0 {
		t.Error("all diagnostics should be cleared")
	}
}

func TestDiagnosticRegistry_HasErrors_True(t *testing.T) {
	r := NewDiagnosticRegistry()
	r.Add("main.go", Diagnostic{Code: "E1", Severity: "error", Source: "test"})
	if !r.HasErrors("main.go") {
		t.Error("expected HasErrors = true when error diagnostic exists")
	}
}

func TestDiagnosticRegistry_HasErrors_False(t *testing.T) {
	r := NewDiagnosticRegistry()
	r.Add("main.go", Diagnostic{Code: "W1", Severity: "warning", Source: "test"})
	if r.HasErrors("main.go") {
		t.Error("expected HasErrors = false when only warnings exist")
	}
}

func TestDiagnosticRegistry_HasErrors_Empty(t *testing.T) {
	r := NewDiagnosticRegistry()
	if r.HasErrors("missing.go") {
		t.Error("expected HasErrors = false for file with no diagnostics")
	}
}

func TestDiagnosticRegistry_SetDiagnostics(t *testing.T) {
	r := NewDiagnosticRegistry()
	r.Add("main.go", Diagnostic{Code: "OLD", Severity: "error", Source: "test"})
	r.SetDiagnostics("main.go", []Diagnostic{
		{Code: "NEW", Severity: "warning", Source: "test"},
	})
	diags := r.Get("main.go")
	if len(diags) != 1 || diags[0].Code != "NEW" {
		t.Error("SetDiagnostics should replace existing diagnostics")
	}
}

func TestDiagnosticRegistry_GetReturnsCopy(t *testing.T) {
	r := NewDiagnosticRegistry()
	r.Add("main.go", Diagnostic{Code: "E1", Severity: "error", Source: "test"})
	d1 := r.Get("main.go")
	d2 := r.Get("main.go")
	if &d1[0] == &d2[0] {
		t.Error("Get should return a copy, not the same slice")
	}
}

type mockLSPClient struct {
	closed bool
}

func (m *mockLSPClient) Initialize(ctx context.Context, rootPath string) error { return nil }
func (m *mockLSPClient) DidOpen(ctx context.Context, filePath, languageID, content string) error {
	return nil
}
func (m *mockLSPClient) DidChange(ctx context.Context, filePath, content string) error {
	return nil
}
func (m *mockLSPClient) GotoDefinition(ctx context.Context, filePath string, line, character int) (any, error) {
	return map[string]any{"uri": "file:///test.go"}, nil
}
func (m *mockLSPClient) FindReferences(ctx context.Context, filePath string, line, character int) (any, error) {
	return []any{}, nil
}
func (m *mockLSPClient) DocumentSymbols(ctx context.Context, filePath string) (any, error) {
	return []any{}, nil
}
func (m *mockLSPClient) Hover(ctx context.Context, filePath string, line, character int) (any, error) {
	return map[string]any{"contents": "info"}, nil
}
func (m *mockLSPClient) Rename(ctx context.Context, filePath string, line, character int, newName string) (any, error) {
	return map[string]any{}, nil
}
func (m *mockLSPClient) Completion(ctx context.Context, filePath string, line, character int) (any, error) {
	return []any{}, nil
}
func (m *mockLSPClient) Close() error {
	m.closed = true
	return nil
}

func TestLSPServerManager_Start(t *testing.T) {
	mgr := NewLSPServerManager()
	srv, err := mgr.Start(context.Background(), "go", "gopls", []string{})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if srv.Language != "go" {
		t.Errorf("server language = %q, want %q", srv.Language, "go")
	}
	if srv.Status != "running" {
		t.Errorf("server status = %q, want %q", srv.Status, "running")
	}
}

func TestLSPServerManager_StartWithClient(t *testing.T) {
	mgr := NewLSPServerManager()
	client := &mockLSPClient{}
	srv, err := mgr.StartWithClient(context.Background(), "go", client)
	if err != nil {
		t.Fatalf("StartWithClient returned error: %v", err)
	}
	if srv.Client == nil {
		t.Error("server should have client attached")
	}
}

func TestLSPServerManager_Stop(t *testing.T) {
	mgr := NewLSPServerManager()
	client := &mockLSPClient{}
	srv, _ := mgr.StartWithClient(context.Background(), "go", client)

	err := mgr.Stop(srv.ID)
	if err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if !client.closed {
		t.Error("client should be closed on stop")
	}

	_, exists := mgr.Get(srv.ID)
	if exists {
		t.Error("server should be removed after stop")
	}
}

func TestLSPServerManager_Stop_NotFound(t *testing.T) {
	mgr := NewLSPServerManager()
	err := mgr.Stop("nonexistent")
	if err == nil {
		t.Error("expected error when stopping non-existent server")
	}
}

func TestLSPServerManager_List(t *testing.T) {
	mgr := NewLSPServerManager()
	mgr.Start(context.Background(), "go", "gopls", nil)
	mgr.Start(context.Background(), "python", "pyright", nil)

	list := mgr.List()
	if len(list) != 2 {
		t.Errorf("expected 2 servers, got %d", len(list))
	}
}

func TestLSPServerManager_StopAll(t *testing.T) {
	mgr := NewLSPServerManager()
	client := &mockLSPClient{}
	mgr.StartWithClient(context.Background(), "go", client)
	mgr.Start(context.Background(), "python", "pyright", nil)

	mgr.StopAll()
	list := mgr.List()
	if len(list) != 0 {
		t.Errorf("expected 0 servers after StopAll, got %d", len(list))
	}
	if !client.closed {
		t.Error("client should be closed on StopAll")
	}
}

func TestLSPServerManager_GotoDefinition(t *testing.T) {
	mgr := NewLSPServerManager()
	client := &mockLSPClient{}
	srv, _ := mgr.StartWithClient(context.Background(), "go", client)

	result, err := mgr.GotoDefinition(context.Background(), srv.ID, "test.go", 10, 5)
	if err != nil {
		t.Fatalf("GotoDefinition returned error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result from GotoDefinition")
	}
}

func TestLSPServerManager_GotoDefinition_NoClient(t *testing.T) {
	mgr := NewLSPServerManager()
	srv, _ := mgr.Start(context.Background(), "go", "gopls", nil)

	_, err := mgr.GotoDefinition(context.Background(), srv.ID, "test.go", 10, 5)
	if err == nil {
		t.Error("expected error when server has no client")
	}
}

func TestLSPServerManager_GotoDefinition_NotFound(t *testing.T) {
	mgr := NewLSPServerManager()
	_, err := mgr.GotoDefinition(context.Background(), "bad_id", "test.go", 10, 5)
	if err == nil {
		t.Error("expected error for non-existent server")
	}
}

func TestLSPServerManager_FindReferences(t *testing.T) {
	mgr := NewLSPServerManager()
	client := &mockLSPClient{}
	srv, _ := mgr.StartWithClient(context.Background(), "go", client)

	result, err := mgr.FindReferences(context.Background(), srv.ID, "test.go", 10, 5)
	if err != nil {
		t.Fatalf("FindReferences returned error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result from FindReferences")
	}
}

func TestLSPServerManager_Hover(t *testing.T) {
	mgr := NewLSPServerManager()
	client := &mockLSPClient{}
	srv, _ := mgr.StartWithClient(context.Background(), "go", client)

	result, err := mgr.Hover(context.Background(), srv.ID, "test.go", 10, 5)
	if err != nil {
		t.Fatalf("Hover returned error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result from Hover")
	}
}

func TestLSPServerManager_DocumentSymbols(t *testing.T) {
	mgr := NewLSPServerManager()
	client := &mockLSPClient{}
	srv, _ := mgr.StartWithClient(context.Background(), "go", client)

	result, err := mgr.DocumentSymbols(context.Background(), srv.ID, "test.go")
	if err != nil {
		t.Fatalf("DocumentSymbols returned error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result from DocumentSymbols")
	}
}

func TestLSPServerManager_Completion(t *testing.T) {
	mgr := NewLSPServerManager()
	client := &mockLSPClient{}
	srv, _ := mgr.StartWithClient(context.Background(), "go", client)

	result, err := mgr.Completion(context.Background(), srv.ID, "test.go", 10, 5)
	if err != nil {
		t.Fatalf("Completion returned error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result from Completion")
	}
}

func TestLSPServerManager_Rename(t *testing.T) {
	mgr := NewLSPServerManager()
	client := &mockLSPClient{}
	srv, _ := mgr.StartWithClient(context.Background(), "go", client)

	result, err := mgr.Rename(context.Background(), srv.ID, "test.go", 10, 5, "newName")
	if err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result from Rename")
	}
}
