package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/instructkr/smartclaw/internal/api"
	"nhooyr.io/websocket"
)

// APIClient abstracts what the TUI needs from the API backend,
// allowing swap between local api.Client and remote HTTP/WebSocket client.
type APIClient interface {
	StreamMessage(ctx context.Context, req *api.MessageRequest, handler func(event string, data []byte) error) error
	SendMessage(ctx context.Context, req *api.MessageRequest) (*api.MessageResponse, error)
	SetModel(model string)
	GetModel() string
	IsOpenAI() bool
	SetOpenAI(isOpenAI bool)
	Close() error
}
var (
	_ APIClient = (*LocalClient)(nil)
	_ APIClient = (*RemoteClient)(nil)
)

// LocalClient wraps *api.Client, delegating StreamMessage to StreamMessageCtx
// which handles Anthropic/OpenAI/Google routing internally.
type LocalClient struct {
	client *api.Client
}

func NewLocalClient(client *api.Client) *LocalClient {
	return &LocalClient{client: client}
}

func (c *LocalClient) StreamMessage(ctx context.Context, req *api.MessageRequest, handler func(event string, data []byte) error) error {
	return c.client.StreamMessageCtx(ctx, req, handler)
}

func (c *LocalClient) SendMessage(ctx context.Context, req *api.MessageRequest) (*api.MessageResponse, error) {
	return c.client.CreateMessageWithSystem(ctx, req.Messages, req.System)
}

func (c *LocalClient) SetModel(model string) {
	c.client.SetModel(model)
}

func (c *LocalClient) GetModel() string {
	return c.client.GetModel()
}

func (c *LocalClient) IsOpenAI() bool {
	return c.client.IsOpenAI
}

func (c *LocalClient) SetOpenAI(isOpenAI bool) {
	c.client.SetOpenAI(isOpenAI)
}

func (c *LocalClient) Close() error {
	return nil
}

// RemoteClient connects to a smartclaw serve instance via HTTP+WebSocket.
// Provider routing runs server-side, so IsOpenAI/SetOpenAI are advisory.
type RemoteClient struct {
	baseURL    string
	token      string
	wsURL      string
	httpClient *http.Client
	model      string
	isOpenAI   bool
}

func NewRemoteClient(baseURL, token string) *RemoteClient {
	wsURL := baseURL
	if len(wsURL) > 4 && wsURL[:5] == "https" {
		wsURL = "wss" + wsURL[5:]
	} else if len(wsURL) > 3 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	}

	return &RemoteClient{
		baseURL:    baseURL,
		token:      token,
		wsURL:      wsURL,
		httpClient: &http.Client{},
	}
}

func (c *RemoteClient) StreamMessage(ctx context.Context, req *api.MessageRequest, handler func(event string, data []byte) error) error {
	wsEndpoint := fmt.Sprintf("%s/ws?token=%s", c.wsURL, c.token)

	conn, _, err := websocket.Dial(ctx, wsEndpoint, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, reqBody); err != nil {
		return fmt.Errorf("websocket write: %w", err)
	}

	for {
		_, msgData, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			break
		}

		var wsEvent struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(msgData, &wsEvent); err != nil {
			if handlerErr := handler("", msgData); handlerErr != nil {
				return handlerErr
			}
			continue
		}

		if wsEvent.Type == "done" {
			break
		}

		if wsEvent.Type == "error" {
			return fmt.Errorf("server error: %s", string(wsEvent.Data))
		}

		if handlerErr := handler(wsEvent.Type, wsEvent.Data); handlerErr != nil {
			return handlerErr
		}

		if wsEvent.Type == "message_stop" {
			break
		}
	}

	return nil
}

func (c *RemoteClient) SendMessage(ctx context.Context, req *api.MessageRequest) (*api.MessageResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	var msgResp api.MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &msgResp, nil
}

func (c *RemoteClient) SetModel(model string) {
	c.model = model
}

func (c *RemoteClient) GetModel() string {
	return c.model
}

func (c *RemoteClient) IsOpenAI() bool {
	return c.isOpenAI
}

func (c *RemoteClient) SetOpenAI(isOpenAI bool) {
	c.isOpenAI = isOpenAI
}

func (c *RemoteClient) Close() error {
	return nil
}
