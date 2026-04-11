package services

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

type InternalLogger struct {
	file    *os.File
	logger  *log.Logger
	level   LogLevel
	entries []LogEntry
	mu      sync.Mutex
}

func NewInternalLogger(path string, level LogLevel) (*InternalLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &InternalLogger{
		file:    file,
		logger:  log.New(file, "", log.LstdFlags),
		level:   level,
		entries: make([]LogEntry, 0),
	}, nil
}

func (l *InternalLogger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		LogLevelDebug: 0,
		LogLevelInfo:  1,
		LogLevelWarn:  2,
		LogLevelError: 3,
	}

	return levels[level] >= levels[l.level]
}

func (l *InternalLogger) log(level LogLevel, message string, fields map[string]any) {
	if !l.shouldLog(level) {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}

	l.mu.Lock()
	l.entries = append(l.entries, entry)
	l.mu.Unlock()

	l.logger.Printf("[%s] %s", level, message)
}

func (l *InternalLogger) Debug(message string, fields map[string]any) {
	l.log(LogLevelDebug, message, fields)
}

func (l *InternalLogger) Info(message string, fields map[string]any) {
	l.log(LogLevelInfo, message, fields)
}

func (l *InternalLogger) Warn(message string, fields map[string]any) {
	l.log(LogLevelWarn, message, fields)
}

func (l *InternalLogger) Error(message string, fields map[string]any) {
	l.log(LogLevelError, message, fields)
}

func (l *InternalLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *InternalLogger) GetEntries() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	result := make([]LogEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

func (l *InternalLogger) GetEntriesByLevel(level LogLevel) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []LogEntry
	for _, entry := range l.entries {
		if entry.Level == level {
			result = append(result, entry)
		}
	}
	return result
}

func (l *InternalLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = l.entries[:0]
}

func (l *InternalLogger) Close() error {
	return l.file.Close()
}

func (l *InternalLogger) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		l.file.Close()
	}

	backup := l.file.Name() + "." + time.Now().Format("20060102-150405")
	os.Rename(l.file.Name(), backup)

	file, err := os.OpenFile(l.file.Name(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	l.file = file
	l.logger = log.New(file, "", log.LstdFlags)
	return nil
}

type LogAggregator struct {
	loggers map[string]*InternalLogger
	mu      sync.RWMutex
}

func NewLogAggregator() *LogAggregator {
	return &LogAggregator{
		loggers: make(map[string]*InternalLogger),
	}
}

func (a *LogAggregator) AddLogger(name string, logger *InternalLogger) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.loggers[name] = logger
}

func (a *LogAggregator) GetLogger(name string) (*InternalLogger, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	logger, exists := a.loggers[name]
	return logger, exists
}

func (a *LogAggregator) RemoveLogger(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.loggers, name)
}

func (a *LogAggregator) Broadcast(level LogLevel, message string, fields map[string]any) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, logger := range a.loggers {
		logger.log(level, message, fields)
	}
}

func (a *LogAggregator) CloseAll() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var lastErr error
	for name, logger := range a.loggers {
		if err := logger.Close(); err != nil {
			lastErr = fmt.Errorf("error closing logger %s: %w", name, err)
		}
	}

	return lastErr
}
