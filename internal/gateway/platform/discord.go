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
)

const discordMaxMessageLen = 2000

type DiscordAdapter struct {
	token   string
	gateway *gateway.Gateway
	client  *http.Client
	apiURL  string

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
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
		errCh <- da.pollGateway(ctx, gatewayURL)
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
	slog.Info("discord: adapter stopped")
	return nil
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

func (da *DiscordAdapter) pollGateway(ctx context.Context, gatewayURL string) error {
	wsURL := gatewayURL + "/?v=10&encoding=json"

	req, err := http.NewRequest("GET", wsURL, nil)
	if err != nil {
		return fmt.Errorf("discord: gateway connect error: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+da.token)

	resp, err := da.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord: gateway HTTP error: %w", err)
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	var sessionID string
	var resumeURL string

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-da.stopCh:
			return nil
		default:
		}

		var payload rawGatewayEvent
		if err := decoder.Decode(&payload); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Warn("discord: gateway decode error", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		switch payload.Op {
		case 10:
			var hello struct {
				HeartbeatInterval int `json:"heartbeat_interval"`
			}
			if err := json.Unmarshal(payload.D, &hello); err == nil {
				go da.heartbeat(hello.HeartbeatInterval)
			}
			da.identify()
		case 0:
			switch payload.T {
			case "READY":
				var ready struct {
					SessionID string `json:"session_id"`
					ResumeURL string `json:"resume_gateway_url"`
				}
				if err := json.Unmarshal(payload.D, &ready); err == nil {
					sessionID = ready.SessionID
					resumeURL = ready.ResumeURL
					_ = resumeURL
					slog.Info("discord: session ready", "session_id", sessionID)
				}
			case "MESSAGE_CREATE":
				da.handleMessageCreate(payload.D, ctx)
			}
		case 7:
			slog.Info("discord: reconnect requested")
			if resumeURL != "" {
				return da.pollGateway(ctx, resumeURL)
			}
			newURL, err := da.getGatewayURL()
			if err == nil {
				return da.pollGateway(ctx, newURL)
			}
			return fmt.Errorf("discord: reconnect failed: %w", err)
		case 9:
			slog.Warn("discord: session invalid, re-identifying")
			da.identify()
		}
	}
}

type rawGatewayEvent struct {
	Op int             `json:"op"`
	T  string          `json:"t"`
	S  int             `json:"s"`
	D  json.RawMessage `json:"d"`
}

func (da *DiscordAdapter) heartbeat(intervalMs int) {
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		da.mu.Lock()
		if !da.running {
			da.mu.Unlock()
			return
		}
		da.mu.Unlock()

		payload := map[string]any{"op": 1, "d": nil}
		data, _ := json.Marshal(payload)
		slog.Debug("discord: sending heartbeat")
		_ = data
	}
}

func (da *DiscordAdapter) identify() {
	slog.Debug("discord: identifying bot")
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
