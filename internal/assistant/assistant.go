package assistant

import (
	"context"
	"fmt"
	"strings"
)

type Assistant struct {
	Name      string
	Model     string
	Persona   string
	Context   []string
	Variables map[string]string
}

func NewAssistant(name, persona string) *Assistant {
	return &Assistant{
		Name:      name,
		Persona:   persona,
		Variables: make(map[string]string),
		Context:   make([]string, 0),
	}
}

func (a *Assistant) SetModel(model string) {
	a.Model = model
}

func (a *Assistant) SetVariable(key, value string) {
	a.Variables[key] = value
}

func (a *Assistant) GetVariable(key string) string {
	return a.Variables[key]
}

func (a *Assistant) AddContext(ctx string) {
	a.Context = append(a.Context, ctx)
}

func (a *Assistant) ClearContext() {
	a.Context = make([]string, 0)
}

func (a *Assistant) GeneratePrompt(input string) string {
	var sb strings.Builder
	sb.WriteString(a.Persona)
	sb.WriteString("\n\n")
	for _, ctx := range a.Context {
		sb.WriteString(ctx)
		sb.WriteString("\n")
	}
	sb.WriteString("User: ")
	sb.WriteString(input)
	return sb.String()
}

func (a *Assistant) Process(ctx context.Context, input string) (string, error) {
	prompt := a.GeneratePrompt(input)
	return fmt.Sprintf("[%s] Processed: %s", a.Name, prompt), nil
}

type AssistantManager struct {
	assistants map[string]*Assistant
	active     *Assistant
}

func NewAssistantManager() *AssistantManager {
	return &AssistantManager{
		assistants: make(map[string]*Assistant),
	}
}

func (m *AssistantManager) Register(assistant *Assistant) {
	m.assistants[assistant.Name] = assistant
}

func (m *AssistantManager) Get(name string) *Assistant {
	return m.assistants[name]
}

func (m *AssistantManager) SetActive(name string) error {
	a, ok := m.assistants[name]
	if !ok {
		return fmt.Errorf("assistant not found: %s", name)
	}
	m.active = a
	return nil
}

func (m *AssistantManager) Active() *Assistant {
	return m.active
}

func (m *AssistantManager) List() []string {
	names := make([]string, 0, len(m.assistants))
	for name := range m.assistants {
		names = append(names, name)
	}
	return names
}
