package native

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FileEntry struct {
	Path       string
	Name       string
	Ext        string
	Size       int64
	ModifiedAt time.Time
	IsDir      bool
}

type FileIndex struct {
	entries map[string]*FileEntry
	roots   []string
	mu      sync.RWMutex
}

func NewFileIndex() *FileIndex {
	return &FileIndex{
		entries: make(map[string]*FileEntry),
		roots:   []string{},
	}
}

func (idx *FileIndex) AddRoot(root string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	idx.roots = append(idx.roots, absRoot)
	return nil
}

func (idx *FileIndex) Scan() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for _, root := range idx.roots {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			entry := &FileEntry{
				Path:       path,
				Name:       info.Name(),
				Ext:        strings.ToLower(filepath.Ext(path)),
				Size:       info.Size(),
				ModifiedAt: info.ModTime(),
				IsDir:      info.IsDir(),
			}

			idx.entries[path] = entry
			return nil
		})
	}

	return nil
}

func (idx *FileIndex) Get(path string) *FileEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.entries[path]
}

func (idx *FileIndex) FindByName(name string) []*FileEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []*FileEntry
	for _, entry := range idx.entries {
		if strings.Contains(strings.ToLower(entry.Name), strings.ToLower(name)) {
			results = append(results, entry)
		}
	}
	return results
}

func (idx *FileIndex) FindByExt(ext string) []*FileEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []*FileEntry
	for _, entry := range idx.entries {
		if entry.Ext == strings.ToLower(ext) {
			results = append(results, entry)
		}
	}
	return results
}

func (idx *FileIndex) Remove(path string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.entries, path)
}

func (idx *FileIndex) Clear() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.entries = make(map[string]*FileEntry)
}

func (idx *FileIndex) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

func (idx *FileIndex) ListAll() []*FileEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	results := make([]*FileEntry, 0, len(idx.entries))
	for _, entry := range idx.entries {
		results = append(results, entry)
	}
	return results
}
