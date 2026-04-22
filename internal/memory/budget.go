package memory

type LayerName string

const (
	LayerSOUL          LayerName = "soul"
	LayerAgents        LayerName = "agents"
	LayerMemory        LayerName = "memory"
	LayerUser          LayerName = "user"
	LayerUserModel     LayerName = "user_model"
	LayerSkills        LayerName = "skills"
	LayerSessionSearch LayerName = "session_search"
	LayerIncident      LayerName = "incident"
	LayerConvention    LayerName = "convention"
	LayerMemoryRecall  LayerName = "memory_recall"
	LayerCursorRules   LayerName = "cursor_rules"
	LayerArchaeology   LayerName = "archaeology"
)

type BudgetLayer struct {
	Name     LayerName
	Weight   float64
	MinChars int
	MaxChars int
}

type LayerContent struct {
	Name    LayerName
	Content string
}

type AllocatedLayer struct {
	Name      LayerName
	Content   string
	Chars     int
	Budget    int
	Truncated bool
}

type ContextBudget struct {
	Layers   []BudgetLayer
	MaxChars int
}

// Clone returns a deep copy of the budget.
func (cb ContextBudget) Clone() ContextBudget {
	layers := make([]BudgetLayer, len(cb.Layers))
	copy(layers, cb.Layers)
	return ContextBudget{
		Layers:   layers,
		MaxChars: cb.MaxChars,
	}
}

func DefaultContextBudget() ContextBudget {
	return ContextBudget{
		MaxChars: 3575,
		Layers: []BudgetLayer{
			{Name: LayerSOUL, Weight: 0.12, MinChars: 0, MaxChars: 1500},
			{Name: LayerAgents, Weight: 0.08, MinChars: 0, MaxChars: 1000},
			{Name: LayerCursorRules, Weight: 0.05, MinChars: 0, MaxChars: 500},
			{Name: LayerMemory, Weight: 0.20, MinChars: 0, MaxChars: 2000},
			{Name: LayerUser, Weight: 0.10, MinChars: 0, MaxChars: 1000},
			{Name: LayerUserModel, Weight: 0.05, MinChars: 0, MaxChars: 500},
			{Name: LayerSkills, Weight: 0.10, MinChars: 0, MaxChars: 800},
			{Name: LayerSessionSearch, Weight: 0.15, MinChars: 0, MaxChars: 2000},
			{Name: LayerIncident, Weight: 0.05, MinChars: 0, MaxChars: 2000},
			{Name: LayerConvention, Weight: 0.05, MinChars: 0, MaxChars: 800},
			{Name: LayerArchaeology, Weight: 0.05, MinChars: 0, MaxChars: 800},
		},
	}
}

func (cb ContextBudget) Allocate(contents []LayerContent) []AllocatedLayer {
	layerMap := make(map[LayerName]BudgetLayer)
	for _, l := range cb.Layers {
		layerMap[l.Name] = l
	}

	var result []AllocatedLayer
	totalWeight := 0.0
	var presentLayers []LayerContent

	for _, c := range contents {
		if c.Content == "" {
			continue
		}
		if _, ok := layerMap[c.Name]; ok {
			presentLayers = append(presentLayers, c)
			totalWeight += layerMap[c.Name].Weight
		} else {
			alloc := AllocatedLayer{
				Name:    c.Name,
				Content: c.Content,
				Chars:   len(c.Content),
			}
			result = append(result, alloc)
		}
	}

	if totalWeight == 0 {
		return result
	}

	remainingBudget := cb.MaxChars

	for _, alloc := range result {
		remainingBudget -= alloc.Chars
	}

	for _, c := range presentLayers {
		bl := layerMap[c.Name]
		proportionalWeight := bl.Weight / totalWeight
		budget := int(float64(remainingBudget) * proportionalWeight)

		if budget < bl.MinChars {
			budget = bl.MinChars
		}
		if budget > bl.MaxChars {
			budget = bl.MaxChars
		}

		content := c.Content
		truncated := false
		if len(content) > budget {
			if budget > 3 {
				content = content[:budget-3] + "..."
			} else {
				content = content[:budget]
			}
			truncated = true
		}

		result = append(result, AllocatedLayer{
			Name:      c.Name,
			Content:   content,
			Chars:     len(content),
			Budget:    budget,
			Truncated: truncated,
		})
	}

	return result
}
