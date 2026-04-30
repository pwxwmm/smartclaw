package platform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/instructkr/smartclaw/internal/gateway"
)

const (
	slackMaxSectionLen = 3000
	slackMaxTextLen    = 40000
	slackThreadCutoff  = 500
	slackSummaryLen    = 200
	slackMaxBlocks     = 50
	slackSigMaxAge     = 300 // seconds
)

type ActionButton struct {
	Text     string `json:"text"`
	ActionID string `json:"action_id"`
	Value    string `json:"value"`
	Style    string `json:"style,omitempty"`
}

type ApprovalRequest struct {
	TaskID    string
	ChannelID string
	UserID    string
	Content   string
	CreatedAt time.Time
	MessageTS string
}

type SlackAdapter struct {
	gateway       *gateway.Gateway
	token         string
	channel       string
	signingSecret string
	botID         string
	client        *http.Client
	lastTS        string
	server        *http.Server
	interactivePort string

	pendingApprovals map[string]*ApprovalRequest
	mu               sync.Mutex
}

func NewSlackAdapter(gw *gateway.Gateway, token, channel string) *SlackAdapter {
	signingSecret := os.Getenv("SLACK_SIGNING_SECRET")
	interactivePort := os.Getenv("SLACK_INTERACTIVE_PORT")
	if interactivePort == "" {
		interactivePort = ":8090"
	}

	return &SlackAdapter{
		gateway:         gw,
		token:           token,
		channel:         channel,
		signingSecret:   signingSecret,
		client:          &http.Client{Timeout: 30 * time.Second},
		interactivePort: interactivePort,
		pendingApprovals: make(map[string]*ApprovalRequest),
	}
}

func (sa *SlackAdapter) Name() string { return "slack" }

func (sa *SlackAdapter) Send(userID string, response *gateway.GatewayResponse) error {
	channel := userID
	if channel == "" {
		channel = sa.channel
	}

	// Approval flow
	if response.RequiresApproval {
		return sa.sendApprovalMessage(channel, response)
	}

	text := response.Content
	if len(text) > slackMaxTextLen {
		text = text[:slackMaxTextLen-10] + "\n[...]"
	}

	// Short response: plain text fallback
	if len(text) <= slackThreadCutoff && !containsDiff(text) {
		return sa.postMessage(channel, text, "")
	}

	// Long response: threaded reply
	return sa.sendRichMessage(channel, text)
}

func (sa *SlackAdapter) Start(ctx context.Context) error {
	slog.Info("slack: adapter starting", "channel", sa.channel)

	botID, err := sa.authTest()
	if err != nil {
		return fmt.Errorf("slack: auth.test failed: %w", err)
	}
	sa.botID = botID
	slog.Info("slack: bot connected", "bot_id", botID)

	if err := sa.startInteractiveServer(); err != nil {
		slog.Warn("slack: interactive server failed to start", "error", err)
	}

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
	if sa.server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := sa.server.Shutdown(shutdownCtx); err != nil {
			slog.Warn("slack: server shutdown error", "error", err)
		}
		sa.server = nil
	}
	return nil
}

func buildTextBlock(text string) map[string]any {
	return map[string]any{
		"type": "section",
		"text": map[string]any{
			"type": "mrkdwn",
			"text": text,
		},
	}
}

func buildCodeBlock(code, language string) map[string]any {
	fenced := fmt.Sprintf("```%s\n%s\n```", language, code)
	return map[string]any{
		"type": "section",
		"text": map[string]any{
			"type": "mrkdwn",
			"text": fenced,
		},
	}
}

func buildHeaderBlock(text string) map[string]any {
	return map[string]any{
		"type": "header",
		"text": map[string]any{
			"type":  "plain_text",
			"text":  text,
			"emoji": true,
		},
	}
}

func buildDividerBlock() map[string]any {
	return map[string]any{
		"type": "divider",
	}
}

func buildActionBlock(buttons []ActionButton) map[string]any {
	elements := make([]map[string]any, 0, len(buttons))
	for _, b := range buttons {
		elements = append(elements, map[string]any{
			"type":     "button",
			"text":     map[string]any{"type": "plain_text", "text": b.Text, "emoji": true},
			"action_id": b.ActionID,
			"value":    b.Value,
			"style":    b.Style,
		})
	}
	return map[string]any{
		"type":     "actions",
		"elements": elements,
	}
}

