package logger

import (
	"bytes"
	"testing"
)

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelInfo, &buf)
	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}
}

func TestLoggerSetLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelInfo, &buf)

	logger.SetLevel(LevelDebug)
	if logger.level != LevelDebug {
		t.Errorf("Expected level %d, got %d", LevelDebug, logger.level)
	}
}

func TestLoggerLevels(t *testing.T) {
	levels := []Level{LevelDebug, LevelInfo, LevelWarn, LevelError}

	for _, level := range levels {
		if level < 0 {
			t.Errorf("Invalid level: %d", level)
		}
	}
}

func TestLoggerWithField(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelInfo, &buf)
	newLogger := logger.WithField("key", "value")

	if newLogger == nil {
		t.Fatal("Expected non-nil logger")
	}

	if newLogger.fields["key"] != "value" {
		t.Error("Expected field to be set")
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelInfo, &buf)
	fields := map[string]any{
		"key1": "value1",
		"key2": 42,
	}

	newLogger := logger.WithFields(fields)
	if newLogger == nil {
		t.Fatal("Expected non-nil logger")
	}

	if len(newLogger.fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(newLogger.fields))
	}
}

func TestLoggerSetPrefix(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelInfo, &buf)
	logger.SetPrefix("test")

	if logger.prefix != "test" {
		t.Errorf("Expected prefix 'test', got '%s'", logger.prefix)
	}
}

func TestSetLevel(t *testing.T) {
	SetLevel(LevelDebug)
	if defaultLogger.level != LevelDebug {
		t.Error("Expected default logger level to be set")
	}
}

func TestLevelStrings(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
	}

	for _, test := range tests {
		if test.level.String() != test.expected {
			t.Errorf("Expected '%s', got '%s'", test.expected, test.level.String())
		}
	}
}

func TestLoggerDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelDebug, &buf)

	logger.Debug("test message")
	if buf.Len() == 0 {
		t.Error("Expected output for debug message")
	}
}

func TestLoggerInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelInfo, &buf)

	logger.Info("test message")
	if buf.Len() == 0 {
		t.Error("Expected output for info message")
	}
}

func TestLoggerWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelWarn, &buf)

	logger.Warn("test message")
	if buf.Len() == 0 {
		t.Error("Expected output for warn message")
	}
}

func TestLoggerError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelError, &buf)

	logger.Error("test message")
	if buf.Len() == 0 {
		t.Error("Expected output for error message")
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelWarn, &buf)

	logger.Debug("debug message")
	logger.Info("info message")

	if buf.Len() > 0 {
		t.Error("Expected no output for debug/info when level is Warn")
	}
}
