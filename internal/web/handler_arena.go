package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/routing"
)

type arenaChatPayload struct {
	Content string   `json:"content"`
	Models  []string `json:"models"`
}

type arenaVotePayload struct {
	WinnerModel string `json:"winner_model"`
	LoserModel  string `json:"loser_model"`
	Prompt      string `json:"prompt"`
}

type arenaStartMsg struct {
	Type   string   `json:"type"`
	Models []string `json:"models"`
}

type arenaResultMsg struct {
	Type    string              `json:"type"`
	Results []*routing.ArenaResult `json:"results"`
}

var defaultArenaModels = []string{"sre-model", "glm-4-plus"}

func (h *Handler) handleArenaChat(client *Client, msg WSMessage) {
	if h.apiClient == nil {
		h.sendError(client, "API client not configured")
		return
	}

	var payload arenaChatPayload
	if len(msg.Data) > 0 {
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			if msg.Content != "" {
				payload.Content = msg.Content
			} else {
				h.sendError(client, "Invalid arena_chat payload")
				return
			}
		}
	} else {
		payload.Content = msg.Content
	}

	if payload.Content == "" {
		h.sendError(client, "Arena chat requires a prompt")
		return
	}

	models := payload.Models
	if len(models) < 2 {
		models = defaultArenaModels
	}

	clients := make(map[string]*api.Client, len(models))
	for _, model := range models {
		c, err := h.router.ClientForModel(model)
		if err != nil {
			c = api.NewClientWithModel(h.apiClient.APIKey, h.apiClient.BaseURL, model)
			c.IsOpenAI = h.apiClient.IsOpenAI
		}
		clients[model] = c
	}

	labels := make([]string, len(models))
	for i := range models {
		labels[i] = fmt.Sprintf("Model %c", 'A'+i)
	}

	h.sendToClient(client, WSResponse{
		Type: "arena_start",
		Data: map[string]any{"models": labels},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	executor := routing.NewArenaExecutor(clients)

	messages := []api.Message{
		{Role: "user", Content: payload.Content},
	}

	var systemPrompt string
	if h.memMgr != nil {
		userMem := h.memMgr.ForUser(client.UserID)
		systemPrompt = userMem.BuildPrompt()
	} else if h.prompt != nil {
		systemPrompt = h.prompt.Build()
	}

	results := executor.Execute(ctx, messages, systemPrompt, client.SessionID)

	h.sendToClient(client, WSResponse{
		Type: "arena_result",
		Data: map[string]any{"results": results},
	})

	slog.Info("arena chat completed",
		"client_id", client.ID,
		"models", models,
		"prompt_len", len(payload.Content),
	)
}

func (h *Handler) handleArenaVote(client *Client, msg WSMessage) {
	var payload arenaVotePayload
	if len(msg.Data) > 0 {
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			h.sendError(client, "Invalid arena_vote payload")
			return
		}
	}

	if payload.WinnerModel == "" || payload.LoserModel == "" {
		h.sendError(client, "arena_vote requires winner_model and loser_model")
		return
	}

	vote := routing.ArenaVote{
		WinnerModel: payload.WinnerModel,
		LoserModel:  payload.LoserModel,
		Prompt:      payload.Prompt,
		Timestamp:   time.Now(),
	}

	arenaGlobalVotes.Record(vote)

	if h.modelRouter != nil {
		arenaGlobalVotes.ApplyToRouter(h.modelRouter)
	}

	h.sendToClient(client, WSResponse{
		Type:    "arena_vote_recorded",
		Message: fmt.Sprintf("Vote recorded: %s over %s", payload.WinnerModel, payload.LoserModel),
	})

	slog.Info("arena vote recorded",
		"client_id", client.ID,
		"winner", payload.WinnerModel,
		"loser", payload.LoserModel,
	)
}

var arenaGlobalVotes = routing.NewArenaVotes()
