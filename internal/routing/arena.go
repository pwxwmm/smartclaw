package routing

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/utils"
)

// ArenaResult holds the response from one model in an arena comparison.
type ArenaResult struct {
	Label    string    `json:"label"`     // "Model A", "Model B"
	Model    string    `json:"model"`     // Hidden until reveal
	Content  string    `json:"content"`   // Full text response
	Tokens   Usage     `json:"tokens"`    // Token usage
	Duration int64     `json:"duration_ms"`
	Error    string    `json:"error,omitempty"`
}

// Usage mirrors api.Usage for JSON serialization in arena results.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ArenaVote records a user preference between two models.
type ArenaVote struct {
	WinnerModel string    `json:"winner_model"`
	LoserModel  string    `json:"loser_model"`
	Prompt      string    `json:"prompt"`
	Timestamp   time.Time `json:"timestamp"`
}

// arenaModelResult is an internal type for collecting parallel results.
type arenaModelResult struct {
	label    string
	model    string
	resp     *api.MessageResponse
	duration time.Duration
	err      error
}

// ArenaVotes holds in-memory arena votes for the MVP.
type ArenaVotes struct {
	mu    sync.Mutex
	votes []ArenaVote
}

// NewArenaVotes creates a new ArenaVotes store.
func NewArenaVotes() *ArenaVotes {
	return &ArenaVotes{
		votes: make([]ArenaVote, 0),
	}
}

// Record stores an arena vote.
func (av *ArenaVotes) Record(v ArenaVote) {
	av.mu.Lock()
	defer av.mu.Unlock()
	av.votes = append(av.votes, v)
}

// All returns all recorded votes.
func (av *ArenaVotes) All() []ArenaVote {
	av.mu.Lock()
	defer av.mu.Unlock()
	out := make([]ArenaVote, len(av.votes))
	copy(out, av.votes)
	return out
}

func (av *ArenaVotes) ApplyToRouter(router *ModelRouter) {
	if router == nil {
		return
	}
	votes := av.All()
	for _, v := range votes {
		router.RecordOutcome(v.WinnerModel, 0.5, true, false, 0)
		router.RecordOutcome(v.LoserModel, 0.5, false, false, 0)
	}
}

// ArenaExecutor dispatches the same prompt to multiple models in parallel
// and returns all results with anonymized labels.
type ArenaExecutor struct {
	clients map[string]*api.Client // model name -> client
	votes   *ArenaVotes
}

// NewArenaExecutor creates a new ArenaExecutor with the given model clients.
func NewArenaExecutor(clients map[string]*api.Client) *ArenaExecutor {
	return &ArenaExecutor{
		clients: clients,
		votes:   NewArenaVotes(),
	}
}

// Execute dispatches the query to all configured models in parallel.
// It returns results with anonymized labels ("Model A", "Model B", etc.).
// For MVP, only 2 models are supported.
func (ae *ArenaExecutor) Execute(ctx context.Context, messages []api.Message, systemPrompt string, sessionID string) []*ArenaResult {
	models := make([]string, 0, len(ae.clients))
	for m := range ae.clients {
		models = append(models, m)
	}

	if len(models) < 2 {
		models = ensureTwoModels(models)
	}
	models = models[:2] // MVP: max 2

	ch := make(chan arenaModelResult, len(models))
	var wg sync.WaitGroup

	for i, model := range models {
		client, ok := ae.clients[model]
		if !ok {
			ch <- arenaModelResult{
				label: labelForIndex(i),
				model: model,
				err:   errNoClient(model),
			}
			continue
		}

		wg.Add(1)
		idx := i
		m := model
		c := client
		utils.Go(func() {
			defer wg.Done()
			start := time.Now()
			resp, err := c.CreateMessageWithSystem(ctx, messages, systemPrompt)
			ch <- arenaModelResult{
				label:    labelForIndex(idx),
				model:    m,
				resp:     resp,
				duration: time.Since(start),
				err:      err,
			}
		})
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	results := make([]*ArenaResult, 0, len(models))
	for r := range ch {
		ar := &ArenaResult{
			Label:    r.label,
			Model:    r.model,
			Duration: r.duration.Milliseconds(),
		}
		var tokenInput, tokenOutput int
		if r.err != nil {
			ar.Error = r.err.Error()
		} else if r.resp != nil {
			ar.Content = extractArenaText(r.resp)
			tokenInput = r.resp.Usage.InputTokens
			tokenOutput = r.resp.Usage.OutputTokens
			ar.Tokens = Usage{
				InputTokens:  tokenInput,
				OutputTokens: tokenOutput,
			}
		}
		observability.RecordOutboundAudit(&observability.OutboundAuditEntry{
			Provider:        "arena",
			DestinationHost: "arena-comparison",
			Model:           r.model,
			MessageCount:    len(messages),
			InputTokens:     tokenInput,
			OutputTokens:    tokenOutput,
			DataCategories:  []string{"arena_comparison"},
			Duration:        r.duration.Milliseconds(),
			SessionID:       sessionID,
		})
		results = append(results, ar)
	}

	return results
}

// RecordVote stores a user's preference vote.
func (ae *ArenaExecutor) RecordVote(v ArenaVote) {
	ae.votes.Record(v)
}

// GetVotes returns all recorded votes.
func (ae *ArenaExecutor) GetVotes() []ArenaVote {
	return ae.votes.All()
}

func labelForIndex(i int) string {
	return "Model " + string(rune('A'+i))
}

func ensureTwoModels(models []string) []string {
	for len(models) < 2 {
		models = append(models, "unknown")
	}
	return models
}

func errNoClient(model string) error {
	return &arenaError{model: model}
}

type arenaError struct {
	model string
}

func (e *arenaError) Error() string {
	return "no client configured for model: " + e.model
}

// extractArenaText extracts the full text content from a MessageResponse.
func extractArenaText(resp *api.MessageResponse) string {
	if resp == nil {
		return ""
	}
	var sb strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
}
