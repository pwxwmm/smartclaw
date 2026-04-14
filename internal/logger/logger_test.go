package logger

import (
	"testing"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger(LevelInfo, nil)
	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}
}

func TestLoggerSetLevel(t *testing.T) {
	logger := NewLogger(LevelInfo, nil)

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
	logger := NewLogger(LevelInfo, nil)
	newLogger := logger.WithField("key", "value")

	if newLogger == nil {
		t.Fatal("Expected non-nil logger")
	}

	found := false
	for _, attr := range newLogger.attrs {
		if attr.Key == "key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected field to be set")
	}
}

func TestLoggerWithFields(t *testing.T) {
	logger := NewLogger(LevelInfo, nil)
	fields := map[string]any{
		"key1": "value1",
		"key2": 42,
	}

	newLogger := logger.WithFields(fields)
	if newLogger == nil {
		t.Fatal("Expected non-nil logger")
	}

	if len(newLogger.attrs) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(newLogger.attrs))
	}
}

func TestLoggerSetPrefix(t *testing.T) {
	logger := NewLogger(LevelInfo, nil)
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
	logger := NewLogger(LevelDebug, nil)
	logger.Debug("test message")
}

func TestLoggerInfo(t *testing.T) {
	logger := NewLogger(LevelInfo, nil)
	logger.Info("test message")
}

func TestLoggerWarn(t *testing.T) {
	logger := NewLogger(LevelWarn, nil)
	logger.Warn("test message")
}

func TestLoggerError(t *testing.T) {
	logger := NewLogger(LevelError, nil)
	logger.Error("test message")
}
