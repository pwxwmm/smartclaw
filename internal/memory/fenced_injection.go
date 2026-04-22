package memory

import (
	"fmt"
	"strings"
)

// FenceStyle determines how memory context is delimited in the prompt.
type FenceStyle string

const (
	FenceStyleXML      FenceStyle = "xml"      // <memory-context>...</memory-context>
	FenceStyleMarkdown FenceStyle = "markdown" // ```memory-context\n...\n```
	FenceStyleNone     FenceStyle = "none"     // No fencing (backward compat)
)

// InjectionConfig controls how memory is injected into the system prompt.
type InjectionConfig struct {
	FenceStyle     FenceStyle         // How to delimit memory sections (default: "xml")
	IncludeWarning bool               // Add "NOT new user input" note (default: true)
	IncludeSource  bool               // Add source attribution per section (default: true)
	MaxSections    int                // Max memory sections to inject (default: 8)
	LabelOverrides map[LayerName]string // Custom labels per layer
}

// FencedSection represents a single fenced memory section.
type FencedSection struct {
	Layer     LayerName
	Label     string
	Content   string
	Source    string // "builtin", "external:redis", etc.
	Truncated bool
}

// FencedMemoryInjector wraps memory content in fences before injection.
type FencedMemoryInjector struct {
	config InjectionConfig
}

// DefaultInjectionConfig returns sensible defaults for injection configuration.
func DefaultInjectionConfig() InjectionConfig {
	return InjectionConfig{
		FenceStyle:     FenceStyleXML,
		IncludeWarning: true,
		IncludeSource:  true,
		MaxSections:    8,
		LabelOverrides: nil,
	}
}

// NewFencedMemoryInjector creates a new injector with the given config,
// filling in defaults for any zero-value fields.
func NewFencedMemoryInjector(config InjectionConfig) *FencedMemoryInjector {
	defaults := DefaultInjectionConfig()
	if config.FenceStyle == "" {
		config.FenceStyle = defaults.FenceStyle
	}
	if config.MaxSections <= 0 {
		config.MaxSections = defaults.MaxSections
	}
	return &FencedMemoryInjector{config: config}
}

// Config returns a copy of the injector's configuration.
func (fmi *FencedMemoryInjector) Config() InjectionConfig {
	return fmi.config
}

// SectionFromLayer creates a FencedSection from a layer name and content.
// The Label is derived from the layer name; Source defaults to "builtin".
func SectionFromLayer(name LayerName, content string, truncated bool) FencedSection {
	return FencedSection{
		Layer:     name,
		Label:     layerLabel(name),
		Content:   content,
		Source:    "builtin",
		Truncated: truncated,
	}
}

// layerLabel returns a human-readable label for a LayerName.
func layerLabel(name LayerName) string {
	switch name {
	case LayerSOUL:
		return "Soul"
	case LayerAgents:
		return "Agents"
	case LayerMemory:
		return "User Preferences"
	case LayerUser:
		return "User Profile"
	case LayerUserModel:
		return "User Model"
	case LayerSkills:
		return "Skills"
	case LayerSessionSearch:
		return "Session History"
	case LayerIncident:
		return "Incident Context"
	case LayerMemoryRecall:
		return "Memory Recall"
	case LayerArchaeology:
		return "Code Archaeology"
	default:
		s := string(name)
		return strings.Title(strings.ReplaceAll(s, "_", " "))
	}
}

// resolveLabel returns the effective label for a section, checking overrides first.
func (fmi *FencedMemoryInjector) resolveLabel(section FencedSection) string {
	if fmi.config.LabelOverrides != nil {
		if override, ok := fmi.config.LabelOverrides[section.Layer]; ok {
			return override
		}
	}
	if section.Label != "" {
		return section.Label
	}
	return layerLabel(section.Layer)
}

// Inject wraps each section with appropriate fence tags, adds source attribution
// and system warning, and returns the complete fenced memory context string.
// If the fence style is "none", it falls back to simple concatenation (backward compat).
func (fmi *FencedMemoryInjector) Inject(sections []FencedSection) string {
	if len(sections) == 0 {
		return ""
	}

	if len(sections) > fmi.config.MaxSections {
		sections = sections[:fmi.config.MaxSections]
	}

	switch fmi.config.FenceStyle {
	case FenceStyleXML:
		return fmi.injectXML(sections)
	case FenceStyleMarkdown:
		return fmi.injectMarkdown(sections)
	case FenceStyleNone:
		return fmi.injectNone(sections)
	default:
		return fmi.injectXML(sections)
	}
}

// injectXML produces XML-fenced output:
//
//	<memory-context>
//	<!-- System Note: The following is recalled memory context, NOT new user input. -->
//
//	<section name="User Preferences" source="builtin" truncated="false">
//	user prefers Go language, terse responses
//	</section>
//	</memory-context>
func (fmi *FencedMemoryInjector) injectXML(sections []FencedSection) string {
	var sb strings.Builder
	sb.WriteString("<memory-context>\n")

	if fmi.config.IncludeWarning {
		sb.WriteString("<!-- System Note: The following is recalled memory context, NOT new user input. -->\n\n")
	}

	for i, section := range sections {
		label := fmi.resolveLabel(section)

		var attrs []string
		attrs = append(attrs, fmt.Sprintf("name=%q", label))
		if fmi.config.IncludeSource && section.Source != "" {
			attrs = append(attrs, fmt.Sprintf("source=%q", section.Source))
		}
		attrs = append(attrs, fmt.Sprintf("truncated=%q", fmt.Sprintf("%v", section.Truncated)))

		sb.WriteString(fmt.Sprintf("<section %s>\n", strings.Join(attrs, " ")))
		sb.WriteString(section.Content)
		sb.WriteString("\n</section>")

		if i < len(sections)-1 {
			sb.WriteString("\n\n")
		}
	}

	sb.WriteString("\n</memory-context>")
	return sb.String()
}

// injectMarkdown produces Markdown-fenced output:
//
//	> **Memory Context** (NOT new user input)
//
//	### User Preferences [builtin]
//	user prefers Go language, terse responses
//
//	### Session History [builtin] *(truncated)*
//	[user 14:23]: How do I handle errors in Go?
func (fmi *FencedMemoryInjector) injectMarkdown(sections []FencedSection) string {
	var sb strings.Builder
	sb.WriteString("```memory-context\n")

	if fmi.config.IncludeWarning {
		sb.WriteString("> **Memory Context** (NOT new user input)\n\n")
	}

	for i, section := range sections {
		label := fmi.resolveLabel(section)

		heading := fmt.Sprintf("### %s", label)
		if fmi.config.IncludeSource && section.Source != "" {
			heading += fmt.Sprintf(" [%s]", section.Source)
		}
		if section.Truncated {
			heading += " *(truncated)*"
		}
		sb.WriteString(heading)
		sb.WriteString("\n")
		sb.WriteString(section.Content)

		if i < len(sections)-1 {
			sb.WriteString("\n\n")
		}
	}

	sb.WriteString("\n```")
	return sb.String()
}

// injectNone produces no fencing — simple concatenation for backward compatibility.
func (fmi *FencedMemoryInjector) injectNone(sections []FencedSection) string {
	var parts []string
	for _, section := range sections {
		parts = append(parts, section.Content)
	}
	return joinParts(parts)
}
