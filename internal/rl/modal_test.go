package rl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestModalBackend(server *httptest.Server) *ModalBackend {
	mb := NewModalBackend(server.URL, "test-api-key", "test-app")
	mb.httpClient = server.Client()
	return mb
}

func TestNewModalBackend(t *testing.T) {
	mb := NewModalBackend("", "key", "app1")
	if mb.apiURL != defaultModalAPIURL {
		t.Errorf("expected default URL %q, got %q", defaultModalAPIURL, mb.apiURL)
	}
	if mb.apiKey != "key" {
		t.Errorf("expected apiKey=key, got %q", mb.apiKey)
	}
	if mb.appID != "app1" {
		t.Errorf("expected appID=app1, got %q", mb.appID)
	}
	if mb.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
	if mb.httpClient.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s, got %v", mb.httpClient.Timeout)
	}

	mb2 := NewModalBackend("https://custom.modal.api", "key2", "app2")
	if mb2.apiURL != "https://custom.modal.api" {
		t.Errorf("expected custom URL, got %q", mb2.apiURL)
	}
}

func TestModalName(t *testing.T) {
	mb := NewModalBackend("", "key", "app")
	if mb.Name() != "modal" {
		t.Errorf("expected name=modal, got %q", mb.Name())
	}
}

func TestModalCreateWorkspace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/apps" {
			t.Errorf("expected path /apps, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Bearer auth header, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if payload["name"] != "test-ws" {
			t.Errorf("expected name=test-ws, got %v", payload["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"app_id": "app-123",
		})
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	id, err := mb.CreateWorkspace(context.Background(), WorkspaceConfig{
		Name:  "test-ws",
		Image: "python:3.11",
		Env:   map[string]string{"FOO": "bar"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	if id != "app-123" {
		t.Errorf("expected app-123, got %q", id)
	}
}

func TestModalExecuteTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		expectedPath := "/apps/ws-456/run"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if payload["command"] != "echo hello" {
			t.Errorf("expected command=echo hello, got %v", payload["command"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"exit_code":    0,
			"stdout":       "hello\n",
			"stderr":       "",
			"duration":     1500000000,
			"workspace_id": "ws-456",
		})
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	result, err := mb.ExecuteTask(context.Background(), "ws-456", TaskSpec{
		Command: "echo hello",
		Args:    []string{"arg1"},
		Env:     map[string]string{"BAZ": "qux"},
	})
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit_code=0, got %d", result.ExitCode)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("expected stdout='hello\\n', got %q", result.Stdout)
	}
	if result.WorkspaceID != "ws-456" {
		t.Errorf("expected workspace_id=ws-456, got %q", result.WorkspaceID)
	}
}

func TestModalDestroyWorkspace(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/apps/ws-789" {
			t.Errorf("expected path /apps/ws-789, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	err := mb.DestroyWorkspace(context.Background(), "ws-789")
	if err != nil {
		t.Fatalf("DestroyWorkspace failed: %v", err)
	}
	if !called {
		t.Error("expected server to be called")
	}
}

func TestModalDestroyWorkspace_200OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	err := mb.DestroyWorkspace(context.Background(), "ws-789")
	if err != nil {
		t.Fatalf("DestroyWorkspace with 200 should succeed: %v", err)
	}
}

func TestModalWorkspaceStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/apps/ws-status" {
			t.Errorf("expected path /apps/ws-status, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":         "ws-status",
			"state":      "running",
			"created_at": time.Now().Format(time.RFC3339),
		})
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	status, err := mb.WorkspaceStatus(context.Background(), "ws-status")
	if err != nil {
		t.Fatalf("WorkspaceStatus failed: %v", err)
	}
	if status.ID != "ws-status" {
		t.Errorf("expected id=ws-status, got %q", status.ID)
	}
	if status.State != "running" {
		t.Errorf("expected state=running, got %q", status.State)
	}
}

func TestModalListWorkspaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/apps" {
			t.Errorf("expected path /apps, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": "app-1", "name": "ws-1", "state": "running", "created_at": time.Now().Format(time.RFC3339)},
			{"id": "app-2", "name": "ws-2", "state": "stopped", "created_at": time.Now().Format(time.RFC3339)},
		})
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	workspaces, err := mb.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
	if workspaces[0].ID != "app-1" {
		t.Errorf("expected id=app-1, got %q", workspaces[0].ID)
	}
	if workspaces[1].Name != "ws-2" {
		t.Errorf("expected name=ws-2, got %q", workspaces[1].Name)
	}
	if workspaces[1].State != "stopped" {
		t.Errorf("expected state=stopped, got %q", workspaces[1].State)
	}
}

func TestModalDeployFunction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/functions" {
			t.Errorf("expected path /functions, got %s", r.URL.Path)
		}

		var fn ModalFunction
		if err := json.NewDecoder(r.Body).Decode(&fn); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if fn.Name != "my-fn" {
			t.Errorf("expected name=my-fn, got %q", fn.Name)
		}
		if fn.Image != "python:3.11" {
			t.Errorf("expected image=python:3.11, got %q", fn.Image)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"function_id": "fn-abc",
		})
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	id, err := mb.DeployFunction(context.Background(), ModalFunction{
		Name:         "my-fn",
		Image:        "python:3.11",
		Code:         "def handler(): pass",
		Requirements: []string{"numpy"},
	})
	if err != nil {
		t.Fatalf("DeployFunction failed: %v", err)
	}
	if id != "fn-abc" {
		t.Errorf("expected fn-abc, got %q", id)
	}
}

func TestModalInvokeFunction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		expectedPath := "/functions/fn-xyz/invoke"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"exit_code":    0,
			"stdout":       "result data",
			"stderr":       "",
			"duration":     500000000,
			"workspace_id": "",
		})
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	result, err := mb.InvokeFunction(context.Background(), "fn-xyz", []byte("test input"))
	if err != nil {
		t.Fatalf("InvokeFunction failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit_code=0, got %d", result.ExitCode)
	}
	if result.Stdout != "result data" {
		t.Errorf("expected stdout='result data', got %q", result.Stdout)
	}
}

func TestModalErrorHandling_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error": "app not found"}`)
	}))
	defer server.Close()

	mb := newTestModalBackend(server)

	_, err := mb.WorkspaceStatus(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !contains(err.Error(), "404") {
		t.Errorf("error should mention status 404, got: %v", err)
	}
}

func TestModalErrorHandling_500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error": "internal server error"}`)
	}))
	defer server.Close()

	mb := newTestModalBackend(server)

	_, err := mb.CreateWorkspace(context.Background(), WorkspaceConfig{Name: "test"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !contains(err.Error(), "500") {
		t.Errorf("error should mention status 500, got: %v", err)
	}
}

func TestModalErrorHandling_401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "unauthorized"}`)
	}))
	defer server.Close()

	mb := newTestModalBackend(server)

	_, err := mb.ListWorkspaces(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestModalErrorHandling_DestroyFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error": "forbidden"}`)
	}))
	defer server.Close()

	mb := newTestModalBackend(server)

	err := mb.DestroyWorkspace(context.Background(), "ws-forbidden")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestModalExecuteTask_WithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if timeout, ok := payload["timeout"]; ok {
			if timeout.(float64) != 30.0 {
				t.Errorf("expected timeout=30, got %v", timeout)
			}
		} else {
			t.Error("expected timeout field in payload")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"exit_code": 0,
			"stdout":    "ok",
			"stderr":    "",
			"duration":  1000000000,
		})
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	result, err := mb.ExecuteTask(context.Background(), "ws-1", TaskSpec{
		Command: "run",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit_code=0, got %d", result.ExitCode)
	}
}

