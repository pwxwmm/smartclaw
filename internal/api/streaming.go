package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/pool"
	"github.com/instructkr/smartclaw/internal/utils"
)

type SSEEvent struct {
	Event string
	Data  string
}

type SSEParser struct {
	reader *bufio.Reader
}

func NewSSEParser(reader io.Reader) *SSEParser {
	return &SSEParser{
		reader: bufio.NewReader(reader),
	}
}

func (p *SSEParser) Parse() <-chan SSEEvent {
	events := make(chan SSEEvent, 100)

	utils.Go(func() {
		defer close(events)

		event := pool.GetBuffer()
		data := pool.GetBuffer()
		defer pool.PutBuffer(event)
		defer pool.PutBuffer(data)

		for {
			line, err := p.reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					if data.Len() > 0 {
						events <- SSEEvent{
							Event: event.String(),
							Data:  data.String(),
						}
					}
				}
				return
			}

			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")

			if line == "" {
				if data.Len() > 0 {
					events <- SSEEvent{
						Event: event.String(),
						Data:  data.String(),
					}
					event.Reset()
					data.Reset()
				}
				continue
			}

			if strings.HasPrefix(line, ":") {
				continue
			}

			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}

			field := parts[0]
			value := strings.TrimSpace(parts[1])

			switch field {
			case "event":
				event.WriteString(value)
			case "data":
				if data.Len() > 0 {
					data.WriteByte('\n')
				}
				data.WriteString(value)
			}
		}
	})

	return events
}

func (c *Client) StreamMessageSSE(ctx context.Context, req *MessageRequest, handler func(event string, data []byte) error) error {
	req.Stream = true

	if req.System != nil {
		if sysStr, ok := req.System.(string); ok && sysStr != "" {
			req.System = []SystemBlock{
				{
					Type: "text",
					Text: sysStr,
					CacheControl: &CacheControl{
						Type: "ephemeral",
					},
				},
			}
		}
	}

	c.ensureSDKClient()

	params := buildSDKMessages(req.Messages, req.System, req.Model, req.MaxTokens, req.Thinking, req.Tools)

	return streamWithSDK(ctx, c.sdkClient, params, handler)
}

type StreamEventResult struct {
	Type              string
	TextDelta         string
	ThinkingDelta     string
	Index             int
	DeltaType         string
	MessageStart      bool
	MessageDelta      bool
	MessageStop       bool
	ContentBlockStart bool
	ContentBlockStop  bool
	Ping              bool
	Error             error
}

type StreamMessageParser struct {
	mu             sync.Mutex
	currentMessage *MessageResponse
	contentBlocks  []ContentBlock
	usage          Usage
	stopReason     string
	isComplete     bool
}

func NewStreamMessageParser() *StreamMessageParser {
	return &StreamMessageParser{
		currentMessage: &MessageResponse{},
		contentBlocks:  []ContentBlock{},
	}
}

