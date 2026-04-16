package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

type Process struct {
	PID     int
	Command string
	Args    []string
	Env     []string
	Dir     string
}

func GetCurrentPID() int {
	return os.Getpid()
}

func GetPPID() int {
	return os.Getppid()
}

func GetGID() int {
	return syscall.Getgid()
}

func GetUID() int {
	return syscall.Getuid()
}

func Hostname() (string, error) {
	return os.Hostname()
}

func Uptime() time.Duration {
	return time.Since(startTime)
}

var startTime = time.Now()

func RunProcess(name string, args []string, env []string, dir string) (string, error) {
	cmd := exec.Command(name, args...)
	if len(env) > 0 {
		cmd.Env = env
	}
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, output)
	}
	return string(output), nil
}

func RunProcessWithTimeout(name string, args []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("process timed out")
	}
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, output)
	}
	return string(output), nil
}

func KillProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}

func ListProcesses() []Process {
	procs := runtime.GOMAXPROCS(0)
	processes := make([]Process, 0)
	for i := 0; i < procs; i++ {
		processes = append(processes, Process{
			PID:     i,
			Command: "goroutine",
		})
	}
	return processes
}

func GetEnvVars() []string {
	return os.Environ()
}

func GetEnv(key string) string {
	return os.Getenv(key)
}

func SetEnv(key, value string) error {
	return os.Setenv(key, value)
}

func UnsetEnv(key string) error {
	return os.Unsetenv(key)
}
