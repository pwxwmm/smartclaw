package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
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

type Logger struct {
	mu     sync.Mutex
	level  Level
	output io.Writer
	prefix string
	fields map[string]any
}

var defaultLogger *Logger

func init() {
	defaultLogger = NewLogger(LevelInfo, os.Stderr)
}

func NewLogger(level Level, output io.Writer) *Logger {
	return &Logger{
		level:  level,
		output: output,
		fields: make(map[string]any),
	}
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) SetOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = output
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
		output: l.output,
		prefix: l.prefix,
		fields: make(map[string]any),
	}

	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value

	return newLogger
}

func (l *Logger) WithFields(fields map[string]any) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:  l.level,
		output: l.output,
		prefix: l.prefix,
		fields: make(map[string]any),
	}

	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

func (l *Logger) log(level Level, format string, args ...any) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)

	var fieldsStr string
	if len(l.fields) > 0 {
		fieldsStr = " ["
		first := true
		for k, v := range l.fields {
			if !first {
				fieldsStr += " "
			}
			fieldsStr += fmt.Sprintf("%s=%v", k, v)
			first = false
		}
		fieldsStr += "]"
	}

	prefix := l.prefix
	if prefix != "" {
		prefix += " "
	}

	line := fmt.Sprintf("%s [%s] %s%s%s\n", timestamp, level, prefix, message, fieldsStr)

	if _, err := l.output.Write([]byte(line)); err != nil {
		log.Printf("failed to write log: %v", err)
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
	os.Exit(1)
}

func (l *Logger) Panic(format string, args ...any) {
	l.log(LevelError, format, args...)
	panic(fmt.Sprintf(format, args...))
}

func SetLevel(level Level) {
	defaultLogger.SetLevel(level)
}

func SetOutput(output io.Writer) {
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
