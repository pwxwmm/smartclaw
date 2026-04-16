package services

import (
	"strings"

	"github.com/instructkr/smartclaw/internal/utils"
)

type TokenEstimator struct {
	avgCharsPerToken int
}

func NewTokenEstimator() *TokenEstimator {
	return &TokenEstimator{
		avgCharsPerToken: 4,
	}
}

func (e *TokenEstimator) Estimate(text string) int {
	if len(text) == 0 {
		return 0
	}

	charCount := len(text)
	return charCount / e.avgCharsPerToken
}

func (e *TokenEstimator) EstimateMessages(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += e.Estimate(msg.Content)
		total += 4
	}
	return total
}

func (e *TokenEstimator) EstimateWithContext(text string, contextTokens int) int {
	return e.Estimate(text) + contextTokens
}

type CostCalculator struct {
	inputPricePerMillion  float64
	outputPricePerMillion float64
}

func NewCostCalculator(inputPrice, outputPrice float64) *CostCalculator {
	return &CostCalculator{
		inputPricePerMillion:  inputPrice,
		outputPricePerMillion: outputPrice,
	}
}

func (c *CostCalculator) Calculate(inputTokens, outputTokens int) float64 {
	inputCost := float64(inputTokens) * c.inputPricePerMillion / 1_000_000
	outputCost := float64(outputTokens) * c.outputPricePerMillion / 1_000_000
	return inputCost + outputCost
}

func CountTokens(text string) int {
	return utils.CountTokens(text)
}

func SplitByTokens(text string, maxTokens int) []string {
	estimator := NewTokenEstimator()
	charsPerChunk := maxTokens * estimator.avgCharsPerToken

	if len(text) <= charsPerChunk {
		return []string{text}
	}

	chunks := []string{}
	for i := 0; i < len(text); i += charsPerChunk {
		end := i + charsPerChunk
		if end > len(text) {
			end = len(text)
		}

		chunk := text[i:end]

		if end < len(text) {
			lastNewline := strings.LastIndex(chunk, "\n")
			if lastNewline > charsPerChunk/2 {
				chunk = text[i : i+lastNewline+1]
				i = i + lastNewline + 1 - charsPerChunk
			}
		}

		chunks = append(chunks, chunk)
	}

	return chunks
}