func (p *StreamMessageParser) HandleEvent(event string, data []byte) (StreamEventResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(data) == 0 {
		return StreamEventResult{}, nil
	}

	var eventData map[string]any
	if err := json.Unmarshal(data, &eventData); err != nil {
		return StreamEventResult{}, nil
	}

	eventType, _ := eventData["type"].(string)
	result := StreamEventResult{Type: eventType}

	switch eventType {
	case "message_start":
		if msg, ok := eventData["message"].(map[string]any); ok {
			if id, ok := msg["id"].(string); ok {
				p.currentMessage.ID = id
			}
			if model, ok := msg["model"].(string); ok {
				p.currentMessage.Model = model
			}
			if role, ok := msg["role"].(string); ok {
				p.currentMessage.Role = role
			}
			if usage, ok := msg["usage"].(map[string]any); ok {
				if input, ok := usage["input_tokens"].(float64); ok {
					p.usage.InputTokens = int(input)
				}
				if output, ok := usage["output_tokens"].(float64); ok {
					p.usage.OutputTokens = int(output)
				}
				if cacheCreate, ok := usage["cache_creation_input_tokens"].(float64); ok {
					p.usage.CacheCreation = int(cacheCreate)
				}
				if cacheRead, ok := usage["cache_read_input_tokens"].(float64); ok {
					p.usage.CacheRead = int(cacheRead)
				}
			}
		}
		result.MessageStart = true

	case "content_block_start":
		if index, ok := eventData["index"].(float64); ok {
			if contentBlock, ok := eventData["content_block"].(map[string]any); ok {
				blockType, _ := contentBlock["type"].(string)

				for int(index) >= len(p.contentBlocks) {
					p.contentBlocks = append(p.contentBlocks, ContentBlock{})
				}

				p.contentBlocks[int(index)].Type = blockType

				switch blockType {
				case "tool_use", "server_tool_use":
					if id, ok := contentBlock["id"].(string); ok {
						p.contentBlocks[int(index)].ID = id
					}
					if name, ok := contentBlock["name"].(string); ok {
						p.contentBlocks[int(index)].Name = name
					}
				case "text":
					if text, ok := contentBlock["text"].(string); ok {
						p.contentBlocks[int(index)].Text = text
					}
				case "thinking":
					if thinking, ok := contentBlock["thinking"].(string); ok {
						p.contentBlocks[int(index)].Thinking = thinking
					}
				}

				result.ContentBlockStart = true
				result.Index = int(index)
			}
		}

	case "content_block_delta":
		if index, ok := eventData["index"].(float64); ok {
			if delta, ok := eventData["delta"].(map[string]any); ok {
				if deltaType, ok := delta["type"].(string); ok {
					idx := int(index)
					if idx < len(p.contentBlocks) {
						switch deltaType {
						case "text_delta":
							if text, ok := delta["text"].(string); ok {
								p.contentBlocks[idx].Text += text
								result.TextDelta = text
							}
						case "input_json_delta":
							if partialJson, ok := delta["partial_json"].(string); ok {
								p.contentBlocks[idx].PartialJSON += partialJson
							}
						case "thinking_delta":
							if thinking, ok := delta["thinking"].(string); ok {
								p.contentBlocks[idx].Thinking += thinking
								result.ThinkingDelta = thinking
							}
						}
					}
					result.Index = idx
					result.DeltaType = deltaType
				}
			}
		}

	case "content_block_stop":
		if index, ok := eventData["index"].(float64); ok {
			idx := int(index)
			if idx < len(p.contentBlocks) {
				if p.contentBlocks[idx].PartialJSON != "" {
					var input map[string]any
					if err := json.Unmarshal([]byte(p.contentBlocks[idx].PartialJSON), &input); err == nil {
						p.contentBlocks[idx].Input = input
					}
					p.contentBlocks[idx].PartialJSON = ""
				}
			}
			result.ContentBlockStop = true
			result.Index = idx
		}

	case "message_delta":
		if delta, ok := eventData["delta"].(map[string]any); ok {
			if stopReason, ok := delta["stop_reason"].(string); ok {
				p.stopReason = stopReason
				p.currentMessage.StopReason = stopReason
			}
		}
		if usage, ok := eventData["usage"].(map[string]any); ok {
			if output, ok := usage["output_tokens"].(float64); ok {
				p.usage.OutputTokens += int(output)
			}
		}
		result.MessageDelta = true

	case "message_stop":
		p.currentMessage.Content = p.contentBlocks
		p.currentMessage.Usage = p.usage
		p.isComplete = true
		result.MessageStop = true

	case "ping":
		result.Ping = true

	case "error":
		if errData, ok := eventData["error"].(map[string]any); ok {
			errType, _ := errData["type"].(string)
			errMsg, _ := errData["message"].(string)
			result.Error = fmt.Errorf("%s: %s", errType, errMsg)
		}
	}

	return result, nil
}

func (p *StreamMessageParser) GetMessage() *MessageResponse {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.currentMessage.Content = p.contentBlocks
	p.currentMessage.Usage = p.usage
	p.currentMessage.StopReason = p.stopReason
	return p.currentMessage
}

func (p *StreamMessageParser) IsComplete() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.isComplete
}

func (p *StreamMessageParser) GetContentBlocks() []ContentBlock {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.contentBlocks
}

type StreamingClient struct {
	client       *Client
	parser       *StreamMessageParser
	eventHandler func(StreamEventResult)
	mu           sync.Mutex
}

func NewStreamingClient(client *Client) *StreamingClient {
	return &StreamingClient{
		client: client,
		parser: NewStreamMessageParser(),
	}
}

