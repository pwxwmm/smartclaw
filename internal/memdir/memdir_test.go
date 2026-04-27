package memdir

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestNewMemDir(t *testing.T) {
	t.Parallel()

	md, err := NewMemDir()
	if err != nil {
		t.Fatalf("NewMemDir() returned error: %v", err)
	}
	if md == nil {
		t.Fatal("NewMemDir() returned nil")
	}
	if md.Path == "" {
		t.Error("MemDir.Path is empty")
	}
}

func TestNewMemDir_PathContainsSmartclaw(t *testing.T) {
	t.Parallel()

	md, err := NewMemDir()
	if err != nil {
		t.Fatalf("NewMemDir() returned error: %v", err)
	}
	if !contains(md.Path, ".smartclaw") {
		t.Errorf("Path = %q, want path containing .smartclaw", md.Path)
	}
	if !contains(md.Path, "memdir") {
		t.Errorf("Path = %q, want path containing memdir", md.Path)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGetPath(t *testing.T) {
	t.Parallel()

	md := &MemDir{Path: "/tmp/test-memdir"}
	if got := md.GetPath(); got != "/tmp/test-memdir" {
		t.Errorf("GetPath() = %q, want %q", got, "/tmp/test-memdir")
	}
}

func TestWriteAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	md := &MemDir{Path: tmpDir}

	data := []byte("hello world")
	if err := md.Write("test.txt", data); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}

	got, err := md.Read("test.txt")
	if err != nil {
		t.Fatalf("Read() returned error: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("Read() = %q, want %q", string(got), "hello world")
	}
}

func TestRead_NonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	md := &MemDir{Path: tmpDir}

	_, err := md.Read("does_not_exist.txt")
	if err == nil {
		t.Error("Read() on nonexistent file should return an error")
	}
}

func TestWrite_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	md := &MemDir{Path: tmpDir}

	if err := md.Write("test.txt", []byte("first")); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := md.Write("test.txt", []byte("second")); err != nil {
		t.Fatalf("Write() overwrite returned error: %v", err)
	}

	got, err := md.Read("test.txt")
	if err != nil {
		t.Fatalf("Read() returned error: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("Read() after overwrite = %q, want %q", string(got), "second")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	md := &MemDir{Path: tmpDir}

	md.Write("file1.txt", []byte("a"))
	md.Write("file2.txt", []byte("b"))

	files, err := md.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}

	sort.Strings(files)
	want := []string{"file1.txt", "file2.txt"}
	if len(files) != len(want) {
		t.Fatalf("List() returned %d files, want %d", len(files), len(want))
	}
	for i, f := range files {
		if f != want[i] {
			t.Errorf("files[%d] = %q, want %q", i, f, want[i])
		}
	}
}

func TestList_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	md := &MemDir{Path: tmpDir}

	files, err := md.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("List() on empty dir returned %d files, want 0", len(files))
	}
}

func TestList_SkipsDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	md := &MemDir{Path: tmpDir}

	md.Write("file.txt", []byte("data"))
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)

	files, err := md.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(files) != 1 || files[0] != "file.txt" {
		t.Errorf("List() = %v, want [file.txt] (skipping directories)", files)
	}
}

func TestList_NonexistentDir(t *testing.T) {
	md := &MemDir{Path: "/nonexistent/path/that/does/not/exist"}

	_, err := md.List()
	if err == nil {
		t.Error("List() on nonexistent directory should return an error")
	}
}

func TestWrite_EmptyData(t *testing.T) {
	tmpDir := t.TempDir()
	md := &MemDir{Path: tmpDir}

	if err := md.Write("empty.txt", []byte{}); err != nil {
		t.Fatalf("Write() with empty data returned error: %v", err)
	}

	got, err := md.Read("empty.txt")
	if err != nil {
		t.Fatalf("Read() returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Read() = %d bytes, want 0 bytes", len(got))
	}
}
