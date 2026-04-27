package web

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/instructkr/smartclaw/internal/commands"
)

func (h *Handler) handleCommand(client *Client, msg WSMessage) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdErr := commands.Execute(msg.Name, msg.Args)

	w.Close()
	os.Stdout = oldStdout

	output, _ := io.ReadAll(r)
	r.Close()
	outputStr := strings.TrimSpace(string(output))

	if outputStr != "" {
		h.sendToClient(client, WSResponse{Type: "cmd_result", Content: outputStr})
	}

	if cmdErr != nil && outputStr == "" {
		h.sendToClient(client, WSResponse{Type: "error", Message: cmdErr.Error()})
	}

	h.sendToClient(client, WSResponse{Type: "done"})
}

func (h *Handler) handleAbort(client *Client) {
	h.mu.Lock()
	cancel, ok := h.cancelFuncs[client.ID]
	if ok {
		cancel()
		delete(h.cancelFuncs, client.ID)
	}
	h.mu.Unlock()
	h.sendToClient(client, WSResponse{Type: "aborted", Message: "Request aborted"})
}

func (h *Handler) handleModelSwitch(client *Client, msg WSMessage) {
	if msg.Model != "" {
		h.mu.Lock()
		h.clientModels[client.ID] = msg.Model
		h.mu.Unlock()
	}

	model := h.apiClient.Model
	if m, ok := h.clientModels[client.ID]; ok {
		model = m
	}

	h.sendToClient(client, WSResponse{
		Type:    "model_changed",
		Message: fmt.Sprintf("Model switched to %s", model),
	})
}
