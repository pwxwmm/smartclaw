package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewTemplateManager(t *testing.T) {
	tm := NewTemplateManagerWithPath(t.TempDir())
	if tm == nil {
		t.Fatal("Expected non-nil template manager")
	}

	builtIn := tm.List()
	if len(builtIn) < 10 {
		t.Errorf("Expected at least 10 built-in templates, got %d", len(builtIn))
	}
}

func TestGetBuiltIn(t *testing.T) {
	tm := NewTemplateManagerWithPath(t.TempDir())

	tmpl, err := tm.Get("code-review")
	if err != nil {
		t.Fatalf("Expected no error getting built-in template, got %v", err)
	}
	if tmpl == nil {
		t.Fatal("Expected non-nil template")
	}
	if !tmpl.IsBuiltIn {
		t.Error("Expected IsBuiltIn to be true")
	}
}

func TestGetNonexistent(t *testing.T) {
	tm := NewTemplateManagerWithPath(t.TempDir())

	_, err := tm.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
}

func TestRender(t *testing.T) {
	tm := NewTemplateManagerWithPath(t.TempDir())

	result, err := tm.Render("commit-message", map[string]string{"diff": "added new feature"})
	if err != nil {
		t.Fatalf("Expected no error rendering template, got %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty rendered content")
	}
	if !contains(result, "added new feature") {
		t.Error("Expected rendered content to contain the diff")
	}
}

func TestRenderWithMissingRequired(t *testing.T) {
	tm := NewTemplateManagerWithPath(t.TempDir())

	_, err := tm.Render("debug", map[string]string{})
	if err == nil {
		t.Error("Expected error for missing required variable")
	}
}

func TestRenderWithDefaults(t *testing.T) {
	tm := NewTemplateManagerWithPath(t.TempDir())

	result, err := tm.Render("debug", map[string]string{
		"code":     "fmt.Println()",
		"language": "go",
	})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty rendered content")
	}
}

func TestCreateAndDelete(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTemplateManagerWithPath(tmpDir)

	custom := &Template{
		ID:          "my-custom",
		Name:        "My Custom",
		Description: "A custom template",
		Content:     "Hello {{name}}!",
		Variables: []Variable{
			{Name: "name", Description: "Name", Required: true},
		},
	}

	err := tm.Create(custom)
	if err != nil {
		t.Fatalf("Expected no error creating template, got %v", err)
	}

	retrieved, err := tm.Get("my-custom")
	if err != nil {
		t.Fatalf("Expected no error getting template, got %v", err)
	}
	if retrieved.Name != "My Custom" {
		t.Errorf("Expected name 'My Custom', got '%s'", retrieved.Name)
	}
	if retrieved.IsBuiltIn {
		t.Error("Expected IsBuiltIn to be false")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "my-custom.json")); os.IsNotExist(err) {
		t.Error("Expected template file to be persisted")
	}

	err = tm.Delete("my-custom")
	if err != nil {
		t.Fatalf("Expected no error deleting template, got %v", err)
	}

	_, err = tm.Get("my-custom")
	if err == nil {
		t.Error("Expected error getting deleted template")
	}
}

func TestCreateDuplicate(t *testing.T) {
	tm := NewTemplateManagerWithPath(t.TempDir())

	err := tm.Create(&Template{ID: "code-review", Name: "Dup"})
	if err == nil {
		t.Error("Expected error creating duplicate template")
	}
}

func TestDeleteBuiltIn(t *testing.T) {
	tm := NewTemplateManagerWithPath(t.TempDir())

	err := tm.Delete("code-review")
	if err == nil {
		t.Error("Expected error deleting built-in template")
	}
}

func TestUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTemplateManagerWithPath(tmpDir)

	tm.Create(&Template{ID: "custom-tpl", Name: "Original", Content: "v1"})

	err := tm.Update("custom-tpl", &Template{Name: "Updated", Content: "v2"})
	if err != nil {
		t.Fatalf("Expected no error updating template, got %v", err)
	}

	retrieved, _ := tm.Get("custom-tpl")
	if retrieved.Name != "Updated" {
		t.Errorf("Expected name 'Updated', got '%s'", retrieved.Name)
	}
}

func TestSearch(t *testing.T) {
	tm := NewTemplateManagerWithPath(t.TempDir())

	results := tm.Search("review")
	if len(results) == 0 {
		t.Error("Expected search results for 'review'")
	}
}

func TestListByCategory(t *testing.T) {
	tm := NewTemplateManagerWithPath(t.TempDir())

	categories := tm.ListByCategory()
	if len(categories) == 0 {
		t.Error("Expected categories")
	}
}

func TestExportImport(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTemplateManagerWithPath(tmpDir)

	jsonExport, err := tm.ExportTemplate("code-review", "json")
	if err != nil {
		t.Fatalf("Expected no error exporting template, got %v", err)
	}
	if jsonExport == "" {
		t.Error("Expected non-empty export")
	}

	mdExport, err := tm.ExportTemplate("code-review", "md")
	if err != nil {
		t.Fatalf("Expected no error exporting template as markdown, got %v", err)
	}
	if mdExport == "" {
		t.Error("Expected non-empty markdown export")
	}

	jsonExport = strings.Replace(jsonExport, `"id": "code-review"`, `"id": "imported-review"`, 1)
	jsonExport = strings.Replace(jsonExport, `"isBuiltIn": true`, `"isBuiltIn": false`, 1)

	tm2 := NewTemplateManagerWithPath(t.TempDir())
	err = tm2.ImportTemplate(jsonExport, "json")
	if err != nil {
		t.Fatalf("Expected no error importing template, got %v", err)
	}

	imported, err := tm2.Get("imported-review")
	if err != nil {
		t.Fatalf("Expected imported template to exist, got error: %v", err)
	}
	if imported.IsBuiltIn {
		t.Error("Expected imported template to not be built-in")
	}
}

func TestExtractVariables(t *testing.T) {
	content := "Hello {{name}}, welcome to {{project}}!"
	vars := ExtractVariables(content)

	if len(vars) != 2 {
		t.Fatalf("Expected 2 variables, got %d", len(vars))
	}
	if vars[0].Name != "name" {
		t.Errorf("Expected first var 'name', got '%s'", vars[0].Name)
	}
	if vars[1].Name != "project" {
		t.Errorf("Expected second var 'project', got '%s'", vars[1].Name)
	}
}

func TestExtractVariablesNoDups(t *testing.T) {
	content := "{{name}} and {{name}} again"
	vars := ExtractVariables(content)

	if len(vars) != 1 {
		t.Errorf("Expected 1 unique variable, got %d", len(vars))
	}
}

func TestCustomTemplatePersistsAcrossManagers(t *testing.T) {
	tmpDir := t.TempDir()

	tm1 := NewTemplateManagerWithPath(tmpDir)
	tm1.Create(&Template{
		ID:      "persist-test",
		Name:    "Persist Test",
		Content: "test {{var}}",
	})

	tm2 := NewTemplateManagerWithPath(tmpDir)
	tmpl, err := tm2.Get("persist-test")
	if err != nil {
		t.Fatalf("Expected template to persist, got error: %v", err)
	}
	if tmpl.Name != "Persist Test" {
		t.Errorf("Expected name 'Persist Test', got '%s'", tmpl.Name)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
