package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHomeDir(t *testing.T) {
	home, err := HomeDir()
	if err != nil {
		t.Errorf("Expected no error getting home dir, got %v", err)
	}

	if home == "" {
		t.Error("Expected non-empty home directory")
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"~/test", filepath.Join(home, "test")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, test := range tests {
		result := ExpandPath(test.input)
		if result != test.expected {
			t.Errorf("ExpandPath(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "utils-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testDir := filepath.Join(tmpDir, "test", "nested", "dir")

	err = EnsureDir(testDir)
	if err != nil {
		t.Errorf("Expected no error creating directory, got %v", err)
	}

	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Error("Expected directory to exist")
	}
}

func TestFileExists(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if !FileExists(tmpFile.Name()) {
		t.Error("Expected file to exist")
	}

	if FileExists("/nonexistent/path/to/file") {
		t.Error("Expected nonexistent file to return false")
	}
}

func TestReadFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.WriteString("test content")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	data, err := ReadFile(tmpFile.Name())
	if err != nil {
		t.Errorf("Expected no error reading file, got %v", err)
	}

	if string(data) != "test content" {
		t.Errorf("Expected 'test content', got '%s'", string(data))
	}
}

func TestWriteFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "utils-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")

	err = WriteFile(testFile, []byte("test content"))
	if err != nil {
		t.Errorf("Expected no error writing file, got %v", err)
	}

	data, _ := os.ReadFile(testFile)
	if string(data) != "test content" {
		t.Errorf("Expected 'test content', got '%s'", string(data))
	}
}

func TestReadLines(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.WriteString("line1\nline2\nline3\n")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	lines, err := ReadLines(tmpFile.Name())
	if err != nil {
		t.Errorf("Expected no error reading lines, got %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	if lines[0] != "line1" {
		t.Errorf("Expected first line 'line1', got '%s'", lines[0])
	}
}

func TestWriteLines(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "utils-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	lines := []string{"line1", "line2", "line3"}

	err = WriteLines(testFile, lines)
	if err != nil {
		t.Errorf("Expected no error writing lines, got %v", err)
	}

	data, _ := os.ReadFile(testFile)
	expected := "line1\nline2\nline3\n"
	if string(data) != expected {
		t.Errorf("Expected '%s', got '%s'", expected, string(data))
	}
}

func TestRunCommand(t *testing.T) {
	output, err := RunCommand("echo", "hello")
	if err != nil {
		t.Errorf("Expected no error running command, got %v", err)
	}

	if output != "hello\n" {
		t.Errorf("Expected 'hello\\n', got '%s'", output)
	}
}

func TestRunCommandError(t *testing.T) {
	_, err := RunCommand("nonexistent-command-12345")
	if err == nil {
		t.Error("Expected error for nonexistent command")
	}
}
