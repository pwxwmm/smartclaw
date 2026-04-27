package process

import (
	"testing"
	"time"
)

func TestGetCurrentPID(t *testing.T) {
	t.Parallel()

	pid := GetCurrentPID()
	if pid <= 0 {
		t.Errorf("GetCurrentPID() = %d, want positive PID", pid)
	}
}

func TestGetPPID(t *testing.T) {
	t.Parallel()

	ppid := GetPPID()
	if ppid < 0 {
		t.Errorf("GetPPID() = %d, want non-negative PPID", ppid)
	}
}

func TestGetGID(t *testing.T) {
	t.Parallel()

	gid := GetGID()
	if gid < 0 {
		t.Errorf("GetGID() = %d, want non-negative GID", gid)
	}
}

func TestGetUID(t *testing.T) {
	t.Parallel()

	uid := GetUID()
	if uid < 0 {
		t.Errorf("GetUID() = %d, want non-negative UID", uid)
	}
}

func TestHostname(t *testing.T) {
	t.Parallel()

	name, err := Hostname()
	if err != nil {
		t.Errorf("Hostname() returned error: %v", err)
	}
	if name == "" {
		t.Error("Hostname() returned empty string")
	}
}

func TestUptime(t *testing.T) {
	t.Parallel()

	uptime := Uptime()
	if uptime < 0 {
		t.Errorf("Uptime() = %v, want non-negative", uptime)
	}
}

func TestUptime_Increases(t *testing.T) {
	first := Uptime()
	time.Sleep(10 * time.Millisecond)
	second := Uptime()
	if second < first {
		t.Errorf("Uptime should increase over time: first=%v, second=%v", first, second)
	}
}

func TestRunProcess_Echo(t *testing.T) {
	output, err := RunProcess("echo", []string{"hello"}, nil, "")
	if err != nil {
		t.Fatalf("RunProcess(\"echo\") returned error: %v", err)
	}
	if output != "hello\n" {
		t.Errorf("RunProcess(\"echo\") = %q, want %q", output, "hello\n")
	}
}

func TestRunProcess_WithArgs(t *testing.T) {
	output, err := RunProcess("printf", []string{"%s %s", "hello", "world"}, nil, "")
	if err != nil {
		t.Fatalf("RunProcess(\"printf\") returned error: %v", err)
	}
	if output != "hello world" {
		t.Errorf("RunProcess(\"printf\") = %q, want %q", output, "hello world")
	}
}

func TestRunProcess_Failure(t *testing.T) {
	_, err := RunProcess("nonexistent_command_12345", []string{}, nil, "")
	if err == nil {
		t.Error("RunProcess with nonexistent command should return error")
	}
}

func TestRunProcess_ExitError(t *testing.T) {
	_, err := RunProcess("false", []string{}, nil, "")
	if err == nil {
		t.Error("RunProcess(\"false\") should return error for non-zero exit code")
	}
}

func TestRunProcessWithTimeout_Success(t *testing.T) {
	output, err := RunProcessWithTimeout("echo", []string{"fast"}, 5*time.Second)
	if err != nil {
		t.Fatalf("RunProcessWithTimeout(\"echo\") returned error: %v", err)
	}
	if output != "fast\n" {
		t.Errorf("RunProcessWithTimeout(\"echo\") = %q, want %q", output, "fast\n")
	}
}

func TestRunProcessWithTimeout_Expired(t *testing.T) {
	_, err := RunProcessWithTimeout("sleep", []string{"10"}, 50*time.Millisecond)
	if err == nil {
		t.Error("RunProcessWithTimeout should return error on timeout")
	}
}

func TestKillProcess_InvalidPID(t *testing.T) {
	err := KillProcess(-1)
	if err == nil {
		t.Error("KillProcess(-1) should return error")
	}
}

func TestListProcesses(t *testing.T) {
	t.Parallel()

	procs := ListProcesses()
	if len(procs) == 0 {
		t.Error("ListProcesses() should return at least one process")
	}
	for i, p := range procs {
		if p.Command != "goroutine" {
			t.Errorf("procs[%d].Command = %q, want %q", i, p.Command, "goroutine")
		}
	}
}

func TestGetEnvVars(t *testing.T) {
	t.Parallel()

	envs := GetEnvVars()
	if len(envs) == 0 {
		t.Error("GetEnvVars() should return at least one env var")
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("TEST_SMARTCLAW_KEY", "test_value")
	got := GetEnv("TEST_SMARTCLAW_KEY")
	if got != "test_value" {
		t.Errorf("GetEnv() = %q, want %q", got, "test_value")
	}
}

func TestGetEnv_NotSet(t *testing.T) {
	got := GetEnv("NONEXISTENT_VAR_12345")
	if got != "" {
		t.Errorf("GetEnv() on unset var = %q, want empty string", got)
	}
}

func TestSetEnv(t *testing.T) {
	if err := SetEnv("TEST_SMARTCLAW_SET", "value123"); err != nil {
		t.Fatalf("SetEnv() returned error: %v", err)
	}
	if got := GetEnv("TEST_SMARTCLAW_SET"); got != "value123" {
		t.Errorf("GetEnv() after SetEnv = %q, want %q", got, "value123")
	}
	UnsetEnv("TEST_SMARTCLAW_SET")
}

func TestUnsetEnv(t *testing.T) {
	SetEnv("TEST_SMARTCLAW_UNSET", "willberemoved")
	if err := UnsetEnv("TEST_SMARTCLAW_UNSET"); err != nil {
		t.Fatalf("UnsetEnv() returned error: %v", err)
	}
	if got := GetEnv("TEST_SMARTCLAW_UNSET"); got != "" {
		t.Errorf("GetEnv() after UnsetEnv = %q, want empty string", got)
	}
}

func TestProcess_Struct(t *testing.T) {
	t.Parallel()

	p := Process{
		PID:     1234,
		Command: "test",
		Args:    []string{"arg1", "arg2"},
		Env:     []string{"KEY=VAL"},
		Dir:     "/tmp",
	}
	if p.PID != 1234 {
		t.Errorf("PID = %d, want 1234", p.PID)
	}
	if p.Command != "test" {
		t.Errorf("Command = %q, want %q", p.Command, "test")
	}
	if len(p.Args) != 2 {
		t.Errorf("Args length = %d, want 2", len(p.Args))
	}
	if p.Dir != "/tmp" {
		t.Errorf("Dir = %q, want %q", p.Dir, "/tmp")
	}
}
