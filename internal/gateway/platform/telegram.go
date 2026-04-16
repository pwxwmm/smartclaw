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

// TelegramAdapter delivers responses to users via the Telegram Bot API.
// It also runs a long-poll loop to receive incoming messages and route
// them through the Gateway.
type TelegramAdapter struct {
	token   string
	apiURL  string
	gateway *gateway.Gateway
	client  *http.Client
	offset  int64
}

func NewTelegramAdapter(token string, gw *gateway.Gateway) *TelegramAdapter {
	return &TelegramAdapter{
		token:   token,
		apiURL:  fmt.Sprintf("https://api.telegram.org/bot%s", token),
		gateway: gw,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (ta *TelegramAdapter) Name() string { return "telegram" }

func (ta *TelegramAdapter) Send(userID string, response *gateway.GatewayResponse) error {
	chatID, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat ID %q: %w", userID, err)
	}

	text := response.Content
	if len(text) > 4096 {
		text = text[:4090] + "\n[...]"
	}

	return ta.sendMessage(chatID, text)
}

// Run starts the long-poll loop for incoming messages. Blocks until
// interrupted by signal or the gateway closes.
func (ta *TelegramAdapter) Run() error {
	slog.Info("telegram: adapter starting", "api_url", ta.apiURL[:30]+"...")

	me, err := ta.getMe()
	if err != nil {
		return fmt.Errorf("telegram: getMe failed: %w", err)
	}
	slog.Info("telegram: bot connected", "username", me)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigCh:
			slog.Info("telegram: shutting down")
			return nil
		default:
		}

		updates, err := ta.getUpdates(ta.offset, 30)
		if err != nil {
			slog.Warn("telegram: getUpdates error", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, update := range updates {
			ta.offset = update.UpdateID + 1

			if update.Message == nil || update.Message.Text == "" {
				continue
			}

			chatID := update.Message.Chat.ID
			userID := strconv.FormatInt(chatID, 10)
			text := strings.TrimSpace(update.Message.Text)

			if strings.HasPrefix(text, "/") {
				ta.handleCommand(chatID, text)
				continue
			}

			slog.Info("telegram: received message", "chat_id", chatID, "length", len(text))

			if ta.gateway != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				resp, err := ta.gateway.HandleMessage(ctx, userID, "telegram", text)
				if err != nil {
					slog.Warn("telegram: gateway error", "error", err)
					ta.sendMessage(chatID, fmt.Sprintf("Error: %v", err))
					continue
				}
				if resp != nil {
					ta.Send(userID, resp)
				}
			}
		}
	}
}

func (ta *TelegramAdapter) handleCommand(chatID int64, text string) {
	parts := strings.SplitN(text, " ", 2)
	cmd := parts[0]

	switch cmd {
	case "/start", "/help":
		ta.sendMessage(chatID, "SmartClaw is ready. Send me a message and I'll help you with coding tasks.")
	case "/status":
		ta.sendMessage(chatID, "SmartClaw is running.")
	default:
		if len(parts) > 1 {
			ta.sendMessage(chatID, "Unknown command. Send a message to start a conversation.")
		}
	}
}

func (ta *TelegramAdapter) sendMessage(chatID int64, text string) error {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	data, _ := json.Marshal(payload)

	resp, err := ta.client.Post(ta.apiURL+"/sendMessage", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("telegram: sendMessage HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram: sendMessage status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (ta *TelegramAdapter) getMe() (string, error) {
	resp, err := ta.client.Get(ta.apiURL + "/getMe")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("getMe returned not ok")
	}
	return result.Result.Username, nil
}

type telegramUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

func (ta *TelegramAdapter) getUpdates(offset int64, timeout int) ([]telegramUpdate, error) {
	payload := map[string]any{
		"offset":  offset,
		"timeout": timeout,
	}
	data, _ := json.Marshal(payload)

	resp, err := ta.client.Post(ta.apiURL+"/getUpdates", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool             `json:"ok"`
		Result []telegramUpdate `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("getUpdates returned not ok")
	}
	return result.Result, nil
}
