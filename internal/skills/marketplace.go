package skills

import (
	"fmt"
	"sort"
)

var defaultCategories = []string{
	"code-review",
	"deployment",
	"debugging",
	"security",
	"sre",
	"testing",
	"automation",
	"code-style",
}

type Marketplace struct {
	registry *Registry
}

func NewMarketplace(registry *Registry) *Marketplace {
	return &Marketplace{registry: registry}
}

func (m *Marketplace) SearchMarketplace(query string, category string, page, pageSize int) (*SkillSearchResult, error) {
	return m.registry.Search(query, category, "", page, pageSize)
}

func (m *Marketplace) InstallSkill(name string) (*SkillMeta, error) {
	meta, err := m.registry.Get(name)
	if err != nil {
		return nil, fmt.Errorf("skill %q not found in marketplace: %w", name, err)
	}

	content := meta.Content
	if content == "" {
		sm := GetSkillManager()
		if sm != nil {
			loaded, loadErr := sm.GetContent(name)
			if loadErr == nil {
				content = loaded
			}
		}
	}

	if content == "" {
		content = renderDefaultContent(meta)
	}

	installMeta := *meta
	if err := m.registry.Install(name, content, installMeta); err != nil {
		return nil, fmt.Errorf("failed to install skill %q: %w", name, err)
	}

	installed, _ := m.registry.Get(name)
	return installed, nil
}

func (m *Marketplace) PublishSkill(name string) error {
	meta, err := m.registry.Get(name)
	if err != nil {
		return fmt.Errorf("skill %q not found: %w", name, err)
	}

	if meta.Source == "marketplace" {
		return fmt.Errorf("skill %q is already published", name)
	}

	return m.registry.Publish(*meta)
}

func (m *Marketplace) GetCategories() []string {
	cats := make([]string, len(defaultCategories))
	copy(cats, defaultCategories)
	return cats
}

func (m *Marketplace) GetFeatured() ([]SkillMeta, error) {
	result, err := m.registry.Search("", "", "", 1, 10)
	if err != nil {
		return nil, err
	}

	featured := result.Skills
	sort.Slice(featured, func(i, j int) bool {
		if featured[i].Rating != featured[j].Rating {
			return featured[i].Rating > featured[j].Rating
		}
		return featured[i].Downloads > featured[j].Downloads
	})

	if len(featured) > 6 {
		featured = featured[:6]
	}

	return featured, nil
}

func (m *Marketplace) GetRegistry() *Registry {
	return m.registry
}

func renderDefaultContent(meta *SkillMeta) string {
	var sb sbuilder
	sb.writeln("# " + meta.Name)
	sb.writeln("")
	if meta.Description != "" {
		sb.writeln(meta.Description)
		sb.writeln("")
	}
	if len(meta.Triggers) > 0 {
		sb.writeln("## Triggers")
		for _, t := range meta.Triggers {
			sb.writeln("- " + t)
		}
		sb.writeln("")
	}
	if len(meta.Tools) > 0 {
		sb.writeln("## Tools")
		for _, t := range meta.Tools {
			sb.writeln("- " + t)
		}
		sb.writeln("")
	}
	if len(meta.Tags) > 0 {
		sb.writeln("## Tags")
		for i, t := range meta.Tags {
			if i > 0 {
				sb.raw(", ")
			}
			sb.raw(t)
		}
		sb.writeln("")
	}
	return sb.string()
}

type sbuilder struct {
	buf []byte
}

func (s *sbuilder) writeln(str string) {
	s.buf = append(s.buf, str...)
	s.buf = append(s.buf, '\n')
}

func (s *sbuilder) raw(str string) {
	s.buf = append(s.buf, str...)
}

func (s *sbuilder) string() string {
	return string(s.buf)
}
