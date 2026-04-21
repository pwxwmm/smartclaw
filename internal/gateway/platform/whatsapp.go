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

const whatsappMaxMessageLen = 4096

type WhatsAppAdapter struct {
	phoneNumberID string
	accessToken   string
	gateway       *gateway.Gateway
	client        *http.Client
	apiURL        string
	verifyToken   string
	webhookAddr   string

	mu      sync.Mutex
	running bool
	server  *http.Server
}

func NewWhatsAppAdapter(phoneNumberID, accessToken string, gw *gateway.Gateway) *WhatsAppAdapter {
	return &WhatsAppAdapter{
		phoneNumberID: phoneNumberID,
		accessToken:   accessToken,
		gateway:       gw,
		client:        &http.Client{Timeout: 30 * time.Second},
		apiURL:        "https://graph.facebook.com/v18.0",
		verifyToken:   "smartclaw_verify",
		webhookAddr:   ":8089",
	}
}

func (wa *WhatsAppAdapter) SetAPIURL(url string) { wa.apiURL = url }

func (wa *WhatsAppAdapter) WithWebhook(addr, verifyToken string) *WhatsAppAdapter {
	wa.webhookAddr = addr
	wa.verifyToken = verifyToken
	return wa
}

func (wa *WhatsAppAdapter) Name() string { return "whatsapp" }

func (wa *WhatsAppAdapter) Send(userID string, response *gateway.GatewayResponse) error {
	text := response.Content
	if len(text) > whatsappMaxMessageLen {
		text = text[:whatsappMaxMessageLen-len("[...truncated]")] + "[...truncated]"
	}
	return wa.sendMessage(userID, text)
}

func (wa *WhatsAppAdapter) Start(ctx context.Context) error {
	slog.Info("whatsapp: adapter starting", "webhook_addr", wa.webhookAddr)

	wa.mu.Lock()
	if wa.running {
		wa.mu.Unlock()
		return fmt.Errorf("whatsapp: adapter already running")
	}
	wa.running = true
	wa.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/whatsapp", wa.handleWebhook)
	mux.HandleFunc("/webhook/whatsapp/verify", wa.handleVerify)

	wa.server = &http.Server{
		Addr:    wa.webhookAddr,
		Handler: mux,
	}

	go func() {
		if err := wa.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Warn("whatsapp: webhook server error", "error", err)
		}
	}()

	<-ctx.Done()
	return wa.Stop()
}

func (wa *WhatsAppAdapter) Stop() error {
	wa.mu.Lock()
	defer wa.mu.Unlock()
	wa.running = false

	if wa.server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := wa.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("whatsapp: server shutdown error: %w", err)
		}
		wa.server = nil
	}
	slog.Info("whatsapp: adapter stopped")
	return nil
}

func (wa *WhatsAppAdapter) sendMessage(recipient, text string) error {
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                recipient,
		"type":              "text",
		"text": map[string]string{
			"body": text,
		},
	}
	data, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/%s/messages", wa.apiURL, wa.phoneNumberID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("whatsapp: sendMessage request error: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+wa.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := wa.client.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: sendMessage HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp: sendMessage status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (wa *WhatsAppAdapter) handleVerify(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == wa.verifyToken {
		slog.Info("whatsapp: webhook verified")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, challenge)
		return
	}
	w.WriteHeader(http.StatusForbidden)
}

func (wa *WhatsAppAdapter) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Warn("whatsapp: failed to read webhook body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload struct {
		Entry []struct {
			Changes []struct {
				Value struct {
					Messages []struct {
						From string `json:"from"`
						Text struct {
							Body string `json:"body"`
						} `json:"text"`
						Type string `json:"type"`
					} `json:"messages"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Warn("whatsapp: failed to parse webhook payload", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, msg := range change.Value.Messages {
				if msg.Type != "text" {
					continue
				}
				text := strings.TrimSpace(msg.Text.Body)
				if text == "" {
					continue
				}

				userID := msg.From
				slog.Info("whatsapp: received message", "from", userID, "length", len(text))

				if wa.gateway != nil {
					ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
					resp, err := wa.gateway.HandleMessage(ctx, userID, "whatsapp", text)
					cancel()
					if err != nil {
						slog.Warn("whatsapp: gateway error", "error", err)
						continue
					}
					if resp != nil {
						wa.Send(userID, resp)
					}
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}
