package memory

import (
	"math"
	"strings"
)

type QueryType int

const (
	QueryTypeUnknown  QueryType = iota
	QueryTypeCode
	QueryTypeDebug
	QueryTypePersonal
	QueryTypePlanning
	QueryTypeSearch
	QueryTypeCreative
)

type SemanticBudgetAdjuster struct{}

func NewSemanticBudgetAdjuster() *SemanticBudgetAdjuster {
	return &SemanticBudgetAdjuster{}
}

type BudgetAdjustment struct {
	Name  LayerName
	Delta float64
}

var queryTypeAdjustments = map[QueryType][]BudgetAdjustment{
	QueryTypeCode: {
		{LayerSkills, 0.05},
		{LayerArchaeology, 0.03},
		{LayerUserModel, -0.03},
		{LayerConvention, -0.03},
		{LayerSessionSearch, -0.02},
	},
	QueryTypeDebug: {
		{LayerSessionSearch, 0.05},
		{LayerIncident, 0.05},
		{LayerSkills, -0.04},
		{LayerConvention, -0.03},
		{LayerUserModel, -0.03},
	},
	QueryTypePersonal: {
		{LayerUserModel, 0.08},
		{LayerUser, 0.04},
		{LayerSkills, -0.04},
		{LayerSessionSearch, -0.04},
		{LayerArchaeology, -0.04},
	},
	QueryTypePlanning: {
		{LayerConvention, 0.05},
		{LayerMemory, 0.05},
		{LayerSkills, -0.04},
		{LayerIncident, -0.03},
		{LayerArchaeology, -0.03},
	},
	QueryTypeSearch: {
		{LayerSessionSearch, 0.05},
		{LayerMemory, 0.03},
		{LayerUserModel, -0.04},
		{LayerConvention, -0.02},
		{LayerIncident, -0.02},
	},
	QueryTypeCreative: {
		{LayerSkills, 0.03},
		{LayerMemory, 0.03},
		{LayerIncident, -0.03},
		{LayerArchaeology, -0.03},
	},
}

func (sba *SemanticBudgetAdjuster) ClassifyQuery(query string) QueryType {
	q := strings.ToLower(query)

	debugPatterns := []string{"error", "bug", "fix", "crash", "broken", "not working", "panic", "traceback", "stack trace", "debug", "fail", "exception"}
	if matchesAny(q, debugPatterns) {
		return QueryTypeDebug
	}

	codePatterns := []string{"implement", "refactor", "function", "method", "class", "variable", "type", "interface", "struct", "package", "import", "api", "endpoint", "handler", "test", "write code", "add feature"}
	if matchesAny(q, codePatterns) {
		return QueryTypeCode
	}

	planPatterns := []string{"plan", "architect", "design", "how should", "structure", "organize", "strategy", "approach", "roadmap", "migrate"}
	if matchesAny(q, planPatterns) {
		return QueryTypePlanning
	}

	personalPatterns := []string{"my preference", "i like", "i prefer", "my style", "remember", "about me", "what do i", "my workflow"}
	if matchesAny(q, personalPatterns) {
		return QueryTypePersonal
	}

	searchPatterns := []string{"how does", "what is", "explain", "describe", "tell me about", "documentation", "how do", "why does"}
	if matchesAny(q, searchPatterns) {
		return QueryTypeSearch
	}

	creativePatterns := []string{"create", "generate", "write", "compose", "brainstorm", "idea", "creative", "story", "draft"}
	if matchesAny(q, creativePatterns) {
		return QueryTypeCreative
	}

	return QueryTypeUnknown
}

func (sba *SemanticBudgetAdjuster) AdjustBudget(queryType QueryType, base ContextBudget) ContextBudget {
	if queryType == QueryTypeUnknown {
		return base.Clone()
	}

	adjusted := base.Clone()

	adjustments, ok := queryTypeAdjustments[queryType]
	if !ok {
		return adjusted
	}

	adjMap := make(map[LayerName]float64, len(adjustments))
	for _, a := range adjustments {
		adjMap[a.Name] = a.Delta
	}

	for i := range adjusted.Layers {
		if delta, exists := adjMap[adjusted.Layers[i].Name]; exists {
			adjusted.Layers[i].Weight += delta
		}
	}

	const minWeight = 0.02
	for {
		for i := range adjusted.Layers {
			if adjusted.Layers[i].Weight < minWeight {
				adjusted.Layers[i].Weight = minWeight
			}
		}

		total := 0.0
		for _, l := range adjusted.Layers {
			total += l.Weight
		}
		if total > 0 {
			for i := range adjusted.Layers {
				adjusted.Layers[i].Weight /= total
			}
		}

		allAboveMin := true
		for _, l := range adjusted.Layers {
			if l.Weight < minWeight {
				allAboveMin = false
				break
			}
		}
		if allAboveMin {
			break
		}
	}

	// Round to avoid floating-point drift
	for i := range adjusted.Layers {
		adjusted.Layers[i].Weight = math.Round(adjusted.Layers[i].Weight*1e6) / 1e6
	}

	return adjusted
}

func matchesAny(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
