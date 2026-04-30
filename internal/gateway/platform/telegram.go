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
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/instructkr/smartclaw/internal/gateway"
	"github.com/instructkr/smartclaw/internal/voice"
)

// TelegramAdapter delivers responses to users via the Telegram Bot API.
// It also runs a long-poll loop to receive incoming messages and route
// them through the Gateway.
type TelegramAdapter struct {
	token        string
	apiURL       string
	gateway      *gateway.Gateway
	client       *http.Client
	offset       int64
	voiceManager *voice.VoiceManager
}

func NewTelegramAdapter(token string, gw *gateway.Gateway, voiceConfig *voice.VoiceConfig) *TelegramAdapter {
	ta := &TelegramAdapter{
		token:   token,
		apiURL:  fmt.Sprintf("https://api.telegram.org/bot%s", token),
		gateway: gw,
		client:  &http.Client{Timeout: 60 * time.Second},
	}

	if voiceConfig != nil {
		ta.voiceManager = voice.NewVoiceManager(*voiceConfig)
	}

	return ta
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

			if update.Message == nil {
				continue
			}

			chatID := update.Message.Chat.ID
			userID := strconv.FormatInt(chatID, 10)

			if update.Message.Text != "" {
				text := strings.TrimSpace(update.Message.Text)

				if strings.HasPrefix(text, "/") {
					ta.handleCommand(chatID, text)
					continue
				}

				slog.Info("telegram: received message", "chat_id", chatID, "length", len(text))
				ta.routeToGateway(userID, chatID, text)
				continue
			}

			if update.Message.Voice != nil {
				ta.handleVoiceMessage(userID, chatID, update.Message.Voice.FileID)
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

func (ta *TelegramAdapter) routeToGateway(userID string, chatID int64, text string) {
	if ta.gateway == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	resp, err := ta.gateway.HandleMessage(ctx, userID, "telegram", text)
	if err != nil {
		slog.Warn("telegram: gateway error", "error", err)
		ta.sendMessage(chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	if resp != nil {
		ta.Send(userID, resp)
	}
}

func (ta *TelegramAdapter) handleVoiceMessage(userID string, chatID int64, fileID string) {
	if ta.voiceManager == nil {
		slog.Warn("telegram: voice message received but voice not configured")
		ta.sendMessage(chatID, "Voice messages are not supported in this configuration.")
		return
	}

	slog.Info("telegram: processing voice message", "chat_id", chatID)

	filePath, err := ta.getFile(fileID)
	if err != nil {
		slog.Warn("telegram: getFile failed", "error", err)
		ta.sendMessage(chatID, "Could not transcribe voice message")
		return
	}

	oggData, err := ta.downloadFile(filePath)
	if err != nil {
		slog.Warn("telegram: download failed", "error", err)
		ta.sendMessage(chatID, "Could not transcribe voice message")
		return
	}

	oggFile, err := os.CreateTemp("", "telegram-voice-*.ogg")
	if err != nil {
		slog.Warn("telegram: temp file creation failed", "error", err)
		ta.sendMessage(chatID, "Could not transcribe voice message")
		return
	}
	oggPath := oggFile.Name()
	defer os.Remove(oggPath)

	if _, err := oggFile.Write(oggData); err != nil {
		oggFile.Close()
		slog.Warn("telegram: temp file write failed", "error", err)
		ta.sendMessage(chatID, "Could not transcribe voice message")
		return
	}
	oggFile.Close()

	wavPath, converted, err := ta.convertOggToWav(oggPath)
	if converted && wavPath != "" {
		defer os.Remove(wavPath)
	}
	if err != nil {
		slog.Warn("telegram: ogg-to-wav conversion failed", "error", err)
		ta.sendMessage(chatID, "Could not transcribe voice message")
		return
	}

	transcribePath := oggPath
	if wavPath != "" {
		transcribePath = wavPath
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := ta.voiceManager.TranscribeFile(ctx, transcribePath)
	if err != nil {
		slog.Warn("telegram: transcription failed", "error", err)
		ta.sendMessage(chatID, "Could not transcribe voice message")
		return
	}

	text := strings.TrimSpace(result.Text)
	if text == "" {
		slog.Warn("telegram: transcription returned empty text")
		ta.sendMessage(chatID, "Could not transcribe voice message")
		return
	}

	slog.Info("telegram: voice transcribed", "chat_id", chatID, "length", len(text))
	ta.routeToGateway(userID, chatID, text)
}

func (ta *TelegramAdapter) getFile(fileID string) (string, error) {
	resp, err := ta.client.Get(fmt.Sprintf("%s/getFile?file_id=%s", ta.apiURL, fileID))
	if err != nil {
		return "", fmt.Errorf("getFile HTTP error: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("getFile decode error: %w", err)
	}
	if !result.OK {
		return "", fmt.Errorf("getFile returned not ok")
	}
	return result.Result.FilePath, nil
}

func (ta *TelegramAdapter) downloadFile(filePath string) ([]byte, error) {
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", ta.token, filePath)
	resp, err := ta.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("download read error: %w", err)
	}
	return data, nil
}

func (ta *TelegramAdapter) convertOggToWav(oggPath string) (wavPath string, converted bool, err error) {
	ffmpegPath, lookupErr := exec.LookPath("ffmpeg")
	if lookupErr != nil {
		slog.Info("telegram: ffmpeg not found, passing ogg directly to transcriber")
		return "", false, nil
	}

	wavFile, err := os.CreateTemp("", "telegram-voice-*.wav")
	if err != nil {
		return "", false, fmt.Errorf("wav temp file creation failed: %w", err)
	}
	wavPath = wavFile.Name()
	wavFile.Close()

	cmd := exec.Command(ffmpegPath, "-y", "-i", oggPath, "-ar", "16000", "-ac", "1", "-f", "wav", wavPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(wavPath)
		return "", false, fmt.Errorf("ffmpeg conversion failed: %w, stderr: %s", err, stderr.String())
	}

	return wavPath, true, nil
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
		Text    string `json:"text"`
		Caption string `json:"caption"`
		Voice   *struct {
			FileID string `json:"file_id"`
		} `json:"voice"`
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
