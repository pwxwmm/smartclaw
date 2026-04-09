package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewTemplateManager(t *testing.T) {
	tm := NewTemplateManager()
	if tm == nil {
		t.Fatal("Expected non-nil template manager")
	}

	if tm.templates == nil {
		t.Error("Expected templates map to be initialized")
	}
}

func TestTemplateManagerLoad(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "template-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := "Hello, {{.Name}}!"
	tmpFile.WriteString(content)
	tmpFile.Close()

	tm := NewTemplateManager()
	err = tm.Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Expected no error loading template, got %v", err)
	}

	if len(tm.templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(tm.templates))
	}
}

func TestTemplateManagerGet(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	path := tmpFile.Name()
	defer os.Remove(path)

	tmpFile.WriteString("test content")
	tmpFile.Close()

	tm := NewTemplateManager()
	tm.Load(path)

	expectedName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	tmpl, err := tm.Get(expectedName)
	if err != nil {
		t.Fatalf("Expected no error getting template, got %v", err)
	}

	if tmpl == nil {
		t.Fatal("Expected non-nil template")
	}
}

func TestTemplateManagerGetNonexistent(t *testing.T) {
	tm := NewTemplateManager()

	_, err := tm.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
}

func TestTemplateManagerList(t *testing.T) {
	tm := NewTemplateManager()

	list := tm.List()
	if len(list) != 0 {
		t.Errorf("Expected empty list, got %d", len(list))
	}
}

func TestTemplateManagerExecute(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	path := tmpFile.Name()
	defer os.Remove(path)

	content := "Hello, {{.Name}}!"
	tmpFile.WriteString(content)
	tmpFile.Close()

	tm := NewTemplateManager()
	tm.Load(path)

	expectedName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	result, err := tm.Execute(expectedName, map[string]string{"Name": "World"})
	if err != nil {
		t.Fatalf("Expected no error executing template, got %v", err)
	}

	if result != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", result)
	}
}

func TestTemplate(t *testing.T) {
	tmpl := Template{
		Name:        "test",
		Description: "Test template",
		Content:     "Hello, {{.Name}}",
		Variables: []Variable{
			{Name: "Name", Description: "Name to greet", Required: true},
		},
	}

	if tmpl.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", tmpl.Name)
	}

	if len(tmpl.Variables) != 1 {
		t.Errorf("Expected 1 variable, got %d", len(tmpl.Variables))
	}
}

func TestVariable(t *testing.T) {
	variable := Variable{
		Name:        "name",
		Description: "User name",
		Default:     "World",
		Required:    true,
	}

	if variable.Name != "name" {
		t.Errorf("Expected name 'name', got '%s'", variable.Name)
	}

	if variable.Default != "World" {
		t.Errorf("Expected default 'World', got '%s'", variable.Default)
	}

	if !variable.Required {
		t.Error("Expected required to be true")
	}
}

func TestTemplateManagerExecuteNonexistent(t *testing.T) {
	tm := NewTemplateManager()

	_, err := tm.Execute("nonexistent", map[string]string{})
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
}
