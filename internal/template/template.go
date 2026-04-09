package template

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"
)

type Template struct {
	Name        string
	Description string
	Content     string
	Variables   []Variable
}

type Variable struct {
	Name        string
	Description string
	Default     string
	Required    bool
}

type TemplateManager struct {
	templates map[string]*Template
}

func NewTemplateManager() *TemplateManager {
	return &TemplateManager{
		templates: make(map[string]*Template),
	}
}

func (tm *TemplateManager) Load(path string) error {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	tmpl := &Template{
		Name:      name,
		Content:   string(content),
		Variables: []Variable{},
	}
	tm.templates[name] = tmpl
	return nil
}

func (tm *TemplateManager) Get(name string) (*Template, error) {
	tmpl, ok := tm.templates[name]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", name)
	}
	return tmpl, nil
}

func (tm *TemplateManager) List() []*Template {
	templates := make([]*Template, 0, len(tm.templates))
	for _, tmpl := range tm.templates {
		templates = append(templates, tmpl)
	}
	return templates
}

func (tm *TemplateManager) Execute(name string, vars map[string]string) (string, error) {
	tmpl, err := tm.Get(name)
	if err != nil {
		return "", err
	}

	t, err := template.New(name).Parse(tmpl.Content)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var result strings.Builder
	if err := t.Execute(&result, vars); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return result.String(), nil
}
