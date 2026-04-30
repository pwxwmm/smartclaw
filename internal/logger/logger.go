package logger

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func (l Level) slogLevel() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type Logger struct {
	mu     sync.Mutex
	level  Level
	attrs  []slog.Attr
	prefix string
}

var defaultLogger *Logger

func init() {
	defaultLogger = NewLogger(LevelInfo, os.Stderr)
}

func NewLogger(level Level, _ interface{}) *Logger {
	return &Logger{
		level: level,
	}
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) SetOutput(_ interface{}) {
	// Output is managed by slog's default handler
}

func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

func (l *Logger) WithField(key string, value any) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:  l.level,
		prefix: l.prefix,
		attrs:  make([]slog.Attr, len(l.attrs)),
	}
	copy(newLogger.attrs, l.attrs)
	newLogger.attrs = append(newLogger.attrs, slog.Any(key, value))

	return newLogger
}

func (l *Logger) WithFields(fields map[string]any) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:  l.level,
		prefix: l.prefix,
		attrs:  make([]slog.Attr, len(l.attrs)),
	}
	copy(newLogger.attrs, l.attrs)
	for k, v := range fields {
		newLogger.attrs = append(newLogger.attrs, slog.Any(k, v))
	}

	return newLogger
}

func (l *Logger) log(level Level, format string, args ...any) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	prefix := l.prefix
	attrs := make([]slog.Attr, len(l.attrs))
	copy(attrs, l.attrs)
	l.mu.Unlock()

	msg := format
	if len(args) > 0 {
		msg = sprintf(format, args...)
	}

	if prefix != "" {
		msg = prefix + " " + msg
	}

	switch level {
	case LevelDebug:
		slog.LogAttrs(nil, slog.LevelDebug, msg, attrs...)
	case LevelInfo:
		slog.LogAttrs(nil, slog.LevelInfo, msg, attrs...)
	case LevelWarn:
		slog.LogAttrs(nil, slog.LevelWarn, msg, attrs...)
	case LevelError:
		slog.LogAttrs(nil, slog.LevelError, msg, attrs...)
	}
}

func (l *Logger) Debug(format string, args ...any) {
	l.log(LevelDebug, format, args...)
}

func (l *Logger) Info(format string, args ...any) {
	l.log(LevelInfo, format, args...)
}

func (l *Logger) Warn(format string, args ...any) {
	l.log(LevelWarn, format, args...)
}

func (l *Logger) Error(format string, args ...any) {
	l.log(LevelError, format, args...)
}

func (l *Logger) Fatal(format string, args ...any) {
	l.log(LevelError, format, args...)
	panic(sprintf(format, args...))
}

func (l *Logger) Panic(format string, args ...any) {
	l.log(LevelError, format, args...)
	panic(sprintf(format, args...))
}

func SetLevel(level Level) {
	defaultLogger.SetLevel(level)
}

func SetOutput(output interface{}) {
	defaultLogger.SetOutput(output)
}

func SetPrefix(prefix string) {
	defaultLogger.SetPrefix(prefix)
}

func WithField(key string, value any) *Logger {
	return defaultLogger.WithField(key, value)
}

func WithFields(fields map[string]any) *Logger {
	return defaultLogger.WithFields(fields)
}

func Debug(format string, args ...any) {
	defaultLogger.Debug(format, args...)
}

func Info(format string, args ...any) {
	defaultLogger.Info(format, args...)
}

func Warn(format string, args ...any) {
	defaultLogger.Warn(format, args...)
}

func Error(format string, args ...any) {
	defaultLogger.Error(format, args...)
}

func Fatal(format string, args ...any) {
	defaultLogger.Fatal(format, args...)
}

func Panic(format string, args ...any) {
	defaultLogger.Panic(format, args...)
}

func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
