package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/store"
)

func TestDefaultInjectionConfig(t *testing.T) {
	cfg := DefaultInjectionConfig()
	if cfg.FenceStyle != FenceStyleXML {
		t.Fatalf("expected FenceStyleXML, got %q", cfg.FenceStyle)
	}
	if !cfg.IncludeWarning {
		t.Fatal("expected IncludeWarning true")
	}
	if !cfg.IncludeSource {
		t.Fatal("expected IncludeSource true")
	}
	if cfg.MaxSections != 8 {
		t.Fatalf("expected MaxSections 8, got %d", cfg.MaxSections)
	}
}

func TestNewFencedMemoryInjector_Defaults(t *testing.T) {
	fmi := NewFencedMemoryInjector(DefaultInjectionConfig())
	cfg := fmi.Config()
	if cfg.FenceStyle != FenceStyleXML {
		t.Fatalf("expected default FenceStyleXML, got %q", cfg.FenceStyle)
	}
	if cfg.MaxSections != 8 {
		t.Fatalf("expected default MaxSections 8, got %d", cfg.MaxSections)
	}
	if !cfg.IncludeWarning {
		t.Fatal("expected default IncludeWarning true")
	}
	if !cfg.IncludeSource {
		t.Fatal("expected default IncludeSource true")
	}
}

func TestNewFencedMemoryInjector_EmptyConfig(t *testing.T) {
	fmi := NewFencedMemoryInjector(InjectionConfig{})
	cfg := fmi.Config()
	if cfg.FenceStyle != FenceStyleXML {
		t.Fatalf("expected FenceStyleXML from zero-value fill, got %q", cfg.FenceStyle)
	}
	if cfg.MaxSections != 8 {
		t.Fatalf("expected MaxSections 8 from zero-value fill, got %d", cfg.MaxSections)
	}
}

func TestNewFencedMemoryInjector_Custom(t *testing.T) {
	fmi := NewFencedMemoryInjector(InjectionConfig{
		FenceStyle:     FenceStyleMarkdown,
		MaxSections:    4,
		IncludeWarning: false,
		IncludeSource:  false,
	})
	cfg := fmi.Config()
	if cfg.FenceStyle != FenceStyleMarkdown {
		t.Fatalf("expected FenceStyleMarkdown, got %q", cfg.FenceStyle)
	}
	if cfg.MaxSections != 4 {
		t.Fatalf("expected MaxSections 4, got %d", cfg.MaxSections)
	}
	if cfg.IncludeWarning {
		t.Fatal("expected IncludeWarning false")
	}
	if cfg.IncludeSource {
		t.Fatal("expected IncludeSource false")
	}
}

func TestInject_XML(t *testing.T) {
	fmi := NewFencedMemoryInjector(InjectionConfig{
		FenceStyle:     FenceStyleXML,
		IncludeWarning: true,
		IncludeSource:  true,
		MaxSections:    8,
	})

	sections := []FencedSection{
		{Layer: LayerMemory, Label: "User Preferences", Content: "user prefers Go language, terse responses", Source: "builtin", Truncated: false},
		{Layer: LayerSessionSearch, Label: "Session History", Content: "[user 14:23]: How do I handle errors in Go?\n[assistant 14:23]: Use fmt.Errorf with %w wrapping...", Source: "builtin", Truncated: true},
	}

	result := fmi.Inject(sections)

	if !strings.HasPrefix(result, "<memory-context>") {
		t.Fatal("expected XML to start with <memory-context>")
	}
	if !strings.HasSuffix(result, "</memory-context>") {
		t.Fatal("expected XML to end with </memory-context>")
	}
	if !strings.Contains(result, "NOT new user input") {
		t.Fatal("expected warning note in XML output")
	}
	if !strings.Contains(result, `name="User Preferences"`) {
		t.Fatal("expected section with name=User Preferences")
	}
	if !strings.Contains(result, `source="builtin"`) {
		t.Fatal("expected source attribution")
	}
	if !strings.Contains(result, `truncated="false"`) {
		t.Fatal("expected truncated=false attribute")
	}
	if !strings.Contains(result, `truncated="true"`) {
		t.Fatal("expected truncated=true attribute")
	}
	if !strings.Contains(result, "user prefers Go language, terse responses") {
		t.Fatal("expected content in output")
	}
	if !strings.Contains(result, "Use fmt.Errorf with %w wrapping") {
		t.Fatal("expected session content in output")
	}
}

