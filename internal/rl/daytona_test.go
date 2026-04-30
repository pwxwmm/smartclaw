package rl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestBackend(t *testing.T, handler http.Handler) *DaytonaBackend {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewDaytonaBackend(server.URL, "test-api-key")
}

func TestNewDaytonaBackend(t *testing.T) {
	db := NewDaytonaBackend("http://localhost:8080", "my-key")
	if db == nil {
		t.Fatal("NewDaytonaBackend returned nil")
	}
	if db.apiURL != "http://localhost:8080" {
		t.Errorf("expected apiURL=http://localhost:8080, got %s", db.apiURL)
	}
	if db.apiKey != "my-key" {
		t.Errorf("expected apiKey=my-key, got %s", db.apiKey)
	}
	if db.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
	if db.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout=30s, got %v", db.httpClient.Timeout)
	}
}

func TestDaytonaName(t *testing.T) {
	db := NewDaytonaBackend("http://localhost", "key")
	if db.Name() != "daytona" {
		t.Errorf("expected name=daytona, got %s", db.Name())
	}
}

func TestDaytonaCreateWorkspace(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/workspace" {
			t.Errorf("expected path /workspace, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Bearer auth header, got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var config WorkspaceConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "ws-123"})
	})

	db := newTestBackend(t, handler)
	wsID, err := db.CreateWorkspace(context.Background(), WorkspaceConfig{
		Name:     "test-ws",
		Image:    "python:3.12",
		CPUCores: 2.0,
		MemoryMB: 4096,
	})
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	if wsID != "ws-123" {
		t.Errorf("expected ws-123, got %s", wsID)
	}
}

func TestDaytonaExecuteTask(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		expectedPath := "/workspace/ws-456/execute"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		var task TaskSpec
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if task.Command != "echo hello" {
			t.Errorf("expected command=echo hello, got %s", task.Command)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TaskResult{
			ExitCode:    0,
			Stdout:      "hello\n",
			Stderr:      "",
			Duration:    500 * time.Millisecond,
			WorkspaceID: "ws-456",
		})
	})

	db := newTestBackend(t, handler)
	result, err := db.ExecuteTask(context.Background(), "ws-456", TaskSpec{
		Command: "echo hello",
		Args:    []string{},
	})
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("expected stdout='hello\\n', got %q", result.Stdout)
	}
	if result.WorkspaceID != "ws-456" {
		t.Errorf("expected workspace_id=ws-456, got %s", result.WorkspaceID)
	}
}

func TestDaytonaDestroyWorkspace(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		expectedPath := "/workspace/ws-789"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	db := newTestBackend(t, handler)
	err := db.DestroyWorkspace(context.Background(), "ws-789")
	if err != nil {
		t.Fatalf("DestroyWorkspace failed: %v", err)
	}
}

func TestDaytonaWorkspaceStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		expectedPath := "/workspace/ws-status"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(WorkspaceStatus{
			ID:        "ws-status",
			State:     "running",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			ExpiresAt: time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC),
		})
	})

	db := newTestBackend(t, handler)
	status, err := db.WorkspaceStatus(context.Background(), "ws-status")
	if err != nil {
		t.Fatalf("WorkspaceStatus failed: %v", err)
	}
	if status.ID != "ws-status" {
		t.Errorf("expected id=ws-status, got %s", status.ID)
	}
	if status.State != "running" {
		t.Errorf("expected state=running, got %s", status.State)
	}
}

func TestDaytonaListWorkspaces(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/workspace" {
			t.Errorf("expected path /workspace, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]WorkspaceInfo{
			{ID: "ws-1", Name: "alpha", State: "running", CreatedAt: time.Now()},
			{ID: "ws-2", Name: "beta", State: "stopped", CreatedAt: time.Now()},
		})
	})

	db := newTestBackend(t, handler)
	workspaces, err := db.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
	if workspaces[0].ID != "ws-1" {
		t.Errorf("expected ws-1, got %s", workspaces[0].ID)
	}
	if workspaces[1].Name != "beta" {
		t.Errorf("expected beta, got %s", workspaces[1].Name)
	}
}

func TestDaytonaErrorHandling(t *testing.T) {
	t.Run("InternalServerError", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "internal error")
		})
		db := newTestBackend(t, handler)

		_, err := db.CreateWorkspace(context.Background(), WorkspaceConfig{Name: "fail"})
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
		var httpErr *httpError
		if !errors.As(err, &httpErr) {
			t.Errorf("expected *httpError in error chain, got %T", err)
		} else {
			if httpErr.StatusCode != 500 {
				t.Errorf("expected status 500, got %d", httpErr.StatusCode)
			}
		}
	})

	t.Run("NotFoundError", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, "not found")
		})
		db := newTestBackend(t, handler)

		_, err := db.WorkspaceStatus(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("expected error for 404 response")
		}
	})

	t.Run("UnauthorizedError", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "unauthorized")
		})
		db := newTestBackend(t, handler)

		_, err := db.ListWorkspaces(context.Background())
		if err == nil {
			t.Fatal("expected error for 401 response")
		}
	})

	t.Run("DestroyOnNon2xx", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "forbidden")
		})
		db := newTestBackend(t, handler)

		err := db.DestroyWorkspace(context.Background(), "ws-x")
		if err == nil {
			t.Fatal("expected error for 403 response")
		}
	})

	t.Run("ExecuteOnNon2xx", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprint(w, "bad gateway")
		})
		db := newTestBackend(t, handler)

		_, err := db.ExecuteTask(context.Background(), "ws-x", TaskSpec{Command: "ls"})
		if err == nil {
			t.Fatal("expected error for 502 response")
		}
	})
}

