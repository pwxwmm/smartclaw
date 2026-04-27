package commands

import (
	"fmt"
	"os"
	"strings"
)

func init() {
	Register(Command{
		Name:    "template",
		Summary: "Manage prompt templates",
	}, templateHandler)

	Register(Command{
		Name:    "template-list",
		Summary: "List templates",
	}, templateListHandler)

	Register(Command{
		Name:    "template-use",
		Summary: "Use a template",
	}, templateUseHandler)

	Register(Command{
		Name:    "template-create",
		Summary: "Create template",
	}, templateCreateHandler)

	Register(Command{
		Name:    "template-delete",
		Summary: "Delete template",
	}, templateDeleteHandler)

	Register(Command{
		Name:    "template-info",
		Summary: "Show template info",
	}, templateInfoHandler)

	Register(Command{
		Name:    "template-export",
		Summary: "Export template",
	}, templateExportHandler)

	Register(Command{
		Name:    "template-import",
		Summary: "Import template",
	}, templateImportHandler)
}

var globalTemplateManager interface {
	GetTemplate(id string) (any, error)
	ListTemplates() []any
	CreateTemplate(template any) error
	DeleteTemplate(id string) error
	RenderTemplate(id string, variables map[string]string) (string, error)
	FormatTemplateInfo(any) string
	FormatTemplateList() string
	ExportTemplate(id string, format string) (string, error)
	ImportTemplate(data string, format string) error
}

type TemplateInfo struct {
	ID          string
	Name        string
	Description string
	Content     string
	Variables   []TemplateVariable
	Tags        []string
	Category    string
	IsBuiltIn   bool
}

type TemplateVariable struct {
	Name         string
	Description  string
	DefaultValue string
	Required     bool
}

func SetGlobalTemplateManager(tm interface {
	GetTemplate(id string) (any, error)
	ListTemplates() []any
	CreateTemplate(template any) error
	DeleteTemplate(id string) error
	RenderTemplate(id string, variables map[string]string) (string, error)
	FormatTemplateInfo(any) string
	FormatTemplateList() string
	ExportTemplate(id string, format string) (string, error)
	ImportTemplate(data string, format string) error
}) {
	globalTemplateManager = tm
}

func templateHandler(args []string) error {
	if len(args) == 0 {
		return templateListHandler(args)
	}
	switch args[0] {
	case "list", "ls":
		return templateListHandler(args[1:])
	case "use":
		return templateUseHandler(args[1:])
	case "create":
		return templateCreateHandler(args[1:])
	case "delete", "rm":
		return templateDeleteHandler(args[1:])
	case "info":
		return templateInfoHandler(args[1:])
	case "export":
		return templateExportHandler(args[1:])
	case "import":
		return templateImportHandler(args[1:])
	default:
		fmt.Printf("Unknown template subcommand: %s\n", args[0])
		fmt.Println("Usage: /template [list|use|create|delete|info|export|import]")
		return nil
	}
}

func templateListHandler(args []string) error {
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	fmt.Print(globalTemplateManager.FormatTemplateList())
	return nil
}

func templateUseHandler(args []string) error {
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	if len(args) == 0 {
		fmt.Println("Usage: /template use <template-id> [var1=value1] [var2=value2] ...")
		return nil
	}
	templateID := args[0]
	variables := make(map[string]string)
	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			variables[parts[0]] = parts[1]
		}
	}
	content, err := globalTemplateManager.RenderTemplate(templateID, variables)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("模板内容:\n%s\n", content)
	return nil
}

func templateCreateHandler(args []string) error {
	if len(args) < 3 {
		fmt.Println("Usage: /template create <id> <name> <description> <content>")
		return nil
	}
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	template := &TemplateInfo{
		ID:          args[0],
		Name:        args[1],
		Description: args[2],
		Content:     strings.Join(args[3:], " "),
	}
	if err := globalTemplateManager.CreateTemplate(template); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Created template: %s\n", args[0])
	return nil
}

func templateDeleteHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /template delete <template-id>")
		return nil
	}
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	if err := globalTemplateManager.DeleteTemplate(args[0]); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Deleted template: %s\n", args[0])
	return nil
}

func templateInfoHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /template info <template-id>")
		return nil
	}
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	template, err := globalTemplateManager.GetTemplate(args[0])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Print(globalTemplateManager.FormatTemplateInfo(template))
	return nil
}

func templateExportHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /template export <template-id> [json|md]")
		return nil
	}
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	templateID := args[0]
	format := "json"
	if len(args) > 1 {
		format = args[1]
	}
	content, err := globalTemplateManager.ExportTemplate(templateID, format)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("%s\n", content)
	return nil
}

func templateImportHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /template import <file-path>")
		return nil
	}
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	filePath := args[0]
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return nil
	}
	format := "json"
	if strings.HasSuffix(filePath, ".md") {
		format = "md"
	}
	if err := globalTemplateManager.ImportTemplate(string(data), format); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Imported template from: %s\n", filePath)
	return nil
}