func TestInject_XML_NoWarning(t *testing.T) {
	fmi := NewFencedMemoryInjector(InjectionConfig{
		FenceStyle:     FenceStyleXML,
		IncludeWarning: false,
		IncludeSource:  true,
		MaxSections:    8,
	})

	sections := []FencedSection{
		{Layer: LayerMemory, Label: "Prefs", Content: "test", Source: "builtin", Truncated: false},
	}

	result := fmi.Inject(sections)
	if strings.Contains(result, "NOT new user input") {
		t.Fatal("expected no warning when IncludeWarning=false")
	}
}

func TestInject_XML_NoSource(t *testing.T) {
	fmi := NewFencedMemoryInjector(InjectionConfig{
		FenceStyle:     FenceStyleXML,
		IncludeWarning: true,
		IncludeSource:  false,
		MaxSections:    8,
	})

	sections := []FencedSection{
		{Layer: LayerMemory, Label: "Prefs", Content: "test", Source: "builtin", Truncated: false},
	}

	result := fmi.Inject(sections)
	if strings.Contains(result, "source=") {
		t.Fatal("expected no source attribution when IncludeSource=false")
	}
}

func TestInject_Markdown(t *testing.T) {
	fmi := NewFencedMemoryInjector(InjectionConfig{
		FenceStyle:     FenceStyleMarkdown,
		IncludeWarning: true,
		IncludeSource:  true,
		MaxSections:    8,
	})

	sections := []FencedSection{
		{Layer: LayerMemory, Label: "User Preferences", Content: "user prefers Go language, terse responses", Source: "builtin", Truncated: false},
		{Layer: LayerSessionSearch, Label: "Session History", Content: "[user 14:23]: How do I handle errors?", Source: "external:redis", Truncated: true},
	}

	result := fmi.Inject(sections)

	if !strings.HasPrefix(result, "```memory-context\n") {
		t.Fatal("expected markdown to start with ```memory-context")
	}
	if !strings.HasSuffix(result, "```") {
		t.Fatal("expected markdown to end with ```")
	}
	if !strings.Contains(result, "NOT new user input") {
		t.Fatal("expected warning note in markdown output")
	}
	if !strings.Contains(result, "### User Preferences [builtin]") {
		t.Fatal("expected markdown heading with source")
	}
	if !strings.Contains(result, "### Session History [external:redis] *(truncated)*") {
		t.Fatal("expected markdown heading with source and truncated marker")
	}
}

func TestInject_Markdown_NoWarning(t *testing.T) {
	fmi := NewFencedMemoryInjector(InjectionConfig{
		FenceStyle:     FenceStyleMarkdown,
		IncludeWarning: false,
		IncludeSource:  true,
		MaxSections:    8,
	})

	sections := []FencedSection{
		{Layer: LayerMemory, Label: "Prefs", Content: "test", Source: "builtin", Truncated: false},
	}

	result := fmi.Inject(sections)
	if strings.Contains(result, "NOT new user input") {
		t.Fatal("expected no warning when IncludeWarning=false")
	}
}

func TestInject_Markdown_NoSource(t *testing.T) {
	fmi := NewFencedMemoryInjector(InjectionConfig{
		FenceStyle:     FenceStyleMarkdown,
		IncludeWarning: true,
		IncludeSource:  false,
		MaxSections:    8,
	})

	sections := []FencedSection{
		{Layer: LayerMemory, Label: "Prefs", Content: "test", Source: "builtin", Truncated: false},
	}

	result := fmi.Inject(sections)
	if strings.Contains(result, "[builtin]") {
		t.Fatal("expected no source bracket when IncludeSource=false")
	}
	if !strings.Contains(result, "### Prefs") {
		t.Fatal("expected heading without source")
	}
}

