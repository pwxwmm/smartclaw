package memdir

import (
	"os"
	"path/filepath"
)

type MemDir struct {
	Path string
}

func NewMemDir() (*MemDir, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".smartclaw", "memdir")
	os.MkdirAll(path, 0755)
	return &MemDir{Path: path}, nil
}

func (m *MemDir) GetPath() string {
	return m.Path
}

func (m *MemDir) List() ([]string, error) {
	entries, err := os.ReadDir(m.Path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

func (m *MemDir) Read(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(m.Path, name))
}

func (m *MemDir) Write(name string, data []byte) error {
	return os.WriteFile(filepath.Join(m.Path, name), data, 0644)
}
