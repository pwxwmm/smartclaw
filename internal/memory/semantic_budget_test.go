package memory

import (
	"math"
	"testing"
)

func TestClassifyQuery_Debug(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	tests := []struct {
		query string
		want  QueryType
	}{
		{"fix the error in my code", QueryTypeDebug},
		{"I'm getting a crash when I run it", QueryTypeDebug},
		{"there's a bug in the handler", QueryTypeDebug},
		{"panic: nil pointer dereference", QueryTypeDebug},
		{"debug this traceback", QueryTypeDebug},
		{"exception thrown on startup", QueryTypeDebug},
		{"it's not working", QueryTypeDebug},
		{"the test is failing", QueryTypeDebug},
	}
	for _, tt := range tests {
		got := sba.ClassifyQuery(tt.query)
		if got != tt.want {
			t.Errorf("ClassifyQuery(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestClassifyQuery_Code(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	tests := []struct {
		query string
		want  QueryType
	}{
		{"implement a new handler", QueryTypeCode},
		{"refactor the class hierarchy", QueryTypeCode},
		{"add a method to the struct", QueryTypeCode},
		{"write code for the endpoint", QueryTypeCode},
		{"add feature to the API", QueryTypeCode},
		{"create a new interface", QueryTypeCode},
	}
	for _, tt := range tests {
		got := sba.ClassifyQuery(tt.query)
		if got != tt.want {
			t.Errorf("ClassifyQuery(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestClassifyQuery_Planning(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	tests := []struct {
		query string
		want  QueryType
	}{
		{"plan the migration strategy", QueryTypePlanning},
		{"how should we architect this system", QueryTypePlanning},
		{"design the new module organization", QueryTypePlanning},
		{"organize the codebase", QueryTypePlanning},
		{"roadmap for next quarter", QueryTypePlanning},
	}
	for _, tt := range tests {
		got := sba.ClassifyQuery(tt.query)
		if got != tt.want {
			t.Errorf("ClassifyQuery(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestClassifyQuery_Personal(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	tests := []struct {
		query string
		want  QueryType
	}{
		{"what do I prefer for my workflow", QueryTypePersonal},
		{"my preference is tabs not spaces", QueryTypePersonal},
		{"I prefer dark mode", QueryTypePersonal},
		{"remember that my workflow uses make", QueryTypePersonal},
		{"about me: I like Go", QueryTypePersonal},
	}
	for _, tt := range tests {
		got := sba.ClassifyQuery(tt.query)
		if got != tt.want {
			t.Errorf("ClassifyQuery(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestClassifyQuery_Search(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	tests := []struct {
		query string
		want  QueryType
	}{
		{"how does the memory system work", QueryTypeSearch},
		{"what is FTS5", QueryTypeSearch},
		{"explain how the system works", QueryTypeSearch},
		{"describe the rendering pipeline", QueryTypeSearch},
		{"why does it hang on startup", QueryTypeSearch},
	}
	for _, tt := range tests {
		got := sba.ClassifyQuery(tt.query)
		if got != tt.want {
			t.Errorf("ClassifyQuery(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestClassifyQuery_Creative(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	tests := []struct {
		query string
		want  QueryType
	}{
		{"create a new story", QueryTypeCreative},
		{"generate ideas for the project", QueryTypeCreative},
		{"brainstorm solutions", QueryTypeCreative},
		{"draft a proposal", QueryTypeCreative},
		{"compose a welcome message", QueryTypeCreative},
	}
	for _, tt := range tests {
		got := sba.ClassifyQuery(tt.query)
		if got != tt.want {
			t.Errorf("ClassifyQuery(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestClassifyQuery_Unknown(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	tests := []struct {
		query string
		want  QueryType
	}{
		{"hello", QueryTypeUnknown},
		{"thanks", QueryTypeUnknown},
		{"", QueryTypeUnknown},
		{"ok sure", QueryTypeUnknown},
	}
	for _, tt := range tests {
		got := sba.ClassifyQuery(tt.query)
		if got != tt.want {
			t.Errorf("ClassifyQuery(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestAdjustBudget_PreservesTotalWeight(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	base := DefaultContextBudget()

	for qt := QueryTypeUnknown; qt <= QueryTypeCreative; qt++ {
		adjusted := sba.AdjustBudget(qt, base)
		total := 0.0
		for _, l := range adjusted.Layers {
			total += l.Weight
		}
		if math.Abs(total-1.0) > 1e-4 {
			t.Errorf("QueryType %d: total weight = %v, want 1.0", qt, total)
		}
	}
}

func TestAdjustBudget_MinWeightFloor(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	base := DefaultContextBudget()

	for qt := QueryTypeUnknown; qt <= QueryTypeCreative; qt++ {
		adjusted := sba.AdjustBudget(qt, base)
		for _, l := range adjusted.Layers {
			if l.Weight < 0.02 {
				t.Errorf("QueryType %d: layer %s weight %v below minimum 0.02", qt, l.Name, l.Weight)
			}
		}
	}
}

func TestAdjustBudget_DoesNotMutateBase(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	base := DefaultContextBudget()
	originalWeights := make(map[LayerName]float64)
	for _, l := range base.Layers {
		originalWeights[l.Name] = l.Weight
	}

	sba.AdjustBudget(QueryTypeCode, base)
	sba.AdjustBudget(QueryTypeDebug, base)
	sba.AdjustBudget(QueryTypePersonal, base)

	for _, l := range base.Layers {
		if l.Weight != originalWeights[l.Name] {
			t.Errorf("base budget mutated: layer %s weight changed from %v to %v", l.Name, originalWeights[l.Name], l.Weight)
		}
	}
}

func TestAdjustBudget_UnknownReturnsClone(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	base := DefaultContextBudget()

	adjusted := sba.AdjustBudget(QueryTypeUnknown, base)

	if len(adjusted.Layers) != len(base.Layers) {
		t.Fatalf("expected %d layers, got %d", len(base.Layers), len(adjusted.Layers))
	}

	for i := range base.Layers {
		if adjusted.Layers[i].Weight != base.Layers[i].Weight {
			t.Errorf("unknown query type should return unchanged budget: layer %s got %v, want %v",
				base.Layers[i].Name, adjusted.Layers[i].Weight, base.Layers[i].Weight)
		}
	}

	adjusted.Layers[0].Weight = 999
	if base.Layers[0].Weight == 999 {
		t.Error("returned budget should be a clone, not a reference to base")
	}
}

func TestAdjustBudget_CodeTypeBoostsSkills(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	base := DefaultContextBudget()

	baseSkills := layerWeight(base, LayerSkills)
	adjusted := sba.AdjustBudget(QueryTypeCode, base)
	adjSkills := layerWeight(adjusted, LayerSkills)

	if adjSkills <= baseSkills {
		t.Errorf("code query should boost skills: got %v, base %v", adjSkills, baseSkills)
	}
}

func TestAdjustBudget_DebugTypeBoostsSessionSearch(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	base := DefaultContextBudget()

	baseSearch := layerWeight(base, LayerSessionSearch)
	adjusted := sba.AdjustBudget(QueryTypeDebug, base)
	adjSearch := layerWeight(adjusted, LayerSessionSearch)

	if adjSearch <= baseSearch {
		t.Errorf("debug query should boost session_search: got %v, base %v", adjSearch, baseSearch)
	}
}

func TestAdjustBudget_PersonalTypeBoostsUserModel(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	base := DefaultContextBudget()

	baseUM := layerWeight(base, LayerUserModel)
	adjusted := sba.AdjustBudget(QueryTypePersonal, base)
	adjUM := layerWeight(adjusted, LayerUserModel)

	if adjUM <= baseUM {
		t.Errorf("personal query should boost user_model: got %v, base %v", adjUM, baseUM)
	}
}

func TestAdjustBudget_PlanningTypeBoostsConvention(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	base := DefaultContextBudget()

	baseConv := layerWeight(base, LayerConvention)
	adjusted := sba.AdjustBudget(QueryTypePlanning, base)
	adjConv := layerWeight(adjusted, LayerConvention)

	if adjConv <= baseConv {
		t.Errorf("planning query should boost convention: got %v, base %v", adjConv, baseConv)
	}
}

func TestAdjustBudget_MaxCharsUnchanged(t *testing.T) {
	sba := NewSemanticBudgetAdjuster()
	base := DefaultContextBudget()

	for qt := QueryTypeUnknown; qt <= QueryTypeCreative; qt++ {
		adjusted := sba.AdjustBudget(qt, base)
		if adjusted.MaxChars != base.MaxChars {
			t.Errorf("QueryType %d: MaxChars changed from %d to %d", qt, base.MaxChars, adjusted.MaxChars)
		}
	}
}

func layerWeight(b ContextBudget, name LayerName) float64 {
	for _, l := range b.Layers {
		if l.Name == name {
			return l.Weight
		}
	}
	return 0
}
