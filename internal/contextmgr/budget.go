package contextmgr

type SourcePriority struct {
	Source    string
	Weight    float64
	MinTokens int
	MaxTokens int
}

type TokenBudget struct {
	Total    int
	Used     int
	BySource map[string]int
}

func NewTokenBudget(total int) *TokenBudget {
	return &TokenBudget{
		Total:    total,
		BySource: make(map[string]int),
	}
}

func (tb *TokenBudget) Remaining() int {
	return tb.Total - tb.Used
}

func (tb *TokenBudget) Record(source string, tokens int) {
	tb.BySource[source] += tokens
	tb.Used += tokens
}

// DefaultSourcePriorities returns the standard priority configuration.
// High-weight sources (system prompt, conversation) are preserved under tight
// budgets; low-weight sources (git, search) are trimmed first.
func DefaultSourcePriorities() []SourcePriority {
	return []SourcePriority{
		{Source: "system_prompt", Weight: 1.0, MinTokens: 500, MaxTokens: 2000},
		{Source: "conversation", Weight: 0.9, MinTokens: 1000, MaxTokens: 8000},
		{Source: "files", Weight: 0.7, MinTokens: 0, MaxTokens: 6000},
		{Source: "memory", Weight: 0.5, MinTokens: 0, MaxTokens: 3000},
		{Source: "skills", Weight: 0.4, MinTokens: 0, MaxTokens: 2000},
		{Source: "search", Weight: 0.3, MinTokens: 0, MaxTokens: 2000},
		{Source: "git", Weight: 0.2, MinTokens: 0, MaxTokens: 1500},
	}
}

// Allocate distributes totalTokens across sources proportional to their
// weights, respecting min/max constraints via a two-pass approach:
// pass 1: satisfy all minimums; pass 2: distribute remainder proportionally, clamped by max.
func Allocate(totalTokens int, sources []SourcePriority) map[string]int {
	result := make(map[string]int, len(sources))

	used := 0
	for _, sp := range sources {
		alloc := sp.MinTokens
		if alloc > totalTokens-used {
			alloc = totalTokens - used
		}
		if alloc < 0 {
			alloc = 0
		}
		result[sp.Source] = alloc
		used += alloc
	}

	remaining := totalTokens - used
	if remaining <= 0 {
		return result
	}

	totalWeight := 0.0
	for _, sp := range sources {
		if result[sp.Source] < sp.MaxTokens {
			totalWeight += sp.Weight
		}
	}

	if totalWeight == 0 {
		return result
	}

	for _, sp := range sources {
		current := result[sp.Source]
		if current >= sp.MaxTokens {
			continue
		}
		proportional := int(float64(remaining) * (sp.Weight / totalWeight))
		newAlloc := current + proportional
		if newAlloc > sp.MaxTokens {
			newAlloc = sp.MaxTokens
		}
		result[sp.Source] = newAlloc
	}

	return result
}
