package utils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home, nil
}

func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func EnsureDir(path string) error {
	absPath := ExpandPath(path)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return os.MkdirAll(absPath, 0755)
	}
	return nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(ExpandPath(path))
}

func WriteFile(path string, data []byte) error {
	absPath := ExpandPath(path)
	if err := EnsureDir(filepath.Dir(absPath)); err != nil {
		return err
	}
	return os.WriteFile(absPath, data, 0644)
}

func ReadLines(path string) ([]string, error) {
	file, err := os.Open(ExpandPath(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func WriteLines(path string, lines []string) error {
	var content bytes.Buffer
	for _, line := range lines {
		content.WriteString(line)
		content.WriteString("\n")
	}
	return WriteFile(path, content.Bytes())
}

func RunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, output)
	}
	return string(output), nil
}

func RunCommandTimeout(name string, args []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out")
	}
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, output)
	}
	return string(output), nil
}

func CountTokens(text string) int {
	count := 0
	for _, word := range text {
		if word > 0 {
			count++
		}
	}
	return count / 4
}

func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func JSONMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func JSONUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func JSONMarshalIndent(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func Base64Decode(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func Contains(s []string, item string) bool {
	for _, v := range s {
		if v == item {
			return true
		}
	}
	return false
}

func RemoveDuplicates(s []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

func MapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func MapValues(m map[string]interface{}) []interface{} {
	values := make([]interface{}, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

func Getenv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func MustGetenv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("environment variable %s not set", key))
	}
	return value
}

func OS() string {
	return runtime.GOOS
}

func Arch() string {
	return runtime.GOARCH
}

func NumCPU() int {
	return runtime.NumCPU()
}

func Version() string {
	return runtime.Version()
}

func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func ParseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

func Since(t time.Time) time.Duration {
	return time.Since(t)
}

func ParseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func Now() time.Time {
	return time.Now()
}

func Sleep(d time.Duration) {
	time.Sleep(d)
}

func Getwd() (string, error) {
	return os.Getwd()
}

func Chdir(dir string) error {
	return os.Chdir(dir)
}

func Hostname() (string, error) {
	return os.Hostname()
}

func Getpid() int {
	return os.Getppid()
}

func IsTerminal(fd int) bool {
	return false
}

func GetTerminalWidth() int {
	return 80
}
