package observability

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AuditEntry represents a single auditable event.
type AuditEntry struct {
	ID              string         `json:"id"`
	Timestamp       time.Time      `json:"timestamp"`
	Type            string         `json:"type"` // tool_execution, tool_approval, tool_denial, command, chat
	Actor           string         `json:"actor"`
	Tool            string         `json:"tool,omitempty"`
	Input           map[string]any `json:"input,omitempty"`
	Output          string         `json:"output,omitempty"`
	OutputTruncated bool           `json:"output_truncated"`
	Duration        time.Duration  `json:"duration"`
	Success         bool           `json:"success"`
	Error           string         `json:"error,omitempty"`
	Approved        *bool          `json:"approved,omitempty"` // nil if no approval needed
	ApprovedBy      string         `json:"approved_by,omitempty"`
	SessionID       string         `json:"session_id,omitempty"`
	ClientID        string         `json:"client_id,omitempty"`
	Model           string         `json:"model,omitempty"`
	Reason          string         `json:"reason,omitempty"`
}

// AuditFilter defines filters for querying audit entries.
type AuditFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	Type      string
	Tool      string
	Actor     string
	Success   *bool
}

// AuditStats holds aggregate statistics about audit entries.
type AuditStats struct {
	TotalEntries int64
	ByType       map[string]int64
	ByTool       map[string]int64
	ApprovalRate float64 // % of approval-needed actions that were approved
	ErrorRate    float64
}

// AuditLogger provides append-only audit trail logging to a JSONL file.
type AuditLogger struct {
	mu         sync.Mutex
	entries    []*AuditEntry
	file       *os.File
	encoder    *json.Encoder
	maxEntries int // max in-memory entries
}

// DefaultAuditLogger is the global audit logger instance.
var DefaultAuditLogger *AuditLogger

var auditMu sync.RWMutex

func getAuditLogger() *AuditLogger {
	auditMu.RLock()
	defer auditMu.RUnlock()
	return DefaultAuditLogger
}

// NewAuditLogger creates a new AuditLogger that writes to dataDir/audit.jsonl.
// The dataDir is typically ~/.smartclaw.
func NewAuditLogger(dataDir string) (*AuditLogger, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("audit: failed to create data directory: %w", err)
	}

	path := filepath.Join(dataDir, "audit.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("audit: failed to open audit file: %w", err)
	}

	return &AuditLogger{
		entries:    make([]*AuditEntry, 0),
		file:       f,
		encoder:    json.NewEncoder(f),
		maxEntries: 10000,
	}, nil
}