func TestInject_None(t *testing.T) {
	fmi := NewFencedMemoryInjector(InjectionConfig{
		FenceStyle:  FenceStyleNone,
		MaxSections: 8,
	})

	sections := []FencedSection{
		{Layer: LayerMemory, Label: "Prefs", Content: "alpha", Source: "builtin", Truncated: false},
		{Layer: LayerUser, Label: "User", Content: "beta", Source: "builtin", Truncated: false},
	}

	result := fmi.Inject(sections)
	if strings.Contains(result, "<memory-context>") {
		t.Fatal("expected no XML tags with FenceStyleNone")
	}
	if strings.Contains(result, "```") {
		t.Fatal("expected no markdown fences with FenceStyleNone")
	}
	if !strings.Contains(result, "alpha") || !strings.Contains(result, "beta") {
		t.Fatal("expected content to be present")
	}
	expected := "alpha\n\nbeta"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestInject_EmptySections(t *testing.T) {
	fmi := NewFencedMemoryInjector(DefaultInjectionConfig())
	result := fmi.Inject(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil sections, got %q", result)
	}

	result = fmi.Inject([]FencedSection{})
	if result != "" {
		t.Fatalf("expected empty string for empty sections, got %q", result)
	}
}

func TestInject_MaxSections(t *testing.T) {
	fmi := NewFencedMemoryInjector(InjectionConfig{
		FenceStyle:     FenceStyleXML,
		IncludeWarning: false,
		IncludeSource:  false,
		MaxSections:    2,
	})

	sections := []FencedSection{
		{Layer: LayerSOUL, Label: "Soul", Content: "first", Source: "builtin", Truncated: false},
		{Layer: LayerMemory, Label: "Memory", Content: "second", Source: "builtin", Truncated: false},
		{Layer: LayerUser, Label: "User", Content: "third", Source: "builtin", Truncated: false},
		{Layer: LayerSkills, Label: "Skills", Content: "fourth", Source: "builtin", Truncated: false},
	}

	result := fmi.Inject(sections)

	if strings.Contains(result, "third") {
		t.Fatal("expected third section to be cut off by MaxSections=2")
	}
	if strings.Contains(result, "fourth") {
		t.Fatal("expected fourth section to be cut off by MaxSections=2")
	}
	if !strings.Contains(result, "first") || !strings.Contains(result, "second") {
		t.Fatal("expected first two sections to be present")
	}
}

func TestInject_LabelOverrides(t *testing.T) {
	fmi := NewFencedMemoryInjector(InjectionConfig{
		FenceStyle:     FenceStyleXML,
		IncludeWarning: false,
		IncludeSource:  false,
		MaxSections:    8,
		LabelOverrides: map[LayerName]string{
			LayerMemory: "Custom Memory Label",
		},
	})

	sections := []FencedSection{
		{Layer: LayerMemory, Label: "User Preferences", Content: "test content", Source: "builtin", Truncated: false},
	}

	result := fmi.Inject(sections)
	if !strings.Contains(result, `name="Custom Memory Label"`) {
		t.Fatal("expected label override to take effect")
	}
	if strings.Contains(result, `name="User Preferences"`) {
		t.Fatal("expected original label to be overridden")
	}
}

func TestSectionFromLayer(t *testing.T) {
	section := SectionFromLayer(LayerMemory, "some content", true)
	if section.Layer != LayerMemory {
		t.Fatalf("expected LayerMemory, got %q", section.Layer)
	}
	if section.Label != "User Preferences" {
		t.Fatalf("expected label 'User Preferences', got %q", section.Label)
	}
	if section.Content != "some content" {
		t.Fatalf("expected content 'some content', got %q", section.Content)
	}
	if section.Source != "builtin" {
		t.Fatalf("expected source 'builtin', got %q", section.Source)
	}
	if !section.Truncated {
		t.Fatal("expected truncated true")
	}
}

func TestSectionFromLayer_AllLayers(t *testing.T) {
	tests := []struct {
		layer       LayerName
		expectLabel string
	}{
		{LayerSOUL, "Soul"},
		{LayerAgents, "Agents"},
		{LayerMemory, "User Preferences"},
		{LayerUser, "User Profile"},
		{LayerUserModel, "User Model"},
		{LayerSkills, "Skills"},
		{LayerSessionSearch, "Session History"},
		{LayerIncident, "Incident Context"},
		{LayerMemoryRecall, "Memory Recall"},
	}

	for _, tt := range tests {
		t.Run(string(tt.layer), func(t *testing.T) {
			section := SectionFromLayer(tt.layer, "content", false)
			if section.Label != tt.expectLabel {
				t.Fatalf("expected label %q, got %q", tt.expectLabel, section.Label)
			}
		})
	}
}

func TestBuildSystemContext_WithInjector(t *testing.T) {
	mm := newTestMemoryManager(t)
	pm := mm.GetPromptMemory()
	pm.UpdateMemory("fenced memory test")

	ctx := context.Background()
	result := mm.BuildSystemContext(ctx, "")
	if result == "" {
		t.Fatal("expected non-empty context with injector")
	}
	if !strings.Contains(result, "<memory-context>") {
		t.Fatal("expected fenced output with default injector")
	}
	if !strings.Contains(result, "NOT new user input") {
		t.Fatal("expected warning in default injector output")
	}
}

func TestBuildSystemContext_WithInjectorDisabled(t *testing.T) {
	mm := newTestMemoryManager(t)
	mm.SetInjectionConfig(InjectionConfig{FenceStyle: FenceStyleNone})
	pm := mm.GetPromptMemory()
	pm.UpdateMemory("unfenced memory test")

	ctx := context.Background()
	result := mm.BuildSystemContext(ctx, "")
	if result == "" {
		t.Fatal("expected non-empty context without injector fencing")
	}
	if strings.Contains(result, "<memory-context>") {
		t.Fatal("expected no fencing when FenceStyleNone")
	}
}

func TestBuildSystemContext_MarkdownInjector(t *testing.T) {
	mm := newTestMemoryManager(t)
	mm.SetInjectionConfig(InjectionConfig{
		FenceStyle:     FenceStyleMarkdown,
		IncludeWarning: true,
		IncludeSource:  true,
		MaxSections:    8,
	})
	pm := mm.GetPromptMemory()
	pm.UpdateMemory("markdown memory test")

	ctx := context.Background()
	result := mm.BuildSystemContext(ctx, "")
	if result == "" {
		t.Fatal("expected non-empty context with markdown injector")
	}
	if !strings.Contains(result, "```memory-context") {
		t.Fatal("expected markdown fence in output")
	}
}

func TestBuildSystemContext_BackwardCompat_NoInjector(t *testing.T) {
	dir := t.TempDir()
	pm, err := layers.NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir error: %v", err)
	}

	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("NewStoreWithDir error: %v", err)
	}
	defer s.Close()

	sm := layers.NewSkillProceduralMemory("", nil)

	mm := NewMemoryManagerWithComponents(pm, s, sm)
	defer mm.Close()

	pm.UpdateMemory("backward compat test")
	ctx := context.Background()
	result := mm.BuildSystemContext(ctx, "")
	if result == "" {
		t.Fatal("expected non-empty context without injector")
	}
	if strings.Contains(result, "<memory-context>") {
		t.Fatal("expected no fencing when built with components (no injector)")
	}
}

func TestGetInjector(t *testing.T) {
	mm := newTestMemoryManager(t)
	injector := mm.GetInjector()
	if injector == nil {
		t.Fatal("expected non-nil injector for NewMemoryManagerWithDir")
	}
	cfg := injector.Config()
	if cfg.FenceStyle != FenceStyleXML {
		t.Fatalf("expected default FenceStyleXML, got %q", cfg.FenceStyle)
	}
}

func TestSetInjectionConfig(t *testing.T) {
	mm := newTestMemoryManager(t)
	mm.SetInjectionConfig(InjectionConfig{
		FenceStyle:     FenceStyleMarkdown,
		IncludeWarning: false,
		IncludeSource:  false,
		MaxSections:    3,
	})

	injector := mm.GetInjector()
	if injector == nil {
		t.Fatal("expected non-nil injector after SetInjectionConfig")
	}
	cfg := injector.Config()
	if cfg.FenceStyle != FenceStyleMarkdown {
		t.Fatalf("expected FenceStyleMarkdown, got %q", cfg.FenceStyle)
	}
	if cfg.MaxSections != 3 {
		t.Fatalf("expected MaxSections 3, got %d", cfg.MaxSections)
	}
}

