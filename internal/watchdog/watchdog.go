package watchdog

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/alertengine"
)

type ErrorPattern struct {
	Regex    *regexp.Regexp
	Severity string // "critical", "high", "medium", "low", "info"
	Source   string // "go_build", "go_test", "go_lint", "runtime"
}

type ProcessWatch struct {
	ID        string    `json:"id"`
	Cmd       *exec.Cmd `json:"-"`
	StartedAt time.Time `json:"started_at"`
	LastError string    `json:"last_error"`
}

type DebugSuggestion struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Command string `json:"command"`
}

type DetectedError struct {
	Line            string           `json:"line"`
	Source          string           `json:"source"`
	Severity        string           `json:"severity"`
	File            string           `json:"file,omitempty"`
	LineNum         int              `json:"line_num,omitempty"`
	ColNum          int              `json:"col_num,omitempty"`
	Message         string           `json:"message,omitempty"`
	Timestamp       time.Time        `json:"timestamp"`
	DebugSuggestion *DebugSuggestion `json:"debug_suggestion,omitempty"`
}

type WatchdogStatus struct {
	Enabled         bool            `json:"enabled"`
	ActiveWatches   []ProcessWatch  `json:"active_watches"`
	RecentErrors    []DetectedError `json:"recent_errors"`
	ErrorCountToday int             `json:"error_count_today"`
}

const maxRecentErrors = 50

// Watchdog monitors process output for error patterns and auto-triggers
// the alert→remediation→verify pipeline.
type Watchdog struct {
	alertEngine   *alertengine.AlertEngine
	patterns      []ErrorPattern
	mu            sync.Mutex
	activeWatches map[string]*ProcessWatch
	enabled       bool
	recentErrors  []DetectedError
	errorCountDay int
	dayStart      time.Time
}

// NewWatchdog creates a Watchdog with default error patterns derived from
// internal/verifyfix/parser.go regexes.
func NewWatchdog(ae *alertengine.AlertEngine) *Watchdog {
	w := &Watchdog{
		alertEngine:   ae,
		activeWatches: make(map[string]*ProcessWatch),
		recentErrors:  make([]DetectedError, 0, maxRecentErrors),
		dayStart:      time.Now().Truncate(24 * time.Hour),
	}
	w.patterns = defaultPatterns()
	return w
}

func defaultPatterns() []ErrorPattern {
	return []ErrorPattern{
		{
			// file.go:line:col: message or file.go:line: message
			Regex:    regexp.MustCompile(`^([\w./-]+\.go):(\d+)(?::(\d+))?:\s+(.+)$`),
			Severity: "high",
			Source:   "go_build",
		},
		{
			// --- FAIL: TestName
			Regex:    regexp.MustCompile(`^--- FAIL:\s+(\S+)`),
			Severity: "high",
			Source:   "go_test",
		},
		{
			// file.go:line:col: message (lint output)
			Regex:    regexp.MustCompile(`^([\w./-]+\.go):(\d+):(\d+):\s+(.+)$`),
			Severity: "medium",
			Source:   "go_lint",
		},
		{
			// panic: <message>
			Regex:    regexp.MustCompile(`^panic:\s+(.+)$`),
			Severity: "critical",
			Source:   "runtime",
		},
	}
}

func (w *Watchdog) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.enabled = true
	slog.Info("watchdog: started")
}

func (w *Watchdog) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.enabled = false
	slog.Info("watchdog: stopped")
}

func (w *Watchdog) IsEnabled() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.enabled
}

// WatchCommand starts a subprocess with piped stdout/stderr, scanning output
// line-by-line against error patterns. MVP: uses exec.Cmd pipes.
func (w *Watchdog) WatchCommand(name string, cmd *exec.Cmd) error {
	w.mu.Lock()
	if !w.enabled {
		w.mu.Unlock()
		return fmt.Errorf("watchdog is not enabled")
	}
	w.mu.Unlock()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	pw := &ProcessWatch{
		ID:        name,
		Cmd:       cmd,
		StartedAt: time.Now(),
	}

	w.mu.Lock()
	w.activeWatches[name] = pw
	w.mu.Unlock()

	if err := cmd.Start(); err != nil {
		w.mu.Lock()
		delete(w.activeWatches, name)
		w.mu.Unlock()
		return fmt.Errorf("command start: %w", err)
	}

	go w.scanOutput(stdout, name)
	go w.scanOutput(stderr, name)

	go func() {
		_ = cmd.Wait()
		w.mu.Lock()
		delete(w.activeWatches, name)
		w.mu.Unlock()
	}()

	return nil
}

