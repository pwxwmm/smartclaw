package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.Reader
	mu     sync.Mutex
	ready  bool
}

func NewStdioTransport(config *McpServerConfig) *StdioTransport {
	cmd := exec.Command(config.Command, config.Args...)

	if len(config.Env) > 0 {
		env := cmd.Env
		for k, v := range config.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	return &StdioTransport{
		cmd: cmd,
	}
}

func (t *StdioTransport) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.ready {
		return nil
	}

	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	t.stdin = stdin
	t.stdout = bufio.NewReader(stdout)
	t.stderr = stderr

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	t.ready = true
	return nil
}

func (t *StdioTransport) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready {
		return nil
	}

	if err := t.stdin.Close(); err != nil {
		return err
	}

	if err := t.cmd.Process.Kill(); err != nil {
		return err
	}

	t.ready = false
	return nil
}

func (t *StdioTransport) Send(msg *JSONRPCRequest) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready {
		return fmt.Errorf("transport not ready")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := t.stdin.Write([]byte(header)); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	if _, err := t.stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

func (t *StdioTransport) Receive() (*JSONRPCResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready {
		return nil, fmt.Errorf("transport not ready")
	}

	var length int
	for {
		line, err := t.stdout.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read header: %w", err)
		}

		if line == "\r\n" {
			break
		}

		var n int
		if _, err := fmt.Sscanf(line, "Content-Length: %d\r\n", &n); err == nil {
			length = n
		}
	}

	if length == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(t.stdout, data); err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	return ParseJSONRPCResponse(data)
}

func (t *StdioTransport) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ready
}

type SSETransport struct {
	url          string
	client       *http.Client
	mu           sync.Mutex
	ready        bool
	sseEndpoint  string
	messageQueue chan *JSONRPCResponse
	cancel       context.CancelFunc
}

func NewSSETransport(url string) *SSETransport {
	return &SSETransport{
		url:          url,
		client:       &http.Client{Timeout: 60 * time.Second},
		messageQueue: make(chan *JSONRPCResponse, 100),
	}
}

func (t *SSETransport) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ready = true
	return nil
}

func (t *SSETransport) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ready = false
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

func (t *SSETransport) Send(msg *JSONRPCRequest) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready {
		return fmt.Errorf("transport not ready")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	endpoint := t.sseEndpoint
	if endpoint == "" {
		endpoint = t.url
	}

	resp, err := t.client.Post(endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (t *SSETransport) Receive() (*JSONRPCResponse, error) {
	select {
	case msg := <-t.messageQueue:
		return msg, nil
	default:
		return nil, fmt.Errorf("no messages available, use ReceiveWithContext for blocking receive")
	}
}

func (t *SSETransport) ReceiveWithContext(ctx context.Context) (*JSONRPCResponse, error) {
	select {
	case msg := <-t.messageQueue:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *SSETransport) StartSSE(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready {
		return fmt.Errorf("transport not ready")
	}

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	req, err := http.NewRequestWithContext(ctx, "GET", t.url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE endpoint: %w", err)
	}

	go t.readSSEStream(ctx, resp)

	return nil
}

func (t *SSETransport) readSSEStream(ctx context.Context, resp *http.Response) {
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var eventType, eventData strings.Builder

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		if line == "" {
			if eventData.Len() > 0 {
				t.processSSEEvent(eventType.String(), eventData.String())
				eventType.Reset()
				eventData.Reset()
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
			eventType.WriteString(value)
		case "data":
			if eventData.Len() > 0 {
				eventData.WriteString("\n")
			}
			eventData.WriteString(value)
		case "endpoint":
			t.mu.Lock()
			t.sseEndpoint = value
			t.mu.Unlock()
		}
	}
}

func (t *SSETransport) processSSEEvent(eventType, data string) {
	if data == "" {
		return
	}

	var response JSONRPCResponse
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		return
	}

	select {
	case t.messageQueue <- &response:
	default:
	}
}

func (t *SSETransport) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ready
}

func (t *SSETransport) GetSSEEndpoint() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sseEndpoint
}
