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

const signalMaxMessageLen = 4096

type SignalAdapter struct {
	signalCLIURL string
	phoneNumber  string
	gateway      *gateway.Gateway
	client       *http.Client

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

func NewSignalAdapter(signalCLIURL, phoneNumber string, gw *gateway.Gateway) *SignalAdapter {
	return &SignalAdapter{
		signalCLIURL: strings.TrimRight(signalCLIURL, "/"),
		phoneNumber:  phoneNumber,
		gateway:      gw,
		client:       &http.Client{Timeout: 30 * time.Second},
		stopCh:       make(chan struct{}),
	}
}

func (sa *SignalAdapter) Name() string { return "signal" }

func (sa *SignalAdapter) Send(userID string, response *gateway.GatewayResponse) error {
	text := response.Content
	if len(text) > signalMaxMessageLen {
		text = text[:signalMaxMessageLen-len("[...truncated]")] + "[...truncated]"
	}
	return sa.sendMessage(userID, text)
}

func (sa *SignalAdapter) Start(ctx context.Context) error {
	slog.Info("signal: adapter starting", "cli_url", sa.signalCLIURL, "phone", sa.phoneNumber)

	sa.mu.Lock()
	if sa.running {
		sa.mu.Unlock()
		return fmt.Errorf("signal: adapter already running")
	}
	sa.running = true
	sa.stopCh = make(chan struct{})
	sa.mu.Unlock()

	if err := sa.registerWebhook(); err != nil {
		slog.Warn("signal: failed to register webhook, falling back to polling", "error", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			sa.Stop()
			return nil
		case <-sa.stopCh:
			return nil
		case <-ticker.C:
			sa.pollMessages(ctx)
		}
	}
}

func (sa *SignalAdapter) Stop() error {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	if sa.running {
		close(sa.stopCh)
		sa.running = false
	}
	slog.Info("signal: adapter stopped")
	return nil
}

func (sa *SignalAdapter) sendMessage(recipient, text string) error {
	payload := map[string]any{
		"message": text,
		"number":  sa.phoneNumber,
		"recipients": []string{recipient},
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", sa.signalCLIURL+"/v2/send", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("signal: sendMessage request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := sa.client.Do(req)
	if err != nil {
		return fmt.Errorf("signal: sendMessage HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("signal: sendMessage status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (sa *SignalAdapter) registerWebhook() error {
	url := fmt.Sprintf("%s/v1/register/%s", sa.signalCLIURL, sa.phoneNumber)
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return fmt.Errorf("signal: register webhook request error: %w", err)
	}

	resp, err := sa.client.Do(req)
	if err != nil {
		return fmt.Errorf("signal: register webhook HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("signal: register webhook status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (sa *SignalAdapter) pollMessages(ctx context.Context) {
	url := fmt.Sprintf("%s/v1/receive/%s", sa.signalCLIURL, sa.phoneNumber)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Warn("signal: poll request error", "error", err)
		return
	}

	resp, err := sa.client.Do(req)
	if err != nil {
		slog.Warn("signal: poll HTTP error", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		slog.Warn("signal: poll status error", "status", resp.StatusCode, "body", string(body))
		return
	}

	var messages []struct {
		Envelope struct {
			Source          string `json:"source"`
			SourceNumber    string `json:"sourceNumber"`
			DataMessage     *struct {
				Message string `json:"message"`
			} `json:"dataMessage"`
			SyncMessage *struct {
				SentMessage *struct {
					Message string `json:"message"`
				} `json:"sentMessage"`
			} `json:"syncMessage"`
		} `json:"envelope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		slog.Warn("signal: poll decode error", "error", err)
		return
	}

	for _, msg := range messages {
		var text string
		var sender string

		if msg.Envelope.DataMessage != nil && msg.Envelope.DataMessage.Message != "" {
			text = msg.Envelope.DataMessage.Message
			sender = msg.Envelope.SourceNumber
			if sender == "" {
				sender = msg.Envelope.Source
			}
		} else if msg.Envelope.SyncMessage != nil && msg.Envelope.SyncMessage.SentMessage != nil {
			continue
		}

		text = strings.TrimSpace(text)
		if text == "" || sender == sa.phoneNumber {
			continue
		}

		slog.Info("signal: received message", "from", sender, "length", len(text))

		if sa.gateway != nil {
			msgCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			resp, err := sa.gateway.HandleMessage(msgCtx, sender, "signal", text)
			cancel()
			if err != nil {
				slog.Warn("signal: gateway error", "error", err)
				sa.sendMessage(sender, fmt.Sprintf("Error: %v", err))
				continue
			}
			if resp != nil {
				sa.Send(sender, resp)
			}
		}
	}
}