func buildContextBlock(elements []map[string]any) map[string]any {
	return map[string]any{
		"type":     "context",
		"elements": elements,
	}
}

func (sa *SlackAdapter) sendRichMessage(channel, text string) error {
	summary := text
	if len(summary) > slackSummaryLen {
		summary = summary[:slackSummaryLen] + "..."
	}
	headerBlocks := []map[string]any{
		buildHeaderBlock("SmartClaw Response"),
		buildTextBlock(summary),
	}

	headerTS, err := sa.postRichMessage(channel, headerBlocks, "")
	if err != nil {
		return sa.postMessage(channel, text, "")
	}

	blocks := sa.buildBlocksFromText(text)
	if len(blocks) == 0 {
		return nil
	}

	chunks := splitBlocksIntoChunks(blocks, slackMaxBlocks)
	for _, chunk := range chunks {
		if _, err := sa.postRichMessage(channel, chunk, headerTS); err != nil {
			slog.Warn("slack: failed to send threaded block chunk", "error", err)
		}
	}

	return nil
}

func (sa *SlackAdapter) buildBlocksFromText(text string) []map[string]any {
	var blocks []map[string]any

	if containsDiff(text) {
		blocks = append(blocks, sa.buildDiffBlocks(text)...)
		return blocks
	}

	sections := splitTextIntoSections(text, slackMaxSectionLen)
	for _, sec := range sections {
		blocks = append(blocks, buildTextBlock(sec))
		blocks = append(blocks, buildDividerBlock())
	}

	return stripTrailingDivider(blocks)
}

func (sa *SlackAdapter) buildDiffBlocks(text string) []map[string]any {
	var blocks []map[string]any
	lines := strings.Split(text, "\n")

	var currentFile string
	var diffBuf strings.Builder
	inDiff := false

	flushDiff := func() {
		if diffBuf.Len() > 0 {
			diffText := diffBuf.String()
			if currentFile != "" {
				blocks = append(blocks, buildTextBlock("*"+currentFile+"*"))
			}
			chunks := splitTextIntoSections(diffText, slackMaxSectionLen-10)
			for _, chunk := range chunks {
				blocks = append(blocks, buildCodeBlock(chunk, "diff"))
			}
			blocks = append(blocks, buildDividerBlock())
			diffBuf.Reset()
			currentFile = ""
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "+++ ") {
			if inDiff {
				flushDiff()
			}
			inDiff = true
			currentFile = strings.TrimPrefix(line, "+++ ")
			currentFile = strings.TrimPrefix(currentFile, "b/")
			continue
		}
		if strings.HasPrefix(line, "--- ") {
			if !inDiff {
				inDiff = true
			}
			continue
		}

		if inDiff || strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "@@") {
			inDiff = true
			diffBuf.WriteString(line + "\n")
		} else {
			if inDiff {
				flushDiff()
				inDiff = false
			}
			diffBuf.WriteString(line + "\n")
		}
	}
	flushDiff()
	return stripTrailingDivider(blocks)
}

func (sa *SlackAdapter) sendApprovalMessage(channel string, response *gateway.GatewayResponse) error {
	taskID := response.ApprovalTaskID
	if taskID == "" {
		taskID = fmt.Sprintf("task_%d", time.Now().UnixNano())
	}

	content := response.Content
	if len(content) > slackMaxSectionLen {
		content = content[:slackMaxSectionLen-10] + "\n[...]"
	}

	blocks := []map[string]any{
		buildHeaderBlock("Approval Required"),
		buildTextBlock(content),
		buildDividerBlock(),
		buildActionBlock([]ActionButton{
			{
				Text:     "Approve",
				ActionID: "smartclaw_approve_" + taskID,
				Value:    taskID,
				Style:    "primary",
			},
			{
				Text:     "Deny",
				ActionID: "smartclaw_deny_" + taskID,
				Value:    taskID,
				Style:    "danger",
			},
		}),
		buildContextBlock([]map[string]any{
			{
				"type": "mrkdwn",
				"text": fmt.Sprintf("Task: `%s` | Requested: %s", taskID, time.Now().Format(time.RFC3339)),
			},
		}),
	}

	ts, err := sa.postRichMessage(channel, blocks, "")
	if err != nil {
		return err
	}

	sa.mu.Lock()
	sa.pendingApprovals[taskID] = &ApprovalRequest{
		TaskID:    taskID,
		ChannelID: channel,
		Content:   response.Content,
		CreatedAt: time.Now(),
		MessageTS: ts,
	}
	sa.mu.Unlock()

	return nil
}

