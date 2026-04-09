package models

import (
	"fmt"
	"strings"
)

type Model struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	MaxTokens    int      `json:"max_tokens"`
	InputPrice   float64  `json:"input_price"`
	OutputPrice  float64  `json:"output_price"`
	Capabilities []string `json:"capabilities"`
	Vision       bool     `json:"vision"`
	Tools        bool     `json:"tools"`
}

var availableModels = map[string]Model{
	"claude-opus-4-6": {
		ID:           "claude-opus-4-6",
		Name:         "Claude Opus 4.6",
		Provider:     "anthropic",
		MaxTokens:    16384,
		InputPrice:   15.0,
		OutputPrice:  75.0,
		Capabilities: []string{"text", "vision", "tools", "code"},
		Vision:       true,
		Tools:        true,
	},
	"claude-sonnet-4-5": {
		ID:           "claude-sonnet-4-5",
		Name:         "Claude Sonnet 4.5",
		Provider:     "anthropic",
		MaxTokens:    8192,
		InputPrice:   3.0,
		OutputPrice:  15.0,
		Capabilities: []string{"text", "vision", "tools", "code"},
		Vision:       true,
		Tools:        true,
	},
	"claude-haiku-3-5": {
		ID:           "claude-haiku-3-5",
		Name:         "Claude Haiku 3.5",
		Provider:     "anthropic",
		MaxTokens:    8192,
		InputPrice:   0.25,
		OutputPrice:  1.25,
		Capabilities: []string{"text", "vision", "tools"},
		Vision:       true,
		Tools:        true,
	},
}

func Get(id string) (*Model, error) {
	model, ok := availableModels[id]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", id)
	}
	return &model, nil
}

func List() []Model {
	models := make([]Model, 0, len(availableModels))
	for _, model := range availableModels {
		models = append(models, model)
	}
	return models
}

func Exists(id string) bool {
	_, ok := availableModels[id]
	return ok
}

func CalculateCost(modelID string, inputTokens, outputTokens int) (float64, error) {
	model, err := Get(modelID)
	if err != nil {
		return 0, err
	}
	inputCost := float64(inputTokens) * model.InputPrice / 1_000_000
	outputCost := float64(outputTokens) * model.OutputPrice / 1_000_000
	return inputCost + outputCost, nil
}

func GetDefault() string {
	return "claude-sonnet-4-5"
}

func Validate(modelID string) error {
	if !Exists(modelID) {
		available := make([]string, 0, len(availableModels))
		for id := range availableModels {
			available = append(available, id)
		}
		return fmt.Errorf("unknown model: %s. Available: %s", modelID, strings.Join(available, ", "))
	}
	return nil
}