func TestDaytonaDestroyWorkspace_200OK(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"deleted": true}`)
	})
	db := newTestBackend(t, handler)

	err := db.DestroyWorkspace(context.Background(), "ws-ok")
	if err != nil {
		t.Fatalf("expected no error for 200, got %v", err)
	}
}

func TestDaytonaImplementsServerlessBackend(t *testing.T) {
	var _ ServerlessBackend = (*DaytonaBackend)(nil)
}

func TestDaytonaCreateWorkspace_SendsConfig(t *testing.T) {
	received := make(chan WorkspaceConfig, 1)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var config WorkspaceConfig
		json.NewDecoder(r.Body).Decode(&config)
		received <- config

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "ws-cfg"})
	})

	db := newTestBackend(t, handler)
	cfg := WorkspaceConfig{
		Name:     "test-name",
		Image:    "golang:1.22",
		Env:      map[string]string{"FOO": "bar"},
		CPUCores: 4.0,
		MemoryMB: 8192,
		Labels:   map[string]string{"env": "test"},
	}

	wsID, err := db.CreateWorkspace(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	if wsID != "ws-cfg" {
		t.Errorf("expected ws-cfg, got %s", wsID)
	}

	select {
	case got := <-received:
		if got.Name != "test-name" {
			t.Errorf("expected name=test-name, got %s", got.Name)
		}
		if got.Image != "golang:1.22" {
			t.Errorf("expected image=golang:1.22, got %s", got.Image)
		}
		if got.Env["FOO"] != "bar" {
			t.Errorf("expected env FOO=bar, got %s", got.Env["FOO"])
		}
		if got.CPUCores != 4.0 {
			t.Errorf("expected cpu=4.0, got %f", got.CPUCores)
		}
		if got.MemoryMB != 8192 {
			t.Errorf("expected memory=8192, got %d", got.MemoryMB)
		}
		if got.Labels["env"] != "test" {
			t.Errorf("expected label env=test, got %s", got.Labels["env"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for request")
	}
}

func TestDaytonaExecuteTask_SendsTaskSpec(t *testing.T) {
	received := make(chan TaskSpec, 1)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var task TaskSpec
		json.NewDecoder(r.Body).Decode(&task)
		received <- task

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TaskResult{
			ExitCode: 0,
			Stdout:   "ok",
			Duration: time.Second,
		})
	})

	db := newTestBackend(t, handler)
	spec := TaskSpec{
		Command:   "python",
		Args:      []string{"-c", "print('hi')"},
		WorkDir:   "/app",
		Env:       map[string]string{"DEBUG": "1"},
		InputData: []byte("stdin data"),
	}

	result, err := db.ExecuteTask(context.Background(), "ws-task", spec)
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.WorkspaceID != "ws-task" {
		t.Errorf("expected workspace_id=ws-task, got %s", result.WorkspaceID)
	}

	select {
	case got := <-received:
		if got.Command != "python" {
			t.Errorf("expected command=python, got %s", got.Command)
		}
		if len(got.Args) != 2 || got.Args[0] != "-c" {
			t.Errorf("expected args [-c, print('hi')], got %v", got.Args)
		}
		if got.WorkDir != "/app" {
			t.Errorf("expected workdir=/app, got %s", got.WorkDir)
		}
		if got.Env["DEBUG"] != "1" {
			t.Errorf("expected env DEBUG=1, got %s", got.Env["DEBUG"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for request")
	}
}

func TestHttpErrorFormat(t *testing.T) {
	err := &httpError{StatusCode: 404, Body: "workspace not found"}
	expected := "HTTP 404: workspace not found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestCheckResponse_SuccessCodes(t *testing.T) {
	for _, code := range []int{200, 201, 202, 204, 299} {
		resp := &http.Response{StatusCode: code, Body: http.NoBody}
		if err := checkResponse(resp); err != nil {
			t.Errorf("expected no error for status %d, got %v", code, err)
		}
	}
}

func TestCheckResponse_ErrorCodes(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404, 500, 502, 503} {
		resp := &http.Response{StatusCode: code, Body: http.NoBody}
		if err := checkResponse(resp); err == nil {
			t.Errorf("expected error for status %d", code)
		}
	}
}

func TestDaytonaCreateWorkspace_CancelledContext(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	db := newTestBackend(t, handler)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := db.CreateWorkspace(ctx, WorkspaceConfig{Name: "cancel"})
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestDaytonaDestroyWorkspace_CancelledContext(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusNoContent)
	})

	db := newTestBackend(t, handler)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := db.DestroyWorkspace(ctx, "ws-cancel")
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}
