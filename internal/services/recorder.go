package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RecordingEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
}

type SessionRecorder struct {
	file      *os.File
	writer    *bufio.Writer
	path      string
	recording bool
	entries   []RecordingEntry
	mu        sync.Mutex
}

func NewSessionRecorder(path string) (*SessionRecorder, error) {
	if path == "" {
		home, _ := os.UserHomeDir()
		dir := filepath.Join(home, ".smartclaw", "recordings")
		os.MkdirAll(dir, 0755)
		path = filepath.Join(dir, fmt.Sprintf("session_%d.jsonl", time.Now().Unix()))
	}

	return &SessionRecorder{
		path:      path,
		entries:   make([]RecordingEntry, 0),
		recording: false,
	}, nil
}

func (r *SessionRecorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording {
		return fmt.Errorf("already recording")
	}

	file, err := os.Create(r.path)
	if err != nil {
		return err
	}

	r.file = file
	r.writer = bufio.NewWriter(file)
	r.recording = true

	return nil
}

func (r *SessionRecorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return nil
	}

	if r.writer != nil {
		r.writer.Flush()
	}
	if r.file != nil {
		r.file.Close()
	}

	r.recording = false
	return nil
}

func (r *SessionRecorder) Record(entryType string, data map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return nil
	}

	entry := RecordingEntry{
		Timestamp: time.Now(),
		Type:      entryType,
		Data:      data,
	}

	r.entries = append(r.entries, entry)

	if r.writer != nil {
		line, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		_, err = r.writer.WriteString(string(line) + "\n")
		return err
	}

	return nil
}

func (r *SessionRecorder) RecordMessage(role, content string) error {
	return r.Record("message", map[string]interface{}{
		"role":    role,
		"content": content,
	})
}

func (r *SessionRecorder) RecordToolCall(tool string, input map[string]interface{}) error {
	return r.Record("tool_call", map[string]interface{}{
		"tool":  tool,
		"input": input,
	})
}

func (r *SessionRecorder) RecordToolResult(tool string, result interface{}) error {
	return r.Record("tool_result", map[string]interface{}{
		"tool":   tool,
		"result": result,
	})
}

func (r *SessionRecorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recording
}

func (r *SessionRecorder) GetPath() string {
	return r.path
}

func (r *SessionRecorder) GetEntries() []RecordingEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]RecordingEntry, len(r.entries))
	copy(result, r.entries)
	return result
}

type Playback struct {
	file    *os.File
	scanner *bufio.Scanner
	entries []RecordingEntry
	index   int
}

func NewPlayback(path string) (*Playback, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &Playback{
		file:    file,
		scanner: bufio.NewScanner(file),
		entries: make([]RecordingEntry, 0),
		index:   0,
	}, nil
}

func (p *Playback) Load() error {
	for p.scanner.Scan() {
		var entry RecordingEntry
		if err := json.Unmarshal([]byte(p.scanner.Text()), &entry); err != nil {
			continue
		}
		p.entries = append(p.entries, entry)
	}
	return p.scanner.Err()
}

func (p *Playback) Next() (*RecordingEntry, bool) {
	if p.index >= len(p.entries) {
		return nil, false
	}
	entry := &p.entries[p.index]
	p.index++
	return entry, true
}

func (p *Playback) Rewind() {
	p.index = 0
}

func (p *Playback) GetAll() []RecordingEntry {
	return p.entries
}

func (p *Playback) Close() error {
	return p.file.Close()
}
