package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSSEParser(t *testing.T) {
	sseData := "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\"}\n\n"
	reader := strings.NewReader(sseData)
	parser := NewSSEParser(reader)

	events := parser.Parse()

	var eventList []SSEEvent
	for event := range events {
		eventList = append(eventList, event)
	}

	if len(eventList) != 2 {
		t.Errorf("Expected 2 events, got %d", len(eventList))
	}

	if eventList[0].Event != "message_start" {
		t.Errorf("Expected event 'message_start', got '%s'", eventList[0].Event)
	}

	if eventList[1].Event != "content_block_delta" {
		t.Errorf("Expected event 'content_block_delta', got '%s'", eventList[1].Event)
	}
}

func TestSSEParserWithMultilineData(t *testing.T) {
	sseData := "event: test\ndata: line1\ndata: line2\n\n"
	reader := strings.NewReader(sseData)
	parser := NewSSEParser(reader)

	events := parser.Parse()

	var eventList []SSEEvent
	for event := range events {
		eventList = append(eventList, event)
	}

	if len(eventList) != 1 {
		t.Errorf("Expected 1 event, got %d", len(eventList))
	}

	expectedData := "line1\nline2"
	if eventList[0].Data != expectedData {
		t.Errorf("Expected data '%s', got '%s'", expectedData, eventList[0].Data)
	}
}

func TestStreamMessageParserMessageStart(t *testing.T) {
	parser := NewStreamMessageParser()

	data := `{"type":"message_start","message":{"id":"msg_123","model":"claude-sonnet-4-5","role":"assistant","usage":{"input_tokens":10,"output_tokens":0}}}`
	result, err := parser.HandleEvent("message_start", []byte(data))

	if err != nil {
		t.Errorf("HandleEvent failed: %v", err)
	}

	if !result.MessageStart {
		t.Error("Expected MessageStart to be true")
	}

	if parser.currentMessage.ID != "msg_123" {
		t.Errorf("Expected ID 'msg_123', got '%s'", parser.currentMessage.ID)
	}
}

func TestStreamMessageParserContentBlockStart(t *testing.T) {
	parser := NewStreamMessageParser()

	data := `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`
	result, err := parser.HandleEvent("content_block_start", []byte(data))

	if err != nil {
		t.Errorf("HandleEvent failed: %v", err)
	}

	if !result.ContentBlockStart {
		t.Error("Expected ContentBlockStart to be true")
	}

	if result.Index != 0 {
		t.Errorf("Expected index 0, got %d", result.Index)
	}
}

func TestStreamMessageParserContentBlockDelta(t *testing.T) {
	parser := NewStreamMessageParser()

	parser.contentBlocks = append(parser.contentBlocks, ContentBlock{Type: "text"})

	data := `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`
	result, err := parser.HandleEvent("content_block_delta", []byte(data))

	if err != nil {
		t.Errorf("HandleEvent failed: %v", err)
	}

	if result.TextDelta != "Hello" {
		t.Errorf("Expected text delta 'Hello', got '%s'", result.TextDelta)
	}

	if parser.contentBlocks[0].Text != "Hello" {
		t.Errorf("Expected content block text 'Hello', got '%s'", parser.contentBlocks[0].Text)
	}
}

func TestStreamMessageParserThinkingDelta(t *testing.T) {
	parser := NewStreamMessageParser()

	parser.contentBlocks = append(parser.contentBlocks, ContentBlock{Type: "thinking"})

	data := `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"thinking..."}}`
	result, err := parser.HandleEvent("content_block_delta", []byte(data))

	if err != nil {
		t.Errorf("HandleEvent failed: %v", err)
	}

	if result.ThinkingDelta != "thinking..." {
		t.Errorf("Expected thinking delta 'thinking...', got '%s'", result.ThinkingDelta)
	}
}

func TestStreamMessageParserMessageStop(t *testing.T) {
	parser := NewStreamMessageParser()

	data := `{"type":"message_stop"}`
	result, err := parser.HandleEvent("message_stop", []byte(data))

	if err != nil {
		t.Errorf("HandleEvent failed: %v", err)
	}

	if !result.MessageStop {
		t.Error("Expected MessageStop to be true")
	}

	if !parser.IsComplete() {
		t.Error("Expected parser to be complete")
	}
}

func TestStreamMessageParserPing(t *testing.T) {
	parser := NewStreamMessageParser()

	data := `{"type":"ping"}`
	result, err := parser.HandleEvent("ping", []byte(data))

	if err != nil {
		t.Errorf("HandleEvent failed: %v", err)
	}

	if !result.Ping {
		t.Error("Expected Ping to be true")
	}
}

func TestStreamMessageParserError(t *testing.T) {
	parser := NewStreamMessageParser()

	data := `{"type":"error","error":{"type":"api_error","message":"Rate limit exceeded"}}`
	result, err := parser.HandleEvent("error", []byte(data))

	if err != nil {
		t.Errorf("HandleEvent failed: %v", err)
	}

	if result.Error == nil {
		t.Error("Expected error in result")
	}
}

func TestStreamingClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"model\":\"claude-sonnet-4-5\",\"role\":\"assistant\"}}\n\n"))
		w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n"))
		w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n"))
		w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL

	streamingClient := NewStreamingClient(client)

	req := &MessageRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages:  []Message{{Role: "user", Content: "Hello"}},
	}

	var events []StreamEventResult
	err := streamingClient.Stream(context.Background(), req, func(event StreamEventResult) {
		events = append(events, event)
	})

	if err != nil {
		t.Errorf("Stream failed: %v", err)
	}

	if len(events) == 0 {
		t.Error("Expected at least one event")
	}

	message := streamingClient.GetMessage()
	if message == nil {
		t.Error("Expected non-nil message")
	}

	if message.ID != "msg_test" {
		t.Errorf("Expected ID 'msg_test', got '%s'", message.ID)
	}
}

