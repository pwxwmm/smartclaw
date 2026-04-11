package components

import (
	"fmt"
	"html/template"
	"strings"
)

type Component struct {
	Name    string
	Props   map[string]any
	Content string
}

type RenderedComponent struct {
	HTML string
	CSS  string
	JS   string
}

func NewComponent(name string) *Component {
	return &Component{
		Name:    name,
		Props:   make(map[string]any),
		Content: "",
	}
}

func (c *Component) SetProp(key string, value any) {
	c.Props[key] = value
}

func (c *Component) SetContent(content string) {
	c.Content = content
}

func (c *Component) Render() (RenderedComponent, error) {
	switch c.Name {
	case "button":
		return c.renderButton()
	case "input":
		return c.renderInput()
	case "card":
		return c.renderCard()
	default:
		return RenderedComponent{HTML: c.Content}, nil
	}
}

func (c *Component) renderButton() (RenderedComponent, error) {
	label := "Button"
	if l, ok := c.Props["label"].(string); ok {
		label = l
	}
	disabled := ""
	if d, ok := c.Props["disabled"].(bool); ok && d {
		disabled = "disabled"
	}
	html := fmt.Sprintf(`<button class="btn" %s>%s</button>`, disabled, label)
	return RenderedComponent{HTML: html}, nil
}

func (c *Component) renderInput() (RenderedComponent, error) {
	placeholder := ""
	if p, ok := c.Props["placeholder"].(string); ok {
		placeholder = p
	}
	inputType := "text"
	if t, ok := c.Props["type"].(string); ok {
		inputType = t
	}
	html := fmt.Sprintf(`<input type="%s" placeholder="%s" class="input">`, inputType, placeholder)
	return RenderedComponent{HTML: html}, nil
}

func (c *Component) renderCard() (RenderedComponent, error) {
	title := ""
	if t, ok := c.Props["title"].(string); ok {
		title = t
	}
	html := fmt.Sprintf(`<div class="card"><h3>%s</h3><div class="content">%s</div></div>`, title, c.Content)
	return RenderedComponent{HTML: html}, nil
}

func RenderTemplate(tmpl string, data any) (string, error) {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	err = t.Execute(&sb, data)
	return sb.String(), err
}