func (w *Watchdog) scanOutput(r io.Reader, source string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		w.OnOutputLine(scanner.Text(), source)
	}
}

// OnOutputLine matches a line against all error patterns; on match,
// records the error and creates an alert.
func (w *Watchdog) OnOutputLine(line string, source string) {
	if line == "" {
		return
	}

	w.mu.Lock()
	enabled := w.enabled
	w.mu.Unlock()

	if !enabled {
		return
	}

	for _, pattern := range w.patterns {
		matches := pattern.Regex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		de := DetectedError{
			Line:      line,
			Source:    pattern.Source,
			Severity:  pattern.Severity,
			Timestamp: time.Now(),
		}

		switch pattern.Source {
		case "go_build":
			if len(matches) >= 5 {
				de.File = matches[1]
				de.Message = matches[4]
				if n, err := strconv.Atoi(matches[2]); err == nil {
					de.LineNum = n
				}
			} else if len(matches) >= 3 {
				de.File = matches[1]
				de.Message = matches[2]
				if n, err := strconv.Atoi(matches[2]); err == nil {
					de.LineNum = n
				}
			}
		case "go_test":
			if len(matches) >= 2 {
				de.Message = "test failed: " + matches[1]
			}
		case "go_lint":
			if len(matches) >= 5 {
				de.File = matches[1]
				de.Message = matches[4]
				if n, err := strconv.Atoi(matches[2]); err == nil {
					de.LineNum = n
				}
				if len(matches) >= 6 {
					if n, err := strconv.Atoi(matches[3]); err == nil {
						de.ColNum = n
					}
				}
			}
		case "runtime":
			if len(matches) >= 2 {
				de.Message = "panic: " + matches[1]
			}
		}

		if (de.Severity == "high" || de.Severity == "critical") && de.File != "" {
			de.DebugSuggestion = &DebugSuggestion{
				File:    de.File,
				Line:    de.LineNum,
				Command: fmt.Sprintf(`/tools dap_start {"program":"%s"}`, de.File),
			}
		}

		w.recordError(de)
		w.createAlert(de)
		break
	}
}

func (w *Watchdog) recordError(de DetectedError) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	if !w.dayStart.Equal(today) {
		w.errorCountDay = 0
		w.dayStart = today
	}

	w.errorCountDay++

	if len(w.recentErrors) >= maxRecentErrors {
		w.recentErrors = w.recentErrors[1:]
	}
	w.recentErrors = append(w.recentErrors, de)

	for _, pw := range w.activeWatches {
		if de.Source == pw.ID || de.File != "" {
			pw.LastError = de.Line
		}
	}
}

func (w *Watchdog) createAlert(de DetectedError) {
	if w.alertEngine == nil {
		return
	}

	name := "watchdog_" + de.Source
	if de.Message != "" {
		name += ": " + de.Message
		if len(name) > 200 {
			name = name[:200]
		}
	}

	alert := alertengine.Alert{
		Source:   "watchdog",
		Name:     name,
		Severity: de.Severity,
		Service:  de.File,
		Status:   "firing",
		Labels: map[string]string{
			"source":   de.Source,
			"file":     de.File,
			"watchdog": "true",
		},
		Annotations: map[string]string{
			"line":      de.Line,
			"message":   de.Message,
			"timestamp": de.Timestamp.Format(time.RFC3339),
		},
		FiredAt: de.Timestamp,
	}

	w.alertEngine.Ingest(alert)
}

func (w *Watchdog) GetStatus() WatchdogStatus {
	w.mu.Lock()
	defer w.mu.Unlock()

	watches := make([]ProcessWatch, 0, len(w.activeWatches))
	for _, pw := range w.activeWatches {
		watches = append(watches, *pw)
	}

	errors := make([]DetectedError, len(w.recentErrors))
	copy(errors, w.recentErrors)

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	count := w.errorCountDay
	if !w.dayStart.Equal(today) {
		count = 0
	}

	return WatchdogStatus{
		Enabled:         w.enabled,
		ActiveWatches:   watches,
		RecentErrors:    errors,
		ErrorCountToday: count,
	}
}

func (w *Watchdog) GetRecentErrors(n int) []DetectedError {
	w.mu.Lock()
	defer w.mu.Unlock()

	if n > len(w.recentErrors) {
		n = len(w.recentErrors)
	}
	result := make([]DetectedError, n)
	copy(result, w.recentErrors[len(w.recentErrors)-n:])
	return result
}
