package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/gateway"
	"nhooyr.io/websocket"
)

const discordMaxMessageLen = 2000

const (
	discordMaxRetries     = 5
	discordBaseBackoff    = 2 * time.Second
	discordMaxBackoff     = 60 * time.Second

	// Discord gateway opcodes
	opDispatch        = 0
	opHeartbeat       = 1
	opIdentify        = 2
	opReconnect       = 7
	opInvalidSession  = 9
	opHello           = 10
	opHeartbeatAck    = 11
)

type DiscordAdapter struct {
	token   string
	gateway *gateway.Gateway
	client  *http.Client
	apiURL  string

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}

	wsConn         *websocket.Conn
	heartbeatMu    sync.Mutex
	sequenceNum    int
	sessionID      string
	resumeURL      string
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

func NewDiscordAdapter(token string, gw *gateway.Gateway) *DiscordAdapter {
	return &DiscordAdapter{
		token:   token,
		gateway: gw,
		client:  &http.Client{Timeout: 30 * time.Second},
		apiURL:  "https://discord.com/api/v10",
		stopCh:  make(chan struct{}),
	}
}

func (da *DiscordAdapter) SetAPIURL(url string) { da.apiURL = url }

func (da *DiscordAdapter) Name() string { return "discord" }

func (da *DiscordAdapter) Send(userID string, response *gateway.GatewayResponse) error {
	text := response.Content
	if len(text) > discordMaxMessageLen {
		text = text[:discordMaxMessageLen-len("[...truncated]")] + "[...truncated]"
	}
	return da.createMessage(userID, text)
}

func (da *DiscordAdapter) Start(ctx context.Context) error {
	slog.Info("discord: adapter starting")

	da.mu.Lock()
	if da.running {
		da.mu.Unlock()
		return fmt.Errorf("discord: adapter already running")
	}
	da.running = true
	da.stopCh = make(chan struct{})
	da.shutdownCtx, da.shutdownCancel = context.WithCancel(context.Background())
	da.mu.Unlock()

	me, err := da.getCurrentUser()
	if err != nil {
		da.mu.Lock()
		da.running = false
		da.mu.Unlock()
		return fmt.Errorf("discord: get current user failed: %w", err)
	}
	slog.Info("discord: bot connected", "username", me)

	gatewayURL, err := da.getGatewayURL()
	if err != nil {
		da.mu.Lock()
		da.running = false
		da.mu.Unlock()
		return fmt.Errorf("discord: get gateway URL failed: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- da.connectWithRetry(da.shutdownCtx, gatewayURL)
	}()

	select {
	case <-ctx.Done():
		da.Stop()
		return nil
	case <-da.stopCh:
		return nil
	case err := <-errCh:
		da.mu.Lock()
		da.running = false
		da.mu.Unlock()
		return err
	}
}

func (da *DiscordAdapter) Stop() error {
	da.mu.Lock()
	defer da.mu.Unlock()
	if da.running {
		close(da.stopCh)
		da.running = false
	}
	if da.shutdownCancel != nil {
		da.shutdownCancel()
	}
	da.closeWS()
	slog.Info("discord: adapter stopped")
	return nil
}

func (da *DiscordAdapter) closeWS() {
	if da.wsConn != nil {
		da.wsConn.Close(websocket.StatusNormalClosure, "adapter stopping")
		da.wsConn = nil
	}
}

func (da *DiscordAdapter) connectWithRetry(ctx context.Context, gatewayURL string) error {
	for attempt := 0; attempt < discordMaxRetries; attempt++ {
		if attempt > 0 {
			backoff := discordBaseBackoff * time.Duration(1<<(attempt-1))
			if backoff > discordMaxBackoff {
				backoff = discordMaxBackoff
			}
			slog.Info("discord: reconnecting", "attempt", attempt+1, "backoff", backoff)

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
		}

		err := da.pollGateway(ctx, gatewayURL)
		if err == nil {
			return nil
		}

		slog.Warn("discord: gateway connection lost", "error", err)

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	return fmt.Errorf("discord: max retries (%d) exceeded", discordMaxRetries)
}

func (da *DiscordAdapter) pollGateway(ctx context.Context, gatewayURL string) error {
	wsURL := strings.Replace(gatewayURL, "wss://", "wss://", 1)
	if !strings.Contains(wsURL, "?") {
		wsURL += "/?v=10&encoding=json"
	} else {
		wsURL += "&v=10&encoding=json"
	}

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bot " + da.token},
		},
	})
	if err != nil {
		return fmt.Errorf("discord: gateway websocket dial error: %w", err)
	}

	da.mu.Lock()
	da.wsConn = conn
	da.mu.Unlock()

	defer func() {
		da.closeWS()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-da.stopCh:
			return nil
		default:
		}

		_, msgData, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("discord: gateway read error: %w", err)
		}

		var payload rawGatewayEvent
		if err := json.Unmarshal(msgData, &payload); err != nil {
			slog.Warn("discord: gateway decode error", "error", err)
			continue
		}

		// Track sequence number for heartbeats
		if payload.S > 0 {
			da.mu.Lock()
			da.sequenceNum = payload.S
			da.mu.Unlock()
		}

		switch payload.Op {
		case opHello:
			var hello struct {
				HeartbeatInterval int `json:"heartbeat_interval"`
			}
			if err := json.Unmarshal(payload.D, &hello); err == nil {
				go da.heartbeat(ctx, hello.HeartbeatInterval)
			}
			da.identify()
		case opDispatch:
			switch payload.T {
			case "READY":
				var ready struct {
					SessionID string `json:"session_id"`
					ResumeURL string `json:"resume_gateway_url"`
				}
				if err := json.Unmarshal(payload.D, &ready); err == nil {
					da.mu.Lock()
					da.sessionID = ready.SessionID
					da.resumeURL = ready.ResumeURL
					da.mu.Unlock()
					slog.Info("discord: session ready", "session_id", ready.SessionID)
				}
			case "MESSAGE_CREATE":
				da.handleMessageCreate(payload.D, ctx)
			}
		case opReconnect:
			slog.Info("discord: reconnect requested")
			return nil // connectWithRetry will handle reconnection
		case opInvalidSession:
			slog.Warn("discord: session invalid, re-identifying")
			da.identify()
		case opHeartbeatAck:
			slog.Debug("discord: heartbeat acknowledged")
		}
	}
}

