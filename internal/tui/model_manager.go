package tui

import (
	"fmt"
	"strings"
)

type ModelInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	ContextSize  int      `json:"context_size"`
	InputCost    float64  `json:"input_cost"`   // per 1M tokens
	OutputCost   float64  `json:"output_cost"`  // per 1M tokens
	Speed        string   `json:"speed"`        // fast, medium, slow
	Intelligence string   `json:"intelligence"` // low, medium, high
	Features     []string `json:"features"`
	Description  string   `json:"description"`
}

var AvailableModels = []ModelInfo{
	{
		ID:           "claude-opus-4-6",
		Name:         "Claude Opus 4.6",
		Provider:     "Anthropic",
		ContextSize:  200000,
		InputCost:    15.0,
		OutputCost:   75.0,
		Speed:        "slow",
		Intelligence: "high",
		Features:     []string{"extended thinking", "vision", "code", "analysis"},
		Description:  "Most intelligent model for complex tasks",
	},
	{
		ID:           "claude-sonnet-4-5",
		Name:         "Claude Sonnet 4.5",
		Provider:     "Anthropic",
		ContextSize:  200000,
		InputCost:    3.0,
		OutputCost:   15.0,
		Speed:        "medium",
		Intelligence: "high",
		Features:     []string{"vision", "code", "analysis"},
		Description:  "Best balance of speed and intelligence",
	},
	{
		ID:           "claude-haiku-3-5",
		Name:         "Claude Haiku 3.5",
		Provider:     "Anthropic",
		ContextSize:  200000,
		InputCost:    0.80,
		OutputCost:   4.0,
		Speed:        "fast",
		Intelligence: "medium",
		Features:     []string{"vision", "code"},
		Description:  "Fast and cost-effective",
	},
	{
		ID:           "claude-3-5-sonnet-20241022",
		Name:         "Claude 3.5 Sonnet (Legacy)",
		Provider:     "Anthropic",
		ContextSize:  200000,
		InputCost:    3.0,
		OutputCost:   15.0,
		Speed:        "medium",
		Intelligence: "high",
		Features:     []string{"vision", "code"},
		Description:  "Previous stable version",
	},
	{
		ID:           "gpt-4o",
		Name:         "GPT-4o",
		Provider:     "OpenAI",
		ContextSize:  128000,
		InputCost:    5.0,
		OutputCost:   15.0,
		Speed:        "medium",
		Intelligence: "high",
		Features:     []string{"vision", "code", "function_calling"},
		Description:  "OpenAI flagship multimodal model",
	},
	{
		ID:           "gpt-4o-mini",
		Name:         "GPT-4o Mini",
		Provider:     "OpenAI",
		ContextSize:  128000,
		InputCost:    0.15,
		OutputCost:   0.60,
		Speed:        "fast",
		Intelligence: "medium",
		Features:     []string{"vision", "code"},
		Description:  "Fast and affordable GPT-4 variant",
	},
	{
		ID:           "gpt-4-turbo",
		Name:         "GPT-4 Turbo",
		Provider:     "OpenAI",
		ContextSize:  128000,
		InputCost:    10.0,
		OutputCost:   30.0,
		Speed:        "medium",
		Intelligence: "high",
		Features:     []string{"vision", "code"},
		Description:  "GPT-4 with improved speed",
	},
	{
		ID:           "glm-4-plus",
		Name:         "GLM-4 Plus",
		Provider:     "ZhipuAI",
		ContextSize:  128000,
		InputCost:    2.0,
		OutputCost:   2.0,
		Speed:        "fast",
		Intelligence: "high",
		Features:     []string{"code", "chinese", "extended_thinking"},
		Description:  "ZhipuAI flagship model",
	},
}

type ModelManager struct {
	currentModel string
	models       []ModelInfo
}

func NewModelManager(defaultModel string) *ModelManager {
	return &ModelManager{
		currentModel: defaultModel,
		models:       AvailableModels,
	}
}

func (mm *ModelManager) GetCurrentModel() *ModelInfo {
	for _, m := range mm.models {
		if m.ID == mm.currentModel {
			return &m
		}
	}
	return nil
}

