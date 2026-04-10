package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db       *sql.DB
	dbPath   string
	jsonlDir string
}

func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	dir := filepath.Join(home, ".smartclaw")
	return NewStoreWithDir(dir)
}

func NewStoreWithDir(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("store: mkdir: %w", err)
	}

	dbPath := filepath.Join(dir, "state.db")
	jsonlDir := filepath.Join(dir, "jsonl")

	s := &Store{
		dbPath:   dbPath,
		jsonlDir: jsonlDir,
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}

	if _, err := db.Exec(SchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: schema: %w", err)
	}

	s.db = db
	slog.Info("store: opened SQLite database", "path", dbPath)
	return s, nil
}

func (s *Store) Close() error {
	if s.db != nil {
		if _, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			slog.Warn("store: WAL checkpoint failed", "error", err)
		}
		return s.db.Close()
	}
	return nil
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) DBPath() string {
	return s.dbPath
}

func (s *Store) JSONLDir() string {
	return s.jsonlDir
}

func (s *Store) WriteWithRetry(query string, args ...interface{}) error {
	return s.WriteWithRetryContext(context.Background(), query, args...)
}

func (s *Store) WriteWithRetryContext(ctx context.Context, query string, args ...interface{}) error {
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Use BEGIN IMMEDIATE to acquire a reserved lock immediately,
		// preventing concurrent writers from conflicting (gateway + CLI).
		_, err := s.db.ExecContext(ctx, "BEGIN IMMEDIATE")
		if err != nil {
			if isLockedError(err) && attempt < maxRetries-1 {
				jitter := time.Duration(50+rand.Intn(150)) * time.Millisecond * time.Duration(1<<attempt)
				time.Sleep(jitter)
				continue
			}
			return fmt.Errorf("store: begin immediate: %w", err)
		}

		if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
			s.db.ExecContext(ctx, "ROLLBACK")
			if isLockedError(err) && attempt < maxRetries-1 {
				jitter := time.Duration(50+rand.Intn(150)) * time.Millisecond * time.Duration(1<<attempt)
				time.Sleep(jitter)
				continue
			}
			return fmt.Errorf("store: exec: %w", err)
		}

		if _, err := s.db.ExecContext(ctx, "COMMIT"); err != nil {
			if isLockedError(err) && attempt < maxRetries-1 {
				jitter := time.Duration(50+rand.Intn(150)) * time.Millisecond * time.Duration(1<<attempt)
				time.Sleep(jitter)
				continue
			}
			return fmt.Errorf("store: commit: %w", err)
		}

		return nil
	}
	return fmt.Errorf("store: max retries exceeded for query")
}

func isLockedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "locked") || contains(msg, "busy") || contains(msg, "SQLITE_BUSY")
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