func (s *StreamingClient) Stream(ctx context.Context, req *MessageRequest, onEvent func(StreamEventResult)) error {
	s.mu.Lock()
	s.eventHandler = onEvent
	s.mu.Unlock()

	return s.client.StreamMessageSSE(ctx, req, func(event string, data []byte) error {
		result, err := s.parser.HandleEvent(event, data)
		if err != nil {
			return err
		}

		s.mu.Lock()
		handler := s.eventHandler
		s.mu.Unlock()

		if handler != nil {
			handler(result)
		}

		return nil
	})
}

func (s *StreamingClient) GetMessage() *MessageResponse {
	return s.parser.GetMessage()
}

type StreamIterator struct {
	events   <-chan StreamEventResult
	message  *MessageResponse
	parser   *StreamMessageParser
	complete bool
}

func (c *Client) Stream(ctx context.Context, req *MessageRequest) (*StreamIterator, error) {
	eventChan := make(chan StreamEventResult, 100)
	parser := NewStreamMessageParser()

	utils.Go(func() {
		defer close(eventChan)

		err := c.StreamMessageSSE(ctx, req, func(event string, data []byte) error {
			result, err := parser.HandleEvent(event, data)
			if err != nil {
				return err
			}

			select {
			case eventChan <- result:
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		})

		if err != nil && err != context.Canceled {
			eventChan <- StreamEventResult{
				Type:  "error",
				Error: err,
			}
		}
	})

	return &StreamIterator{
		events: eventChan,
		parser: parser,
	}, nil
}

func (it *StreamIterator) Next() (StreamEventResult, bool) {
	if it.complete {
		return StreamEventResult{}, false
	}

	event, ok := <-it.events
	if !ok {
		it.complete = true
		it.message = it.parser.GetMessage()
		return StreamEventResult{Type: "done", MessageStop: true}, false
	}

	if event.MessageStop {
		it.complete = true
		it.message = it.parser.GetMessage()
	}

	return event, true
}

func (it *StreamIterator) Message() *MessageResponse {
	if it.message != nil {
		return it.message
	}
	return it.parser.GetMessage()
}

func (it *StreamIterator) ForEach(fn func(StreamEventResult) error) error {
	for {
		event, ok := it.Next()
		if !ok {
			return nil
		}
		if event.Error != nil {
			return event.Error
		}
		if fn != nil {
			if err := fn(event); err != nil {
				return err
			}
		}
	}
}

type StreamStats struct {
	StartTime      time.Time
	FirstChunkTime *time.Time
	LastChunkTime  *time.Time
	ChunkCount     int
	InputTokens    int
	OutputTokens   int
	StallCount     int
	TotalStallTime time.Duration
}

func (s *StreamStats) TTFB() time.Duration {
	if s.FirstChunkTime == nil {
		return 0
	}
	return s.FirstChunkTime.Sub(s.StartTime)
}

func (s *StreamStats) Duration() time.Duration {
	if s.LastChunkTime == nil {
		return 0
	}
	return s.LastChunkTime.Sub(s.StartTime)
}

type StreamWithStats struct {
	iterator *StreamIterator
	stats    StreamStats
	onStats  func(StreamStats)
}

func (c *Client) StreamWithStats(ctx context.Context, req *MessageRequest, onStats func(StreamStats)) (*StreamWithStats, error) {
	iterator, err := c.Stream(ctx, req)
	if err != nil {
		return nil, err
	}

	return &StreamWithStats{
		iterator: iterator,
		stats: StreamStats{
			StartTime: time.Now(),
		},
		onStats: onStats,
	}, nil
}

func (s *StreamWithStats) Next() (StreamEventResult, bool) {
	event, ok := s.iterator.Next()
	if !ok {
		return event, false
	}

	now := time.Now()
	s.stats.ChunkCount++

	if s.stats.FirstChunkTime == nil {
		s.stats.FirstChunkTime = &now
	}
	s.stats.LastChunkTime = &now

	if s.onStats != nil {
		s.onStats(s.stats)
	}

	return event, ok
}

func (s *StreamWithStats) Stats() StreamStats {
	return s.stats
}

func (s *StreamWithStats) Message() *MessageResponse {
	return s.iterator.Message()
}