func TestModalCreateWorkspace_WithImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if payload["image"] != "golang:1.22" {
			t.Errorf("expected image=golang:1.22, got %v", payload["image"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"app_id": "app-img",
		})
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	id, err := mb.CreateWorkspace(context.Background(), WorkspaceConfig{
		Name:  "img-ws",
		Image: "golang:1.22",
	})
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	if id != "app-img" {
		t.Errorf("expected app-img, got %q", id)
	}
}

func TestModalContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mb := newTestModalBackend(server)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := mb.ListWorkspaces(ctx)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestServerlessBackendInterface(t *testing.T) {
	var _ ServerlessBackend = (*ModalBackend)(nil)
}

func TestModalRunEpisodeRemote(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/apps":
			json.NewEncoder(w).Encode(map[string]any{"app_id": "ep-app-1"})

		case "/functions":
			json.NewEncoder(w).Encode(map[string]any{"function_id": "ep-fn-1"})

		case "/functions/ep-fn-1/invoke":
			episodeJSON, _ := json.Marshal(EpisodeResult{
				Steps: []StepResult{
					{Step: 0, Action: "think", Observation: "thinking", Reward: 0.5, Done: false},
					{Step: 1, Action: "code", Observation: "result", Reward: 0.8, Done: true},
				},
				TotalReward: 1.3,
				Success:     true,
				Duration:    5 * time.Second,
			})
			json.NewEncoder(w).Encode(map[string]any{
				"exit_code": 0,
				"stdout":    string(episodeJSON),
				"stderr":    "",
				"duration":  5000000000,
			})

		case "/apps/ep-app-1":
			w.WriteHeader(http.StatusNoContent)

		default:
			if r.Method == http.MethodDelete && r.URL.Path == "/apps/ep-app-1" {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, `{"error": "unexpected path: %s"}`, r.URL.Path)
			}
		}
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	result, err := mb.RunEpisodeRemote(context.Background(), EnvironmentConfig{
		MaxSteps:      10,
		Timeout:       30 * time.Second,
		SuccessMetric: "exact_match",
	}, "Write a hello world")
	if err != nil {
		t.Fatalf("RunEpisodeRemote failed: %v", err)
	}
	if result.TotalReward != 1.3 {
		t.Errorf("expected total_reward=1.3, got %.2f", result.TotalReward)
	}
	if !result.Success {
		t.Error("expected success=true")
	}
	if len(result.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(result.Steps))
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 API calls (create, deploy, invoke), got %d", callCount)
	}
}

func TestModalRunEpisodeRemote_CleanupOnError(t *testing.T) {
	deleteCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/apps" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{"app_id": "cleanup-app"})

		case r.URL.Path == "/functions" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error": "deploy failed"}`)

		case r.URL.Path == "/apps/cleanup-app" && r.Method == http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	_, err := mb.RunEpisodeRemote(context.Background(), EnvironmentConfig{
		MaxSteps:      5,
		Timeout:       10 * time.Second,
		SuccessMetric: "exact_match",
	}, "test task")
	if err == nil {
		t.Fatal("expected error from deploy failure")
	}
	if !deleteCalled {
		t.Error("expected DestroyWorkspace cleanup to be called on error")
	}
}

func TestModalExecuteTask_NonZeroExitCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"exit_code":    1,
			"stdout":       "",
			"stderr":       "command not found",
			"duration":     100000000,
			"workspace_id": "ws-err",
		})
	}))
	defer server.Close()

	mb := newTestModalBackend(server)
	result, err := mb.ExecuteTask(context.Background(), "ws-err", TaskSpec{
		Command: "bad-command",
	})
	if err != nil {
		t.Fatalf("ExecuteTask should not error on non-zero exit code: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit_code=1, got %d", result.ExitCode)
	}
	if result.Stderr != "command not found" {
		t.Errorf("expected stderr='command not found', got %q", result.Stderr)
	}
}
