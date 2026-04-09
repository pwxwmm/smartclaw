package services

import (
	"context"
)

type CompactService struct {
	threshold int
}

func NewCompactService(threshold int) *CompactService {
	if threshold <= 0 {
		threshold = 100000
	}
	return &CompactService{threshold: threshold}
}

func (s *CompactService) ShouldCompact(tokenCount int) bool {
	return tokenCount >= s.threshold
}

func (s *CompactService) GetThreshold() int {
	return s.threshold
}

func (s *CompactService) SetThreshold(threshold int) {
	s.threshold = threshold
}

type Message struct {
	Role    string
	Content string
}

func (s *CompactService) Compact(ctx context.Context, messages []Message) ([]Message, error) {
	if len(messages) <= 2 {
		return messages, nil
	}

	compacted := make([]Message, 0, len(messages)/2)
	compacted = append(compacted, messages[0])

	for i := 1; i < len(messages)-1; i += 2 {
		if i+1 < len(messages)-1 {
			compacted = append(compacted, Message{
				Role:    "assistant",
				Content: "[compacted]",
			})
		}
	}

	compacted = append(compacted, messages[len(messages)-1])
	return compacted, nil
}

type AutoCompact struct {
	service   *CompactService
	maxTokens int
}

func NewAutoCompact(service *CompactService, maxTokens int) *AutoCompact {
	if maxTokens <= 0 {
		maxTokens = 200000
	}
	return &AutoCompact{
		service:   service,
		maxTokens: maxTokens,
	}
}

func (a *AutoCompact) CheckAndCompact(ctx context.Context, messages []Message, currentTokens int) ([]Message, bool, error) {
	if !a.service.ShouldCompact(currentTokens) {
		return messages, false, nil
	}

	compacted, err := a.service.Compact(ctx, messages)
	if err != nil {
		return messages, false, err
	}

	return compacted, true, nil
}

type MicroCompact struct {
	service *CompactService
}

func NewMicroCompact(service *CompactService) *MicroCompact {
	return &MicroCompact{service: service}
}

func (m *MicroCompact) CompactRecent(ctx context.Context, messages []Message, keepLast int) ([]Message, error) {
	if len(messages) <= keepLast {
		return messages, nil
	}

	result := make([]Message, 0, keepLast+2)
	result = append(result, messages[0])
	result = append(result, Message{
		Role:    "assistant",
		Content: "[earlier messages compacted]",
	})
	result = append(result, messages[len(messages)-keepLast:]...)

	return result, nil
}
