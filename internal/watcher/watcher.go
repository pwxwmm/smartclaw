package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type EventType string

const (
	EventCreate EventType = "create"
	EventWrite  EventType = "write"
	EventRemove EventType = "remove"
	EventRename EventType = "rename"
	EventChmod  EventType = "chmod"
)

type Event struct {
	Type      EventType
	Path      string
	Timestamp time.Time
}

type Handler func(event Event)

type FileWatcher struct {
	watcher  *fsnotify.Watcher
	handlers map[EventType][]Handler
	paths    map[string]bool
	mu       sync.RWMutex
	running  bool
	debounce time.Duration
	timers   map[string]*time.Timer
}

func NewFileWatcher() (*FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &FileWatcher{
		watcher:  w,
		handlers: make(map[EventType][]Handler),
		paths:    make(map[string]bool),
		debounce: 100 * time.Millisecond,
		timers:   make(map[string]*time.Timer),
	}, nil
}

func (fw *FileWatcher) Add(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if fw.paths[absPath] {
		return nil
	}

	if err := fw.watcher.Add(absPath); err != nil {
		return err
	}

	fw.paths[absPath] = true
	return nil
}

func (fw *FileWatcher) Remove(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if !fw.paths[absPath] {
		return nil
	}

	if err := fw.watcher.Remove(absPath); err != nil {
		return err
	}

	delete(fw.paths, absPath)
	return nil
}

func (fw *FileWatcher) On(eventType EventType, handler Handler) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	fw.handlers[eventType] = append(fw.handlers[eventType], handler)
}

func (fw *FileWatcher) Start() error {
	fw.mu.Lock()
	if fw.running {
		fw.mu.Unlock()
		return fmt.Errorf("watcher already running")
	}
	fw.running = true
	fw.mu.Unlock()

	go fw.eventLoop()

	return nil
}

func (fw *FileWatcher) Stop() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if !fw.running {
		return nil
	}

	fw.running = false
	return fw.watcher.Close()
}

func (fw *FileWatcher) eventLoop() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
		}
	}
}

func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	var eventType EventType
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		eventType = EventCreate
	case event.Op&fsnotify.Write == fsnotify.Write:
		eventType = EventWrite
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		eventType = EventRemove
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		eventType = EventRename
	case event.Op&fsnotify.Chmod == fsnotify.Chmod:
		eventType = EventChmod
	default:
		return
	}

	key := event.Name + string(eventType)

	if timer, exists := fw.timers[key]; exists {
		timer.Stop()
	}

	timer := time.AfterFunc(fw.debounce, func() {
		evt := Event{
			Type:      eventType,
			Path:      event.Name,
			Timestamp: time.Now(),
		}

		handlers := fw.handlers[eventType]
		for _, handler := range handlers {
			handler(evt)
		}

		fw.mu.Lock()
		delete(fw.timers, key)
		fw.mu.Unlock()
	})

	fw.timers[key] = timer
}

func (fw *FileWatcher) SetDebounce(duration time.Duration) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.debounce = duration
}

func (fw *FileWatcher) WatchedPaths() []string {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	paths := make([]string, 0, len(fw.paths))
	for path := range fw.paths {
		paths = append(paths, path)
	}
	return paths
}

func (fw *FileWatcher) IsWatching(path string) bool {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	return fw.paths[absPath]
}

func WatchDirectory(path string, handler Handler) (*FileWatcher, error) {
	fw, err := NewFileWatcher()
	if err != nil {
		return nil, err
	}

	if err := fw.Add(path); err != nil {
		fw.Stop()
		return nil, err
	}

	fw.On(EventCreate, handler)
	fw.On(EventWrite, handler)
	fw.On(EventRemove, handler)

	if err := fw.Start(); err != nil {
		fw.Stop()
		return nil, err
	}

	return fw, nil
}
