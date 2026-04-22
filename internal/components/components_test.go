package components

import (
	"strings"
	"testing"
)

func TestNewComponent(t *testing.T) {
	c := NewComponent("button")
	if c.Name != "button" {
		t.Errorf("Name = %q, want %q", c.Name, "button")
	}
	if c.Props == nil {
		t.Error("Props is nil, want non-nil map")
	}
	if c.Content != "" {
		t.Errorf("Content = %q, want empty", c.Content)
	}
}

func TestSetProp(t *testing.T) {
	c := NewComponent("button")
	c.SetProp("label", "Click")
	if v, ok := c.Props["label"].(string); !ok || v != "Click" {
		t.Errorf("Props[label] = %v, want %q", c.Props["label"], "Click")
	}
}

func TestSetContent(t *testing.T) {
	c := NewComponent("card")
	c.SetContent("body text")
	if c.Content != "body text" {
		t.Errorf("Content = %q, want %q", c.Content, "body text")
	}
}

func TestRender_Button(t *testing.T) {
	t.Run("with label", func(t *testing.T) {
		c := NewComponent("button")
		c.SetProp("label", "Submit")
		rc, err := c.Render()
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !strings.Contains(rc.HTML, "Submit") {
			t.Errorf("HTML = %q, want to contain %q", rc.HTML, "Submit")
		}
		if !strings.Contains(rc.HTML, "<button") {
			t.Errorf("HTML = %q, want to contain <button", rc.HTML)
		}
	})

	t.Run("with disabled", func(t *testing.T) {
		c := NewComponent("button")
		c.SetProp("disabled", true)
		rc, err := c.Render()
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !strings.Contains(rc.HTML, "disabled") {
			t.Errorf("HTML = %q, want to contain 'disabled'", rc.HTML)
		}
	})
}

func TestRender_Input(t *testing.T) {
	c := NewComponent("input")
	c.SetProp("placeholder", "Enter name")
	rc, err := c.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(rc.HTML, `placeholder="Enter name"`) {
		t.Errorf("HTML = %q, want to contain placeholder", rc.HTML)
	}
	if !strings.Contains(rc.HTML, `<input`) {
		t.Errorf("HTML = %q, want to contain <input", rc.HTML)
	}
}

func TestRender_Card(t *testing.T) {
	c := NewComponent("card")
	c.SetProp("title", "My Card")
	c.SetContent("card body")
	rc, err := c.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(rc.HTML, "My Card") {
		t.Errorf("HTML = %q, want to contain title", rc.HTML)
	}
	if !strings.Contains(rc.HTML, "card body") {
		t.Errorf("HTML = %q, want to contain content", rc.HTML)
	}
}

func TestRender_Unknown(t *testing.T) {
	c := NewComponent("custom")
	c.SetContent("raw content")
	rc, err := c.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rc.HTML != "raw content" {
		t.Errorf("HTML = %q, want %q", rc.HTML, "raw content")
	}
}

func TestRenderTemplate(t *testing.T) {
	t.Run("valid template", func(t *testing.T) {
		result, err := RenderTemplate("Hello {{.Name}}!", map[string]string{"Name": "World"})
		if err != nil {
			t.Fatalf("RenderTemplate() error = %v", err)
		}
		if result != "Hello World!" {
			t.Errorf("RenderTemplate() = %q, want %q", result, "Hello World!")
		}
	})

	t.Run("invalid template", func(t *testing.T) {
		_, err := RenderTemplate("{{.Name", nil)
		if err == nil {
			t.Fatal("RenderTemplate() expected error for invalid template, got nil")
		}
	})
}
