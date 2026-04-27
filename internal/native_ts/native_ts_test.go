package nativets

import (
	"testing"
)

func TestYogaLayout(t *testing.T) {
	t.Parallel()

	err := YogaLayout()
	if err != nil {
		t.Errorf("YogaLayout() returned unexpected error: %v", err)
	}
}

func TestColorDiff(t *testing.T) {
	t.Parallel()

	err := ColorDiff()
	if err != nil {
		t.Errorf("ColorDiff() returned unexpected error: %v", err)
	}
}

func TestFileIndex(t *testing.T) {
	t.Parallel()

	err := FileIndex()
	if err != nil {
		t.Errorf("FileIndex() returned unexpected error: %v", err)
	}
}

func TestRunNative_Success(t *testing.T) {
	err := RunNative("echo", "hello")
	if err != nil {
		t.Errorf("RunNative(\"echo\", \"hello\") returned unexpected error: %v", err)
	}
}

func TestRunNative_Failure(t *testing.T) {
	err := RunNative("nonexistent_command_that_does_not_exist_12345")
	if err == nil {
		t.Error("RunNative with nonexistent command should return an error")
	}
}

func TestRunNative_WithArgs(t *testing.T) {
	err := RunNative("printf", "%s %s", "hello", "world")
	if err != nil {
		t.Errorf("RunNative with multiple args returned unexpected error: %v", err)
	}
}

func TestRunNative_ExitCode(t *testing.T) {
	err := RunNative("false")
	if err == nil {
		t.Error("RunNative(\"false\") should return an error for non-zero exit code")
	}
}
