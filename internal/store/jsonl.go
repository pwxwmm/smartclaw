package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type JSONLWriter struct {
	dir string
}

func NewJSONLWriter(dir string) *JSONLWriter {
	return &JSONLWriter{dir: dir}
}

type JSONLEntry struct {
	ID           int64     `json:"id"`
	SessionID    string    `json:"session_id"`
	Role         string    `json:"role"`
	Content      string    `json:"content"`
	Tokens       int       `json:"tokens"`
	ToolCalls    string    `json:"tool_calls,omitempty"`
	ToolName     string    `json:"tool_name,omitempty"`
	FinishReason string    `json:"finish_reason,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

func (w *JSONLWriter) Append(msg *Message) error {
	if w.dir == "" {
		return nil
	}

	if err := os.MkdirAll(w.dir, 0755); err != nil {
		return fmt.Errorf("jsonl: mkdir: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.jsonl", msg.SessionID, time.Now().Format("20060102"))
	path := filepath.Join(w.dir, filename)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("jsonl: open: %w", err)
	}
	defer f.Close()

	entry := JSONLEntry{
		ID:           msg.ID,
		SessionID:    msg.SessionID,
		Role:         msg.Role,
		Content:      msg.Content,
		Tokens:       msg.Tokens,
		ToolCalls:    msg.ToolCalls,
		ToolName:     msg.ToolName,
		FinishReason: msg.FinishReason,
		Timestamp:    msg.Timestamp,
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("jsonl: marshal: %w", err)
	}

	writer := bufio.NewWriter(f)
	if _, err := writer.WriteString(string(line) + "\n"); err != nil {
		return fmt.Errorf("jsonl: write: %w", err)
	}

	return writer.Flush()
}
