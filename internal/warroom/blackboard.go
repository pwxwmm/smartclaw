package warroom

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// BlackboardEntry is a key-value observation written by an agent.
type BlackboardEntry struct {
	Key       string          `json:"key"`
	Value     string          `json:"value"`
	Author    DomainAgentType `json:"author"`
	Timestamp time.Time       `json:"timestamp"`
	Category  string          `json:"category"` // observation, metric, log_excerpt, hypothesis
}

// Hypothesis is a shared hypothesis that agents can add to and refine.
type Hypothesis struct {
	ID                    string          `json:"id"`
	Description           string          `json:"description"`
	ProposedBy            DomainAgentType `json:"proposed_by"`
	Confidence            float64         `json:"confidence"`
	SupportingEvidence    []string        `json:"supporting_evidence"`
	ContradictingEvidence []string        `json:"contradicting_evidence"`
	Status                string          `json:"status"` // proposed, confirmed, refuted
}

// SharedFact is a confirmed fact visible to all agents.
type SharedFact struct {
	Content     string            `json:"content"`
	Source      DomainAgentType   `json:"source"`
	ConfirmedBy []DomainAgentType `json:"confirmed_by"`
	Confidence  float64           `json:"confidence"`
}

// Blackboard acts as a shared mutable context store for a War Room session.
// Phase 1 agents write observations and hypotheses to it, and other agents
// can read them in real time to inform their own investigation.
type Blackboard struct {
	sessionID   string
	mu          sync.RWMutex
	entries     map[string]BlackboardEntry
	hypotheses  []Hypothesis
	sharedFacts []SharedFact
}

// NewBlackboard creates a new blackboard for the given session.
func NewBlackboard(sessionID string) *Blackboard {
	return &Blackboard{
		sessionID:   sessionID,
		entries:     make(map[string]BlackboardEntry),
		hypotheses:  []Hypothesis{},
		sharedFacts: []SharedFact{},
	}
}

// WriteEntry writes an observation to the blackboard. If the key already exists,
// it is overwritten with the new entry.
func (b *Blackboard) WriteEntry(entry BlackboardEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	b.entries[entry.Key] = entry
}

// ReadEntries returns all blackboard entries, optionally filtered by category.
// If category is empty, all entries are returned.
func (b *Blackboard) ReadEntries(category string) []BlackboardEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]BlackboardEntry, 0, len(b.entries))
	for _, e := range b.entries {
		if category == "" || e.Category == category {
			result = append(result, e)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result
}

// AddHypothesis adds a new hypothesis to the blackboard.
func (b *Blackboard) AddHypothesis(h Hypothesis) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if h.Status == "" {
		h.Status = "proposed"
	}
	b.hypotheses = append(b.hypotheses, h)
}

// UpdateHypothesis updates an existing hypothesis identified by its ID.
// Returns true if the hypothesis was found and updated.
func (b *Blackboard) UpdateHypothesis(id string, fn func(*Hypothesis)) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := range b.hypotheses {
		if b.hypotheses[i].ID == id {
			fn(&b.hypotheses[i])
			return true
		}
	}
	return false
}

// AddSharedFact adds a new shared fact to the blackboard.
func (b *Blackboard) AddSharedFact(fact SharedFact) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sharedFacts = append(b.sharedFacts, fact)
}

// ConfirmFact confirms a shared fact by adding the confirming agent to the
// confirmed-by list. Returns true if the fact was found.
func (b *Blackboard) ConfirmFact(content string, confirmedBy DomainAgentType) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := range b.sharedFacts {
		if b.sharedFacts[i].Content == content {
			for _, c := range b.sharedFacts[i].ConfirmedBy {
				if c == confirmedBy {
					return true // already confirmed
				}
			}
			b.sharedFacts[i].ConfirmedBy = append(b.sharedFacts[i].ConfirmedBy, confirmedBy)
			return true
		}
	}
	return false
}

// GetSnapshot returns the full blackboard state as structured text for
// injecting into agent prompts.
func (b *Blackboard) GetSnapshot() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("=== Shared Blackboard Context ===\n")

	if len(b.sharedFacts) > 0 {
		sb.WriteString("\n--- Confirmed Facts ---\n")
		for i, f := range b.sharedFacts {
			confirmers := strings.Join(agentTypeNamesSlice(f.ConfirmedBy), ", ")
			sb.WriteString(fmt.Sprintf("%d. [%s] %s (confirmed by: %s, confidence: %.2f)\n",
				i+1, f.Source, f.Content, confirmers, f.Confidence))
		}
	}

	if len(b.entries) > 0 {
		sb.WriteString("\n--- Observations ---\n")
		entries := make([]BlackboardEntry, 0, len(b.entries))
		for _, e := range b.entries {
			entries = append(entries, e)
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Timestamp.Before(entries[j].Timestamp)
		})
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("[%s][%s] %s: %s\n", e.Author, e.Category, e.Key, e.Value))
		}
	}

	if len(b.hypotheses) > 0 {
		sb.WriteString("\n--- Hypotheses ---\n")
		for _, h := range b.hypotheses {
			sb.WriteString(fmt.Sprintf("[%s] %s (status: %s, confidence: %.2f)\n",
				h.ProposedBy, h.Description, h.Status, h.Confidence))
			if len(h.SupportingEvidence) > 0 {
				sb.WriteString("  Supporting: " + strings.Join(h.SupportingEvidence, "; ") + "\n")
			}
			if len(h.ContradictingEvidence) > 0 {
				sb.WriteString("  Contradicting: " + strings.Join(h.ContradictingEvidence, "; ") + "\n")
			}
		}
	}

	if len(b.entries) == 0 && len(b.hypotheses) == 0 && len(b.sharedFacts) == 0 {
		sb.WriteString("(No shared context yet)\n")
	}

	sb.WriteString("=== End Blackboard ===\n")
	return sb.String()
}

// GetHypotheses returns a copy of all hypotheses.
func (b *Blackboard) GetHypotheses() []Hypothesis {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]Hypothesis, len(b.hypotheses))
	copy(result, b.hypotheses)
	return result
}

// GetSharedFacts returns a copy of all shared facts.
func (b *Blackboard) GetSharedFacts() []SharedFact {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]SharedFact, len(b.sharedFacts))
	copy(result, b.sharedFacts)
	return result
}

// agentTypeNamesSlice converts a slice of DomainAgentType to string names.
func agentTypeNamesSlice(types []DomainAgentType) []string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return names
}
