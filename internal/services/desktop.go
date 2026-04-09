package services

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

type DesktopNotifier struct {
	enabled bool
	sound   bool
	mu      sync.Mutex
}

func NewDesktopNotifier() *DesktopNotifier {
	return &DesktopNotifier{
		enabled: true,
		sound:   true,
	}
}

func (n *DesktopNotifier) Enable() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enabled = true
}

func (n *DesktopNotifier) Disable() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enabled = false
}

func (n *DesktopNotifier) SetSound(enabled bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sound = enabled
}

func (n *DesktopNotifier) Notify(title, message string) error {
	n.mu.Lock()
	enabled := n.enabled
	sound := n.sound
	n.mu.Unlock()

	if !enabled {
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		return n.notifyMacOS(title, message, sound)
	case "linux":
		return n.notifyLinux(title, message, sound)
	case "windows":
		return n.notifyWindows(title, message)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func (n *DesktopNotifier) notifyMacOS(title, message string, sound bool) error {
	script := fmt.Sprintf(`display notification "%s" with title "%s"`,
		escapeForAppleScript(message),
		escapeForAppleScript(title))

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return err
	}

	if sound {
		exec.Command("afplay", "/System/Library/Sounds/Glass.aiff").Run()
	}

	return nil
}

func (n *DesktopNotifier) notifyLinux(title, message string, sound bool) error {
	args := []string{}
	if sound {
		args = append(args, "--sound")
	}
	args = append(args, title, message)

	cmd := exec.Command("notify-send", args...)
	return cmd.Run()
}

func (n *DesktopNotifier) notifyWindows(title, message string) error {
	script := fmt.Sprintf(
		`[System.Windows.Forms.MessageBox]::Show('%s', '%s')`,
		escapeForPowerShell(message),
		escapeForPowerShell(title),
	)

	cmd := exec.Command("powershell", "-Command",
		"Add-Type -AssemblyName System.Windows.Forms; "+script)
	return cmd.Run()
}

func escapeForAppleScript(s string) string {
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `\`, `\\`)
	return s
}

func escapeForPowerShell(s string) string {
	s = strings.ReplaceAll(s, `'`, `''`)
	return s
}

func (n *DesktopNotifier) Success(message string) error {
	return n.Notify("✓ Success", message)
}

func (n *DesktopNotifier) Error(message string) error {
	return n.Notify("✗ Error", message)
}

func (n *DesktopNotifier) Warning(message string) error {
	return n.Notify("⚠ Warning", message)
}

func (n *DesktopNotifier) Info(message string) error {
	return n.Notify("ℹ Info", message)
}

type SleepManager struct {
	prevented bool
	cmd       *exec.Cmd
	mu        sync.Mutex
}

func NewSleepManager() *SleepManager {
	return &SleepManager{
		prevented: false,
	}
}

func (s *SleepManager) PreventSleep() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.prevented {
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("caffeinate", "-d", "-i")
		if err := cmd.Start(); err != nil {
			return err
		}
		s.cmd = cmd
	case "linux":
		cmd := exec.Command("systemd-inhibit", "--what=sleep", "sleep", "infinity")
		cmd.Start()
		s.cmd = cmd
	case "windows":
		exec.Command("powercfg", "/change", "standby-timeout-ac", "0").Run()
		exec.Command("powercfg", "/change", "standby-timeout-dc", "0").Run()
	}

	s.prevented = true
	return nil
}

func (s *SleepManager) AllowSleep() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.prevented {
		return nil
	}

	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd = nil
	}

	if runtime.GOOS == "windows" {
		exec.Command("powercfg", "/change", "standby-timeout-ac", "30").Run()
		exec.Command("powercfg", "/change", "standby-timeout-dc", "15").Run()
	}

	s.prevented = false
	return nil
}

func (s *SleepManager) IsSleepPrevented() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prevented
}
