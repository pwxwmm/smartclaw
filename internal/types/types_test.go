package types

import (
	"math"
	"testing"
	"time"
)

func TestError_Error(t *testing.T) {
	e := &Error{Type: "test_error", Message: "something went wrong"}
	if got := e.Error(); got != "something went wrong" {
		t.Errorf("Error.Error() = %q, want %q", got, "something went wrong")
	}
}

func TestNewSession(t *testing.T) {
	s := NewSession("sess-1")
	if s.ID != "sess-1" {
		t.Errorf("NewSession ID = %q, want %q", s.ID, "sess-1")
	}
	if s.CreatedAt.IsZero() {
		t.Error("NewSession CreatedAt is zero")
	}
	if s.UpdatedAt.IsZero() {
		t.Error("NewSession UpdatedAt is zero")
	}
	if s.Messages == nil || len(s.Messages) != 0 {
		t.Errorf("NewSession Messages = %v, want empty non-nil slice", s.Messages)
	}
	if s.Metadata == nil || len(s.Metadata) != 0 {
		t.Errorf("NewSession Metadata = %v, want empty non-nil map", s.Metadata)
	}
}

func TestSession_AddMessage(t *testing.T) {
	s := NewSession("sess-1")
	before := s.UpdatedAt

	time.Sleep(time.Millisecond)
	s.AddMessage(Message{Role: "user", Content: "hello"})

	if len(s.Messages) != 1 {
		t.Fatalf("AddMessage: len(Messages) = %d, want 1", len(s.Messages))
	}
	if s.Messages[0].Role != "user" {
		t.Errorf("AddMessage: Role = %q, want %q", s.Messages[0].Role, "user")
	}
	if !s.UpdatedAt.After(before) {
		t.Error("AddMessage: UpdatedAt not updated")
	}
}

func TestSession_GetMessages(t *testing.T) {
	s := NewSession("sess-1")
	s.AddMessage(Message{Role: "user", Content: "a"})
	s.AddMessage(Message{Role: "assistant", Content: "b"})

	msgs := s.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("GetMessages: len = %d, want 2", len(msgs))
	}
	if msgs[0].Content != "a" || msgs[1].Content != "b" {
		t.Errorf("GetMessages: contents = %v, %v", msgs[0].Content, msgs[1].Content)
	}
}

func TestCredentials_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"zero time", time.Time{}, false},
		{"future", time.Now().Add(1 * time.Hour), false},
		{"past", time.Now().Add(-1 * time.Hour), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Credentials{ExpiresAt: tt.expiresAt}
			if got := c.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCredentials_IsValid(t *testing.T) {
	tests := []struct {
		name        string
		apiKey      string
		accessToken string
		want        bool
	}{
		{"both empty", "", "", false},
		{"api key only", "key123", "", true},
		{"access token only", "", "token123", true},
		{"both set", "key123", "token123", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Credentials{APIKey: tt.apiKey, AccessToken: tt.accessToken}
			if got := c.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheEntry_IsExpired(t *testing.T) {
	t.Run("nil ExpiresAt", func(t *testing.T) {
		e := &CacheEntry{ExpiresAt: nil}
		if e.IsExpired() {
			t.Error("IsExpired() = true for nil ExpiresAt, want false")
		}
	})

	t.Run("past ExpiresAt", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		e := &CacheEntry{ExpiresAt: &past}
		if !e.IsExpired() {
			t.Error("IsExpired() = false for past ExpiresAt, want true")
		}
	})

	t.Run("future ExpiresAt", func(t *testing.T) {
		future := time.Now().Add(1 * time.Hour)
		e := &CacheEntry{ExpiresAt: &future}
		if e.IsExpired() {
			t.Error("IsExpired() = true for future ExpiresAt, want false")
		}
	})
}

func TestProgress_Percent(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		completed int
		want      float64
	}{
		{"zero total", 0, 0, 0},
		{"half done", 100, 50, 50.0},
		{"all done", 10, 10, 100.0},
		{"one third", 3, 1, 100.0 / 3.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Progress{Total: tt.total, Completed: tt.completed}
			got := p.Percent()
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("Percent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProgress_IsComplete(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		completed int
		failed    int
		want      bool
	}{
		{"none done", 10, 0, 0, false},
		{"all completed", 10, 10, 0, true},
		{"all failed", 10, 0, 10, true},
		{"mixed", 10, 7, 3, true},
		{"partial", 10, 5, 2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Progress{Total: tt.total, Completed: tt.completed, Failed: tt.failed}
			if got := p.IsComplete(); got != tt.want {
				t.Errorf("IsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}