// Log appends an entry to the JSONL file and in-memory buffer.
// The JSONL write is synchronous for durability.
func (l *AuditLogger) Log(entry *AuditEntry) {
	if entry.ID == "" {
		entry.ID = generateID()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Append to JSONL file (synchronous for durability)
	if l.encoder != nil {
		if err := l.encoder.Encode(entry); err != nil {
			slog.Warn("failed to encode audit entry", "error", err)
		}
	}

	// Append to in-memory buffer
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxEntries {
		// Trim oldest entries to stay within limit
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}
}

// Query returns audit entries matching the given filters.
func (l *AuditLogger) Query(filters AuditFilter) ([]*AuditEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var results []*AuditEntry
	for _, e := range l.entries {
		if !matchesFilter(e, filters) {
			continue
		}
		results = append(results, e)
	}
	return results, nil
}

// Recent returns the most recent audit entries up to the given limit.
func (l *AuditLogger) Recent(limit int) ([]*AuditEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	n := limit
	if n > len(l.entries) {
		n = len(l.entries)
	}
	if n <= 0 {
		return []*AuditEntry{}, nil
	}

	// Return most recent entries
	start := len(l.entries) - n
	results := make([]*AuditEntry, n)
	copy(results, l.entries[start:])
	return results, nil
}

// Stats returns aggregate statistics about the audit log.
func (l *AuditLogger) Stats() AuditStats {
	l.mu.Lock()
	defer l.mu.Unlock()

	stats := AuditStats{
		TotalEntries: int64(len(l.entries)),
		ByType:       make(map[string]int64),
		ByTool:       make(map[string]int64),
	}

	var errorCount int64
	var approvalNeeded int64
	var approvalGranted int64

	for _, e := range l.entries {
		stats.ByType[e.Type]++
		if e.Tool != "" {
			stats.ByTool[e.Tool]++
		}
		if !e.Success {
			errorCount++
		}
		if e.Approved != nil {
			approvalNeeded++
			if *e.Approved {
				approvalGranted++
			}
		}
	}

	if stats.TotalEntries > 0 {
		stats.ErrorRate = float64(errorCount) / float64(stats.TotalEntries) * 100
	}
	if approvalNeeded > 0 {
		stats.ApprovalRate = float64(approvalGranted) / float64(approvalNeeded) * 100
	}

	return stats
}

// Close closes the audit log file.
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// matchesFilter checks if an entry matches the given filter criteria.
func matchesFilter(e *AuditEntry, f AuditFilter) bool {
	if f.StartTime != nil && e.Timestamp.Before(*f.StartTime) {
		return false
	}
	if f.EndTime != nil && e.Timestamp.After(*f.EndTime) {
		return false
	}
	if f.Type != "" && e.Type != f.Type {
		return false
	}
	if f.Tool != "" && e.Tool != f.Tool {
		return false
	}
	if f.Actor != "" && e.Actor != f.Actor {
		return false
	}
	if f.Success != nil && e.Success != *f.Success {
		return false
	}
	return true
}

// generateID creates a unique identifier for audit entries.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// truncate caps a string at maxLen characters and reports whether truncation occurred.
func truncate(s string, maxLen int) (string, bool) {
	if len(s) <= maxLen {
		return s, false
	}
	return s[:maxLen], true
}

// sensitiveKeys are input keys whose values should be redacted in the audit log.
var sensitiveKeys = map[string]bool{
	"api_key":       true,
	"apikey":        true,
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"password":      true,
	"secret":        true,
	"secret_key":    true,
	"private_key":   true,
	"credentials":   true,
	"authorization": true,
}

// sanitizeInput removes or truncates sensitive data from tool inputs.
func sanitizeInput(tool string, input map[string]any) map[string]any {
	if input == nil {
		return nil
	}

	sanitized := make(map[string]any, len(input))
	for k, v := range input {
		lower := strings.ToLower(k)

		// Redact sensitive keys
		if sensitiveKeys[lower] {
			sanitized[k] = "***REDACTED***"
			continue
		}

		// Tool-specific sanitization
		switch tool {
		case "write_file", "edit_file":
			if lower == "content" {
				if s, ok := v.(string); ok {
					if len(s) > 100 {
						sanitized[k] = s[:100] + "..."
					} else {
						sanitized[k] = s
					}
					continue
				}
			}
		}

		sanitized[k] = v
	}
	return sanitized
}

// InitAudit initializes the global audit logger with the given data directory.
func InitAudit(dataDir string) error {
	logger, err := NewAuditLogger(dataDir)
	if err != nil {
		return err
	}
	auditMu.Lock()
	defer auditMu.Unlock()
	DefaultAuditLogger = logger
	return nil
}

// AuditLog logs an entry to the default audit logger.
func AuditLog(entry *AuditEntry) {
	if l := getAuditLogger(); l != nil {
		l.Log(entry)
	}
}

// AuditToolExecution records a tool execution in the audit log.
// This is a convenience function that constructs the AuditEntry.
func AuditToolExecution(tool string, input map[string]any, output string, duration time.Duration, success bool, err error) {
	truncatedOutput, wasTruncated := truncate(output, 10000)

	entry := &AuditEntry{
		ID:              generateID(),
		Timestamp:       time.Now(),
		Type:            "tool_execution",
		Actor:           "agent",
		Tool:            tool,
		Input:           sanitizeInput(tool, input),
		Output:          truncatedOutput,
		OutputTruncated: wasTruncated,
		Duration:        duration,
		Success:         success,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	AuditLog(entry)
}

// AuditApproval records a tool approval or denial in the audit log.
func AuditApproval(tool string, blockID string, approved bool, approvedBy string) {
	AuditLog(&AuditEntry{
		ID:         generateID(),
		Timestamp:  time.Now(),
		Type:       "tool_approval",
		Actor:      approvedBy,
		Tool:       tool,
		Approved:   &approved,
		ApprovedBy: approvedBy,
		Reason:     blockID,
	})
}

// AuditDenial records a tool denial in the audit log.
func AuditDenial(tool string, blockID string, deniedBy string, reason string) {
	approved := false
	AuditLog(&AuditEntry{
		ID:         generateID(),
		Timestamp:  time.Now(),
		Type:       "tool_denial",
		Actor:      deniedBy,
		Tool:       tool,
		Approved:   &approved,
		ApprovedBy: deniedBy,
		Reason:     reason,
	})
}

// AuditCommand records a command execution in the audit log.
func AuditCommand(command string, actor string, success bool, err error) {
	entry := &AuditEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Type:      "command",
		Actor:     actor,
		Tool:      "command",
		Output:    command,
		Success:   success,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	AuditLog(entry)
}