func (sa *SlackAdapter) postMessage(channel, text, threadTS string) error {
	payload := map[string]any{
		"channel": channel,
		"text":    text,
	}
	if threadTS != "" {
		payload["thread_ts"] = threadTS
	}
	data, _ := json.Marshal(payload)

	return sa.doSlackAPIPost("https://slack.com/api/chat.postMessage", data)
}

func (sa *SlackAdapter) postRichMessage(channel string, blocks []map[string]any, threadTS string) (string, error) {
	fallbackText := "SmartClaw message"
	for _, b := range blocks {
		if b["type"] == "section" {
			if textObj, ok := b["text"].(map[string]any); ok {
				if t, ok := textObj["text"].(string); ok {
					fallbackText = t
					break
				}
			}
		} else if b["type"] == "header" {
			if textObj, ok := b["text"].(map[string]any); ok {
				if t, ok := textObj["text"].(string); ok {
					fallbackText = t
					break
				}
			}
		}
	}

	payload := map[string]any{
		"channel": channel,
		"text":    fallbackText,
		"blocks":  blocks,
	}
	if threadTS != "" {
		payload["thread_ts"] = threadTS
	}
	data, _ := json.Marshal(payload)

	return sa.doSlackAPIPostWithResponse("https://slack.com/api/chat.postMessage", data)
}

func (sa *SlackAdapter) updateRichMessage(channel, ts string, blocks []map[string]any) error {
	payload := map[string]any{
		"channel": channel,
		"ts":      ts,
		"blocks":  blocks,
		"text":    "Updated",
	}
	data, _ := json.Marshal(payload)
	return sa.doSlackAPIPost("https://slack.com/api/chat.update", data)
}

func (sa *SlackAdapter) doSlackAPIPost(apiURL string, data []byte) error {
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("slack: request error: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+sa.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := sa.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("slack: decode error: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("slack: API error: %s", result.Error)
	}
	return nil
}

