package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
)

func TestQueryStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		msgStart := api.MessageResponse{
			ID:    "msg_stream_test",
			Model: "claude-sonnet-4-5",
			Role:  "assistant",
			Usage: api.Usage{InputTokens: 10, OutputTokens: 0},
		}
		startData, _ := json.Marshal(map[string]interface{}{
			"type":    "message_start",
			"message": msgStart,
		})
		w.Write([]byte("event: message_start\ndata: " + string(startData) + "\n\n"))

		cbStartData, _ := json.Marshal(map[string]interface{}{
			"type":          "content_block_start",
			"index":         0,
			"content_block": map[string]string{"type": "text", "text": ""},
		})
		w.Write([]byte("event: content_block_start\ndata: " + string(cbStartData) + "\n\n"))

		deltaData, _ := json.Marshal(map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]string{"type": "text_delta", "text": "Hello streaming"},
		})
		w.Write([]byte("event: content_block_delta\ndata: " + string(deltaData) + "\n\n"))

		deltaStopData, _ := json.Marshal(map[string]interface{}{
			"type":  "content_block_stop",
			"index": 0,
		})
		w.Write([]byte("event: content_block_stop\ndata: " + string(deltaStopData) + "\n\n"))

		msgDeltaData, _ := json.Marshal(map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]string{
				"stop_reason": "end_turn",
			},
			"usage": map[string]int{"output_tokens": 5},
		})
		w.Write([]byte("event: message_delta\ndata: " + string(msgDeltaData) + "\n\n"))

		w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	client := api.NewClient("test-key")
	client.BaseURL = server.URL

	engine := NewQueryEngine(client, QueryConfig{})

	var mu sync.Mutex
	var events []QueryEvent

	err := engine.QueryStream(context.Background(), "test streaming", func(event QueryEvent) error {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
		return nil
	})

	if err != nil {
		t.Fatalf("QueryStream failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	hasStart := false
	hasDelta := false
	hasDone := false
	for _, e := range events {
		switch e.Type {
		case "start":
			hasStart = true
		case "text_delta":
			hasDelta = true
		case "done":
			hasDone = true
		}
	}

	if !hasStart {
		t.Error("expected start event")
	}
	if !hasDelta {
		t.Error("expected text_delta event")
	}
	if !hasDone {
		t.Error("expected done event")
	}

	state := engine.GetState()
	msgs := state.GetMessages()
	if len(msgs) < 2 {
		t.Errorf("expected at least 2 messages (user + assistant), got %d", len(msgs))
	}
}

func TestQueryStreamCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte("event: ping\ndata: {\"type\":\"ping\"}\n\n"))
	}))
	defer server.Close()

	client := api.NewClient("test-key")
	client.BaseURL = server.URL

	engine := NewQueryEngine(client, QueryConfig{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := engine.QueryStream(ctx, "test cancel", func(event QueryEvent) error {
		return nil
	})

	if err == nil {
		t.Error("expected error from cancelled context")
	}
}
