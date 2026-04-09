package transports

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type SSETransport struct {
	endpoint  string
	headers   map[string]string
	client    *http.Client
	resp      *http.Response
	connected bool
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewSSETransport(cfg *Config) *SSETransport {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60
	}

	return &SSETransport{
		endpoint: cfg.Endpoint,
		headers:  cfg.Headers,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

func (t *SSETransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, "GET", t.endpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	t.resp = resp
	t.connected = true
	t.ctx, t.cancel = context.WithCancel(ctx)

	return nil
}

func (t *SSETransport) Send(message []byte) error {
	return fmt.Errorf("SSE is read-only transport")
}

func (t *SSETransport) Receive() ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected || t.resp == nil {
		return nil, fmt.Errorf("not connected")
	}

	reader := bufio.NewReader(t.resp.Body)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				t.connected = false
				return nil, err
			}
			return nil, err
		}

		line = trimNewline(line)

		if len(line) == 0 {
			continue
		}

		if isDataLine(line) {
			return extractData(line), nil
		}
	}
}

func (t *SSETransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
	}

	if t.resp != nil {
		t.resp.Body.Close()
		t.resp = nil
	}

	t.connected = false
	return nil
}

func (t *SSETransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

func trimNewline(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	if len(data) > 0 && data[len(data)-1] == '\r' {
		data = data[:len(data)-1]
	}
	return data
}

func isDataLine(line []byte) bool {
	return len(line) > 5 && string(line[:5]) == "data:"
}

func extractData(line []byte) []byte {
	if len(line) > 5 {
		return line[6:]
	}
	return line
}