func (sa *SlackAdapter) doSlackAPIPostWithResponse(apiURL string, data []byte) (string, error) {
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("slack: request error: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+sa.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := sa.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("slack: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("slack: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
		TS    string `json:"ts,omitempty"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("slack: decode error: %w", err)
	}
	if !result.OK {
		return "", fmt.Errorf("slack: API error: %s", result.Error)
	}
	return result.TS, nil
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

		if text == "help" || text == "/smartclaw help" {
			sa.sendHelpMessage(sa.channel)
			continue
		}

		if sa.gateway != nil {
			msgCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			resp, err := sa.gateway.HandleMessage(msgCtx, userID, "slack", text)
			cancel()
			if err != nil {
				slog.Warn("slack: gateway error", "error", err)
				sa.sendErrorMessage(sa.channel, fmt.Sprintf("Error: %v", err))
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

func (sa *SlackAdapter) getConversationHistory(channel, oldest string) ([]slackMessage, error) {
	apiURL := fmt.Sprintf("https://slack.com/api/conversations.history?channel=%s&limit=20", channel)
	if oldest != "" {
		tsFloat, err := strconv.ParseFloat(oldest, 64)
		if err == nil {
			apiURL += fmt.Sprintf("&oldest=%.6f", tsFloat)
		}
	}

	req, err := http.NewRequest("GET", apiURL, nil)
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

func (sa *SlackAdapter) sendErrorMessage(channel, errMsg string) error {
	blocks := []map[string]any{
		buildTextBlock("❌ " + errMsg),
	}
	_, err := sa.postRichMessage(channel, blocks, "")
	return err
}

func (sa *SlackAdapter) sendSuccessMessage(channel, msg string) error {
	blocks := []map[string]any{
		buildTextBlock("✅ " + msg),
	}
	_, err := sa.postRichMessage(channel, blocks, "")
	return err
}

func (sa *SlackAdapter) sendHelpMessage(channel string) error {
	blocks := []map[string]any{
		buildHeaderBlock("SmartClaw Commands"),
		buildDividerBlock(),
		buildTextBlock("• `/smartclaw fix <description>` — Fix a bug or issue\n• `/smartclaw review` — Review recent code changes\n• `/smartclaw deploy` — Run deployment workflow\n• `/smartclaw status` — Show agent status and recent activity\n• `/smartclaw help` — Show this help message"),
		buildDividerBlock(),
		buildContextBlock([]map[string]any{
			{
				"type": "mrkdwn",
				"text": "You can also send messages directly to the bot for general questions.",
			},
		}),
	}
	_, err := sa.postRichMessage(channel, blocks, "")
	return err
}

func (sa *SlackAdapter) startInteractiveServer() error {
	if sa.signingSecret == "" {
		slog.Warn("slack: SLACK_SIGNING_SECRET not set, interactive server disabled")
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/slack/events", sa.handleEvents)
	mux.HandleFunc("/slack/interact", sa.handleInteract)
	mux.HandleFunc("/slack/commands", sa.handleCommands)

	sa.server = &http.Server{
		Addr:    sa.interactivePort,
		Handler: mux,
	}

	go func() {
		slog.Info("slack: interactive server starting", "addr", sa.interactivePort)
		if err := sa.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Warn("slack: interactive server error", "error", err)
		}
	}()

	return nil
}

// verifySlackSignature verifies HMAC-SHA256 signature per Slack spec:
// v0=HMAC_SHA256(signing_secret, "v0:timestamp:body")
func (sa *SlackAdapter) verifySlackSignature(body []byte, timestamp, signature string) bool {
	if sa.signingSecret == "" {
		return false
	}

	// Reject requests older than 5 minutes
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-ts > slackSigMaxAge {
		return false
	}

	sigBase := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(sa.signingSecret))
	mac.Write([]byte(sigBase))
	expectedSig := "v0=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

func (sa *SlackAdapter) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")
	if !sa.verifySlackSignature(body, timestamp, signature) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var challengeReq struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(body, &challengeReq); err == nil && challengeReq.Type == "url_verification" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"challenge": challengeReq.Challenge})
		return
	}

	var eventPayload struct {
		Type string `json:"type"`
		Event struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			User    string `json:"user"`
			Channel string `json:"channel"`
			BotID   string `json:"bot_id,omitempty"`
		} `json:"event"`
	}
	if err := json.Unmarshal(body, &eventPayload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)

	if eventPayload.Type == "event_callback" && eventPayload.Event.Type == "message" {
		ev := eventPayload.Event
		if ev.BotID != "" || ev.User == sa.botID {
			return
		}

		text := strings.TrimSpace(ev.Text)
		if text == "" {
			return
		}

		slog.Info("slack: received event message", "user", ev.User, "channel", ev.Channel)

		if sa.gateway != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			resp, err := sa.gateway.HandleMessage(ctx, ev.User, "slack", text)
			cancel()
			if err != nil {
				slog.Warn("slack: gateway error from event", "error", err)
				sa.sendErrorMessage(ev.Channel, fmt.Sprintf("Error: %v", err))
				return
			}
			if resp != nil {
				sa.Send(ev.User, resp)
			}
		}
	}
}

func (sa *SlackAdapter) handleInteract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")
	if !sa.verifySlackSignature(body, timestamp, signature) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	payloadStr := r.FormValue("payload")

	var payload struct {
		Type      string `json:"type"`
		Actions   []struct {
			ActionID string `json:"action_id"`
			Value    string `json:"value"`
		} `json:"actions"`
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
		User struct {
			ID string `json:"id"`
		} `json:"user"`
		Message struct {
			TS string `json:"ts"`
		} `json:"message"`
		ResponseURL string `json:"response_url"`
	}
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)

	if payload.Type != "block_actions" || len(payload.Actions) == 0 {
		return
	}

	action := payload.Actions[0]
	actionID := action.ActionID
	taskID := action.Value

	sa.mu.Lock()
	approval, exists := sa.pendingApprovals[taskID]
	if exists {
		delete(sa.pendingApprovals, taskID)
	}
	sa.mu.Unlock()

	if !exists {
		slog.Warn("slack: unknown approval task", "task_id", taskID)
		return
	}

	channel := payload.Channel.ID
	if channel == "" {
		channel = approval.ChannelID
	}
	messageTS := payload.Message.TS
	if messageTS == "" {
		messageTS = approval.MessageTS
	}

	switch {
	case strings.HasPrefix(actionID, "smartclaw_approve_"):
		updatedBlocks := []map[string]any{
			buildTextBlock("✅ *Approved* by <@" + payload.User.ID + ">"),
			buildDividerBlock(),
			buildTextBlock(approval.Content),
		}
		if err := sa.updateRichMessage(channel, messageTS, updatedBlocks); err != nil {
			slog.Warn("slack: failed to update approval message", "error", err)
		}
		slog.Info("slack: approval granted", "task_id", taskID, "user", payload.User.ID)

	case strings.HasPrefix(actionID, "smartclaw_deny_"):
		updatedBlocks := []map[string]any{
			buildTextBlock("❌ *Denied* by <@" + payload.User.ID + ">"),
			buildDividerBlock(),
			buildTextBlock(approval.Content),
		}
		if err := sa.updateRichMessage(channel, messageTS, updatedBlocks); err != nil {
			slog.Warn("slack: failed to update denial message", "error", err)
		}
		slog.Info("slack: approval denied", "task_id", taskID, "user", payload.User.ID)
	}
}

func (sa *SlackAdapter) handleCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")
	if !sa.verifySlackSignature(body, timestamp, signature) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	command := values.Get("command")
	text := strings.TrimSpace(values.Get("text"))
	channelID := values.Get("channel_id")
	userID := values.Get("user_id")
	responseURL := values.Get("response_url")

	slog.Info("slack: slash command received", "command", command, "text", text, "user", userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"response_type": "ephemeral",
		"text":          "Processing your request...",
	})

	var gatewayText string
	switch {
	case strings.HasPrefix(text, "fix"):
		gatewayText = "fix " + strings.TrimPrefix(text, "fix ")
	case strings.HasPrefix(text, "review"):
		gatewayText = "review " + strings.TrimPrefix(text, "review ")
	case strings.HasPrefix(text, "deploy"):
		gatewayText = "deploy " + strings.TrimPrefix(text, "deploy ")
	case text == "status":
		gatewayText = "status"
	case text == "help":
		sa.sendHelpMessage(channelID)
		return
	default:
		gatewayText = text
	}

	if gatewayText == "" {
		gatewayText = "help"
	}

	if sa.gateway != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		resp, err := sa.gateway.HandleMessage(ctx, userID, "slack", gatewayText)
		cancel()
		if err != nil {
			sa.sendErrorMessage(channelID, fmt.Sprintf("Error: %v", err))
			return
		}
		if resp != nil {
			sa.Send(userID, resp)
		}
	}

	if responseURL != "" {
		sa.postToResponseURL(responseURL, "✅ Command processed")
	}
}

func (sa *SlackAdapter) postToResponseURL(responseURL, text string) {
	payload := map[string]string{
		"text": text,
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", responseURL, bytes.NewReader(data))
	if err != nil {
		slog.Warn("slack: response_url request error", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := sa.client.Do(req)
	if err != nil {
		slog.Warn("slack: response_url HTTP error", "error", err)
		return
	}
	resp.Body.Close()
}

func containsDiff(text string) bool {
	return strings.Contains(text, "+++") || strings.Contains(text, "---") || strings.Contains(text, "@@")
}

func splitTextIntoSections(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var sections []string
	remaining := text

	for len(remaining) > maxLen {
		breakPoint := maxLen
		if idx := strings.LastIndex(remaining[:maxLen], "\n"); idx > maxLen/2 {
			breakPoint = idx + 1
		}

		sections = append(sections, remaining[:breakPoint])
		remaining = remaining[breakPoint:]
	}

	if len(remaining) > 0 {
		sections = append(sections, remaining)
	}

	return sections
}

func splitBlocksIntoChunks(blocks []map[string]any, maxBlocks int) [][]map[string]any {
	if len(blocks) <= maxBlocks {
		return [][]map[string]any{blocks}
	}

	var chunks [][]map[string]any
	for i := 0; i < len(blocks); i += maxBlocks {
		end := i + maxBlocks
		if end > len(blocks) {
			end = len(blocks)
		}
		chunks = append(chunks, blocks[i:end])
	}
	return chunks
}

// stripTrailingDivider removes a trailing divider block from the blocks array.
func stripTrailingDivider(blocks []map[string]any) []map[string]any {
	for len(blocks) > 0 {
		last := blocks[len(blocks)-1]
		if typ, _ := last["type"].(string); typ == "divider" {
			blocks = blocks[:len(blocks)-1]
		} else {
			break
		}
	}
	return blocks
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
