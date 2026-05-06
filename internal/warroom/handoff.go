package warroom

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// HandoffRequest represents a structured request from one agent to another.
type HandoffRequest struct {
	ID       string          `json:"id"`
	FromAgent DomainAgentType `json:"from_agent"`
	ToAgent   DomainAgentType `json:"to_agent"`
	Question  string          `json:"question"`
	Context   string          `json:"context"`
	Priority  string          `json:"priority"` // low, medium, high
}

// HandoffResponse represents the answer to a handoff request.
type HandoffResponse struct {
	RequestID  string          `json:"request_id"`
	FromAgent  DomainAgentType `json:"from_agent"`
	ToAgent    DomainAgentType `json:"to_agent"`
	Answer     string          `json:"answer"`
	Confidence float64         `json:"confidence"`
}

type HandoffManager struct {
	mu         sync.Mutex
	requests   map[string]chan HandoffRequest
	responses  map[string]map[string]chan HandoffResponse
}

func NewHandoffManager() *HandoffManager {
	return &HandoffManager{
		requests:  make(map[string]chan HandoffRequest),
		responses: make(map[string]map[string]chan HandoffResponse),
	}
}

func (h *HandoffManager) CreateSession(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.requests[sessionID] = make(chan HandoffRequest, 16)
	h.responses[sessionID] = make(map[string]chan HandoffResponse)
}

func (h *HandoffManager) CloseSession(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.requests, sessionID)
	delete(h.responses, sessionID)
}

func (h *HandoffManager) RequestHandoff(ctx context.Context, sessionID string, req HandoffRequest) (*HandoffResponse, error) {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	h.mu.Lock()
	reqCh, ok := h.requests[sessionID]
	if !ok {
		h.mu.Unlock()
		return nil, fmt.Errorf("handoff: session not found: %s", sessionID)
	}

	respCh := make(chan HandoffResponse, 1)
	if h.responses[sessionID] == nil {
		h.responses[sessionID] = make(map[string]chan HandoffResponse)
	}
	h.responses[sessionID][req.ID] = respCh
	h.mu.Unlock()

	select {
	case reqCh <- req:
	case <-ctx.Done():
		h.mu.Lock()
		delete(h.responses[sessionID], req.ID)
		h.mu.Unlock()
		return nil, ctx.Err()
	}

	timeout := 30 * time.Second
	select {
	case resp := <-respCh:
		h.mu.Lock()
		delete(h.responses[sessionID], req.ID)
		h.mu.Unlock()
		return &resp, nil
	case <-time.After(timeout):
		h.mu.Lock()
		delete(h.responses[sessionID], req.ID)
		h.mu.Unlock()
		return nil, fmt.Errorf("handoff: timeout waiting for response from %s", req.ToAgent)
	case <-ctx.Done():
		h.mu.Lock()
		delete(h.responses[sessionID], req.ID)
		h.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (h *HandoffManager) SendResponse(sessionID string, resp HandoffResponse) error {
	h.mu.Lock()
	respMap, ok := h.responses[sessionID]
	if !ok {
		h.mu.Unlock()
		return fmt.Errorf("handoff: session not found: %s", sessionID)
	}
	respCh, ok := respMap[resp.RequestID]
	if !ok {
		h.mu.Unlock()
		return fmt.Errorf("handoff: no pending request with ID %s", resp.RequestID)
	}
	h.mu.Unlock()

	select {
	case respCh <- resp:
	default:
		slog.Warn("handoff: response channel full, dropping response", "request_id", resp.RequestID)
	}
	return nil
}

func (h *HandoffManager) PendingRequests(sessionID string) <-chan HandoffRequest {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.requests[sessionID]
}

func (h *HandoffManager) TryRecvRequest(sessionID string) (HandoffRequest, bool) {
	h.mu.Lock()
	reqCh := h.requests[sessionID]
	h.mu.Unlock()

	if reqCh == nil {
		return HandoffRequest{}, false
	}

	select {
	case req := <-reqCh:
		return req, true
	default:
		return HandoffRequest{}, false
	}
}