func (mm *ModelManager) SetCurrentModel(modelID string) error {
	for _, m := range mm.models {
		if m.ID == modelID {
			mm.currentModel = modelID
			return nil
		}
	}
	return fmt.Errorf("model %s not found", modelID)
}

func (mm *ModelManager) ListModels() []ModelInfo {
	return mm.models
}

func (mm *ModelManager) GetModel(modelID string) *ModelInfo {
	for _, m := range mm.models {
		if m.ID == modelID {
			return &m
		}
	}
	return nil
}

func (mm *ModelManager) CalculateCost(inputTokens, outputTokens int) float64 {
	model := mm.GetCurrentModel()
	if model == nil {
		return 0
	}

	inputCost := float64(inputTokens) * model.InputCost / 1_000_000
	outputCost := float64(outputTokens) * model.OutputCost / 1_000_000
	return inputCost + outputCost
}

func (mm *ModelManager) FormatModelInfo(model *ModelInfo) string {
	var sb strings.Builder

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("🤖 %s\n", model.Name))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("📍 Provider: %s\n", model.Provider))
	sb.WriteString(fmt.Sprintf("📊 Context: %d tokens\n", model.ContextSize))
	sb.WriteString(fmt.Sprintf("⚡ Speed: %s\n", model.Speed))
	sb.WriteString(fmt.Sprintf("🧠 Intelligence: %s\n", model.Intelligence))
	sb.WriteString("\n💰 Pricing (per 1M tokens):\n")
	sb.WriteString(fmt.Sprintf("   Input:  $%.2f\n", model.InputCost))
	sb.WriteString(fmt.Sprintf("   Output: $%.2f\n", model.OutputCost))
	sb.WriteString("\n✨ Features:\n")
	for _, feature := range model.Features {
		sb.WriteString(fmt.Sprintf("   • %s\n", feature))
	}
	sb.WriteString(fmt.Sprintf("\n📝 %s\n", model.Description))

	return sb.String()
}

func (mm *ModelManager) FormatModelList() string {
	var sb strings.Builder

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("🤖 Available Models\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	for _, m := range mm.models {
		current := ""
		if m.ID == mm.currentModel {
			current = " ✓"
		}

		sb.WriteString(fmt.Sprintf("%s%s\n", m.ID, current))
		sb.WriteString(fmt.Sprintf("   %s\n", m.Description))
		sb.WriteString(fmt.Sprintf("   Speed: %s | Intelligence: %s | $%.2f/1M in\n",
			m.Speed, m.Intelligence, m.InputCost))
		sb.WriteString("\n")
	}

	sb.WriteString("\n💡 Usage: /model switch <id>\n")
	sb.WriteString("   Example: /model switch claude-sonnet-4-5\n")

	return sb.String()
}

func (mm *ModelManager) CompareModels(modelIDs []string) string {
	var sb strings.Builder

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("⚖️  Model Comparison\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	for _, id := range modelIDs {
		model := mm.GetModel(id)
		if model == nil {
			sb.WriteString(fmt.Sprintf("❌ %s: Not found\n\n", id))
			continue
		}

		sb.WriteString(fmt.Sprintf("🤖 %s\n", model.Name))
		sb.WriteString(fmt.Sprintf("   Context: %d | Speed: %s | Intelligence: %s\n",
			model.ContextSize, model.Speed, model.Intelligence))
		sb.WriteString(fmt.Sprintf("   Input: $%.2f/1M | Output: $%.2f/1M\n",
			model.InputCost, model.OutputCost))
		sb.WriteString("\n")
	}

	return sb.String()
}

func GetSpeedIcon(speed string) string {
	switch speed {
	case "fast":
		return "⚡⚡⚡"
	case "medium":
		return "⚡⚡"
	case "slow":
		return "⚡"
	default:
		return "⚡"
	}
}

func GetIntelligenceIcon(intelligence string) string {
	switch intelligence {
	case "high":
		return "🧠🧠🧠"
	case "medium":
		return "🧠🧠"
	case "low":
		return "🧠"
	default:
		return "🧠"
	}
}