type rawGatewayEvent struct {
	Op int             `json:"op"`
	T  string          `json:"t"`
	S  int             `json:"s"`
	D  json.RawMessage `json:"d"`
}

func (da *DiscordAdapter) heartbeat(ctx context.Context, intervalMs int) {
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-da.stopCh:
			return
		case <-ticker.C:
		}

		da.mu.Lock()
		running := da.running
		seq := da.sequenceNum
		da.mu.Unlock()

		if !running {
			return
		}

		da.heartbeatMu.Lock()
		payload := map[string]any{"op": opHeartbeat, "d": seq}
		data, err := json.Marshal(payload)
		if err != nil {
			da.heartbeatMu.Unlock()
			slog.Warn("discord: heartbeat marshal error", "error", err)
			continue
		}

		da.mu.Lock()
		conn := da.wsConn
		da.mu.Unlock()

		if conn != nil {
			if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
				da.heartbeatMu.Unlock()
				slog.Warn("discord: heartbeat send failed", "error", err)
				return
			}
			slog.Debug("discord: heartbeat sent")
		}
		da.heartbeatMu.Unlock()
	}
}

func (da *DiscordAdapter) identify() {
	da.heartbeatMu.Lock()
	defer da.heartbeatMu.Unlock()

	slog.Debug("discord: identifying bot")

	payload := map[string]any{
		"op": opIdentify,
		"d": map[string]any{
			"token": da.token,
			"intents": 1 << 15, // GuildMessages intent (32768)
			"properties": map[string]string{
				"os":      "linux",
				"browser": "smartclaw",
				"device":  "smartclaw",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("discord: identify marshal error", "error", err)
		return
	}

	da.mu.Lock()
	conn := da.wsConn
	da.mu.Unlock()

	if conn == nil {
		slog.Warn("discord: cannot identify, no websocket connection")
		return
	}

	if err := conn.Write(da.shutdownCtx, websocket.MessageText, data); err != nil {
		slog.Warn("discord: identify send failed", "error", err)
		return
	}
	slog.Info("discord: identify payload sent")
}

type discordMessageCreate struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot,omitempty"`
	} `json:"author"`
	Content string `json:"content"`
}

func (da *DiscordAdapter) handleMessageCreate(raw json.RawMessage, ctx context.Context) {
	var msg discordMessageCreate
	if err := json.Unmarshal(raw, &msg); err != nil {
		slog.Warn("discord: failed to parse MESSAGE_CREATE", "error", err)
		return
	}

	if msg.Author.Bot {
		return
	}

	text := strings.TrimSpace(msg.Content)
	if text == "" {
		return
	}

	userID := msg.ChannelID
	slog.Info("discord: received message", "author", msg.Author.Username, "channel", msg.ChannelID, "length", len(text))

	if da.gateway != nil {
		msgCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		resp, err := da.gateway.HandleMessage(msgCtx, userID, "discord", text)
		cancel()
		if err != nil {
			slog.Warn("discord: gateway error", "error", err)
			da.createMessage(msg.ChannelID, fmt.Sprintf("Error: %v", err))
			return
		}
		if resp != nil {
			da.Send(userID, resp)
		}
	}
}

func (da *DiscordAdapter) createMessage(channelID, content string) error {
	payload := map[string]any{
		"content": content,
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/channels/%s/messages", da.apiURL, channelID),
		bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("discord: createMessage request error: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+da.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := da.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord: createMessage HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		body, _ := io.ReadAll(resp.Body)
		var rateLimit struct {
			RetryAfter float64 `json:"retry_after"`
		}
		if json.Unmarshal(body, &rateLimit) == nil && rateLimit.RetryAfter > 0 {
			time.Sleep(time.Duration(rateLimit.RetryAfter * float64(time.Second)))
		} else {
			time.Sleep(5 * time.Second)
		}
		return fmt.Errorf("discord: rate limited, retry after %.0fs", rateLimit.RetryAfter)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord: createMessage status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (da *DiscordAdapter) getCurrentUser() (string, error) {
	req, err := http.NewRequest("GET", da.apiURL+"/users/@me", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bot "+da.token)

	resp, err := da.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("getCurrentUser HTTP error: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("getCurrentUser decode error: %w", err)
	}
	return result.Username, nil
}

func (da *DiscordAdapter) getGatewayURL() (string, error) {
	req, err := http.NewRequest("GET", da.apiURL+"/gateway/bot", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bot "+da.token)

	resp, err := da.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("getGatewayURL HTTP error: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("getGatewayURL decode error: %w", err)
	}
	if result.URL == "" {
		return "", fmt.Errorf("getGatewayURL returned empty URL")
	}
	return result.URL, nil
}
