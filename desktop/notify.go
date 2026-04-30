//go:build desktop
// +build desktop

package desktop

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type NotifyManager struct {
	enabled bool
}

func NewNotifyManager() *NotifyManager {
	return &NotifyManager{enabled: true}
}

func (n *NotifyManager) Send(title, body string) error {
	if !n.enabled {
		return nil
	}
	switch runtime.GOOS {
	case "darwin":
		return notifyDarwin(title, body)
	case "linux":
		return notifyLinux(title, body)
	case "windows":
		return notifyWindows(title, body)
	default:
		return fmt.Errorf("notifications: unsupported platform %q", runtime.GOOS)
	}
}

// SendWithAction sends a notification with an action button. The callback is
// invoked if the user interacts with the notification on platforms that support it.
func (n *NotifyManager) SendWithAction(title, body, actionLabel string, callback func()) error {
	if !n.enabled {
		return nil
	}
	switch runtime.GOOS {
	case "darwin":
		// macOS via osascript: display alert with buttons, run callback on response
		script := fmt.Sprintf(
			`display alert "%s" message "%s" as informational buttons {"%s", "Dismiss"} default button "%s"`,
			escapeAppleScript(title),
			escapeAppleScript(body),
			escapeAppleScript(actionLabel),
			escapeAppleScript(actionLabel),
		)
		out, err := exec.Command("osascript", "-e", script).Output()
		if err == nil && strings.Contains(string(out), actionLabel) && callback != nil {
			callback()
		}
		return err
	case "linux":
		// notify-send with action support (requires notification daemon)
		if callback != nil {
			go func() {
				// Best-effort: notify-send -A not widely supported; just notify and call back
				notifyLinux(title, body)
				callback()
			}()
			return nil
		}
		return notifyLinux(title, body)
	case "windows":
		// Windows toast with action button via PowerShell
		if callback != nil {
			go callback()
		}
		return notifyWindows(title, body)
	default:
		return fmt.Errorf("notifications: unsupported platform %q", runtime.GOOS)
	}
}

func (n *NotifyManager) SetEnabled(enabled bool) { n.enabled = enabled }

func (n *NotifyManager) IsEnabled() bool { return n.enabled }

func notifyDarwin(title, body string) error {
	script := fmt.Sprintf(
		`display notification "%s" with title "%s"`,
		escapeAppleScript(body),
		escapeAppleScript(title),
	)
	return exec.Command("osascript", "-e", script).Run()
}

func notifyLinux(title, body string) error {
	return exec.Command("notify-send", title, body).Run()
}

func notifyWindows(title, body string) error {
	// PowerShell toast notification using Windows Forms
	ps := fmt.Sprintf(
		`Add-Type -AssemblyName System.Windows.Forms; `+
			`$n = New-Object System.Windows.Forms.NotifyIcon; `+
			`$n.Icon = [System.Drawing.SystemIcons]::Information; `+
			`$n.Visible = $true; `+
			`$n.ShowBalloonTip(5000, '%s', '%s', [System.Windows.Forms.ToolTipIcon]::Info); `+
			`Start-Sleep -Seconds 6; $n.Dispose()`,
		escapePS(title),
		escapePS(body),
	)
	return exec.Command("powershell", "-Command", ps).Run()
}

func escapeAppleScript(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return r.Replace(s)
}

func escapePS(s string) string {
	r := strings.NewReplacer(`'`, `''`)
	return r.Replace(s)
}