func TestStreamIterator(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_iter\",\"model\":\"claude-sonnet-4-5\",\"role\":\"assistant\"}}\n\n"))
		w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Test\"}}\n\n"))
		w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL

	req := &MessageRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages:  []Message{{Role: "user", Content: "Test"}},
	}

	iterator, err := client.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	var eventCount int
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}
		eventCount++
		if event.Error != nil {
			t.Errorf("Event error: %v", event.Error)
		}
	}

	if eventCount == 0 {
		t.Error("Expected at least one event")
	}

	message := iterator.Message()
	if message == nil {
		t.Error("Expected non-nil message")
	}
}

func TestStreamStats(t *testing.T) {
	startTime := time.Now()
	stats := StreamStats{
		StartTime: startTime,
	}

	if stats.TTFB() != 0 {
		t.Error("Expected TTFB to be 0 with no FirstChunkTime")
	}

	if stats.Duration() != 0 {
		t.Error("Expected Duration to be 0 with no LastChunkTime")
	}

	now := time.Now()
	stats.FirstChunkTime = &now
	stats.LastChunkTime = &now

	if stats.TTFB() == 0 {
		t.Error("Expected non-zero TTFB")
	}

	if stats.Duration() == 0 {
		t.Error("Expected non-zero Duration")
	}
}

func TestStreamWithStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_stats\"}}\n\n"))
		w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL

	req := &MessageRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages:  []Message{{Role: "user", Content: "Test"}},
	}

	var statsUpdates []StreamStats
	stream, err := client.StreamWithStats(context.Background(), req, func(s StreamStats) {
		statsUpdates = append(statsUpdates, s)
	})
	if err != nil {
		t.Fatalf("StreamWithStats failed: %v", err)
	}

	for {
		_, ok := stream.Next()
		if !ok {
			break
		}
	}

	stats := stream.Stats()
	if stats.ChunkCount == 0 {
		t.Error("Expected at least one chunk")
	}

	if stats.FirstChunkTime == nil {
		t.Error("Expected FirstChunkTime to be set")
	}

	if stats.LastChunkTime == nil {
		t.Error("Expected LastChunkTime to be set")
	}
}

func TestStreamMessageParserMessageDelta(t *testing.T) {
	parser := NewStreamMessageParser()

	data := `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":50}}`
	result, err := parser.HandleEvent("message_delta", []byte(data))

	if err != nil {
		t.Errorf("HandleEvent failed: %v", err)
	}

	if !result.MessageDelta {
		t.Error("Expected MessageDelta to be true")
	}

	if parser.stopReason != "end_turn" {
		t.Errorf("Expected stop reason 'end_turn', got '%s'", parser.stopReason)
	}
}

func TestStreamMessageParserGetMessage(t *testing.T) {
	parser := NewStreamMessageParser()
	parser.currentMessage.ID = "msg_test"
	parser.contentBlocks = []ContentBlock{{Type: "text", Text: "Hello"}}
	parser.usage = Usage{InputTokens: 10, OutputTokens: 5}
	parser.stopReason = "end_turn"

	message := parser.GetMessage()

	if message.ID != "msg_test" {
		t.Errorf("Expected ID 'msg_test', got '%s'", message.ID)
	}

	if len(message.Content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(message.Content))
	}

	if message.StopReason != "end_turn" {
		t.Errorf("Expected stop reason 'end_turn', got '%s'", message.StopReason)
	}
}

func TestStreamMessageParserGetContentBlocks(t *testing.T) {
	parser := NewStreamMessageParser()
	parser.contentBlocks = []ContentBlock{
		{Type: "text", Text: "Block 1"},
		{Type: "text", Text: "Block 2"},
	}

	blocks := parser.GetContentBlocks()

	if len(blocks) != 2 {
		t.Errorf("Expected 2 content blocks, got %d", len(blocks))
	}
}

func TestClientStreamMessageSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Expected Accept header 'text/event-stream', got '%s'", r.Header.Get("Accept"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: ping\ndata: {\"type\":\"ping\"}\n\n"))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL

	req := &MessageRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages:  []Message{{Role: "user", Content: "Test"}},
	}

	var receivedEvents []string
	err := client.StreamMessageSSE(context.Background(), req, func(event string, data []byte) error {
		receivedEvents = append(receivedEvents, event)
		return nil
	})

	if err != nil {
		t.Errorf("StreamMessageSSE failed: %v", err)
	}

	if len(receivedEvents) == 0 {
		t.Error("Expected at least one event")
	}
}

func TestClientStreamMessageSSEError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL

	req := &MessageRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages:  []Message{{Role: "user", Content: "Test"}},
	}

	err := client.StreamMessageSSE(context.Background(), req, func(event string, data []byte) error {
		return nil
	})

	if err == nil {
		t.Error("Expected error for 500 response")
	}
}

func TestClientStreamMessageSSEHandlerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: test\ndata: {}\n\n"))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL

	req := &MessageRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages:  []Message{{Role: "user", Content: "Test"}},
	}

	err := client.StreamMessageSSE(context.Background(), req, func(event string, data []byte) error {
		return fmt.Errorf("handler error")
	})

	if err == nil {
		t.Error("Expected error from handler")
	}
}
