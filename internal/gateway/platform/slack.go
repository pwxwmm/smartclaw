package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/instructkr/smartclaw/internal/gateway"
)

type SlackAdapter struct {
	gateway *gateway.Gateway
	token   string
	channel string
	client  *http.Client
	lastTS  string
	botID   string
}

func NewSlackAdapter(gw *gateway.Gateway, token, channel string) *SlackAdapter {
	return &SlackAdapter{
		gateway: gw,
		token:   token,
		channel: channel,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (sa *SlackAdapter) Name() string { return "slack" }

func (sa *SlackAdapter) Send(userID string, response *gateway.GatewayResponse) error {
	channel := userID
	if channel == "" {
		channel = sa.channel
	}

	text := response.Content
	if len(text) > 40000 {
		text = text[:39990] + "\n[...]"
	}

	return sa.postMessage(channel, text)
}

func (sa *SlackAdapter) Start(ctx context.Context) error {
	slog.Info("slack: adapter starting", "channel", sa.channel)

	botID, err := sa.authTest()
	if err != nil {
		return fmt.Errorf("slack: auth.test failed: %w", err)
	}
	sa.botID = botID
	slog.Info("slack: bot connected", "bot_id", botID)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			slog.Info("slack: shutting down")
			return nil
		case <-ctx.Done():
			slog.Info("slack: context cancelled")
			return nil
		case <-ticker.C:
			sa.pollMessages(ctx)
		}
	}
}

func (sa *SlackAdapter) Stop() error {
	slog.Info("slack: adapter stopped")
	return nil
}

func (sa *SlackAdapter) pollMessages(ctx context.Context) {
	messages, err := sa.getConversationHistory(sa.channel, sa.lastTS)
	if err != nil {
		slog.Warn("slack: conversations.history error", "error", err)
		return
	}

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		if msg.BotID != "" {
			continue
		}
		if msg.User == sa.botID {
			continue
		}

		text := strings.TrimSpace(msg.Text)
		if text == "" {
			continue
		}

		sa.lastTS = msg.TS

		userID := msg.User
		if userID == "" {
			continue
		}

		slog.Info("slack: received message", "user", userID, "channel", sa.channel, "length", len(text))

		if sa.gateway != nil {
			msgCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			resp, err := sa.gateway.HandleMessage(msgCtx, userID, "slack", text)
			cancel()
			if err != nil {
				slog.Warn("slack: gateway error", "error", err)
				sa.postMessage(sa.channel, fmt.Sprintf("Error: %v", err))
				continue
			}
			if resp != nil {
				sa.Send(userID, resp)
			}
		}
	}

	if len(messages) > 0 {
		sa.lastTS = messages[0].TS
	}
}

func (sa *SlackAdapter) postMessage(channel, text string) error {
	payload := map[string]any{
		"channel": channel,
		"text":    text,
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("slack: postMessage request error: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+sa.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := sa.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: postMessage HTTP error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack: postMessage status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("slack: postMessage decode error: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("slack: postMessage API error: %s", result.Error)
	}

	return nil
}

func (sa *SlackAdapter) authTest() (string, error) {
	req, err := http.NewRequest("POST", "https://slack.com/api/auth.test", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+sa.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := sa.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool   `json:"ok"`
		UserID string `json:"user_id"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("auth.test error: %s", result.Error)
	}
	return result.UserID, nil
}

type slackMessage struct {
	TS    string `json:"ts"`
	Text  string `json:"text"`
	User  string `json:"user"`
	BotID string `json:"bot_id,omitempty"`
}

func (sa *SlackAdapter) getConversationHistory(channel, oldest string) ([]slackMessage, error) {
	url := fmt.Sprintf("https://slack.com/api/conversations.history?channel=%s&limit=20", channel)
	if oldest != "" {
		tsFloat, err := strconv.ParseFloat(oldest, 64)
		if err == nil {
			url += fmt.Sprintf("&oldest=%.6f", tsFloat)
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+sa.token)

	resp, err := sa.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK       bool           `json:"ok"`
		Messages []slackMessage `json:"messages"`
		Error    string         `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("conversations.history error: %s", result.Error)
	}

	return result.Messages, nil
}

func NewSlackAdapterFromEnv(gw *gateway.Gateway) *SlackAdapter {
	token := os.Getenv("SLACK_BOT_TOKEN")
	if token == "" {
		return nil
	}
	channel := os.Getenv("SLACK_DEFAULT_CHANNEL")
	if channel == "" {
		channel = "general"
	}
	return NewSlackAdapter(gw, token, channel)
}
