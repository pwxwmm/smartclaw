package index

import (
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/watcher"
)

type IndexWatcher struct {
	index   *CodebaseIndex
	fw      *watcher.FileWatcher
	mu      sync.Mutex
	running bool
}

func NewIndexWatcher(idx *CodebaseIndex) (*IndexWatcher, error) {
	fw, err := watcher.NewFileWatcher()
	if err != nil {
		return nil, err
	}
	fw.SetDebounce(100 * time.Millisecond)

	iw := &IndexWatcher{
		index: idx,
		fw:    fw,
	}

	fw.On(watcher.EventCreate, iw.handleFileEvent)
	fw.On(watcher.EventWrite, iw.handleFileEvent)
	fw.On(watcher.EventRemove, iw.handleRemoveEvent)
	fw.On(watcher.EventRename, iw.handleRemoveEvent)

	return iw, nil
}

func (iw *IndexWatcher) Start() error {
	iw.mu.Lock()
	defer iw.mu.Unlock()

	if iw.running {
		return nil
	}

	if err := iw.addWatchedDirs(); err != nil {
		return err
	}

	if err := iw.fw.Start(); err != nil {
		return err
	}

	iw.running = true
	slog.Info("IndexWatcher: started", "root", iw.index.RootPath())
	return nil
}

func (iw *IndexWatcher) Stop() {
	iw.mu.Lock()
	defer iw.mu.Unlock()

	if !iw.running {
		return
	}

	iw.fw.Stop()
	iw.running = false
	slog.Info("IndexWatcher: stopped")
}

func (iw *IndexWatcher) handleFileEvent(evt watcher.Event) {
	path := evt.Path
	if !iw.index.shouldIndex(path) {
		return
	}

	relPath, err := filepath.Rel(iw.index.RootPath(), path)
	if err != nil {
		return
	}

	slog.Debug("IndexWatcher: re-indexing file", "file", relPath)
	if err := iw.index.IndexFile(path); err != nil {
		slog.Warn("IndexWatcher: failed to index file", "file", relPath, "error", err)
	}
}

func (iw *IndexWatcher) handleRemoveEvent(evt watcher.Event) {
	path := evt.Path
	if !iw.index.shouldIndex(path) {
		return
	}

	slog.Debug("IndexWatcher: removing file", "file", path)
	iw.index.RemoveFile(path)
}

func (iw *IndexWatcher) addWatchedDirs() error {
	return iw.fw.Add(iw.index.RootPath())
}
