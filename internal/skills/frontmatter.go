package skills

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const frontmatterDelim = "---"

// ParseSKILLFrontmatter extracts YAML frontmatter from a SKILL.md file.
// Returns the parsed schema, the remaining markdown body, and any error.
// If no frontmatter is found, returns nil schema, the original content, and nil error.
func ParseSKILLFrontmatter(content string) (*SkillSchema, string, error) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, frontmatterDelim) {
		return nil, content, nil
	}

	afterFirst := trimmed[len(frontmatterDelim):]
	endIdx := strings.Index(afterFirst, "\n"+frontmatterDelim)
	if endIdx < 0 {
		return nil, content, nil
	}

	yamlBlock := afterFirst[:endIdx]
	body := strings.TrimSpace(afterFirst[endIdx+len("\n"+frontmatterDelim):])

	var schema SkillSchema
	if err := yaml.Unmarshal([]byte(yamlBlock), &schema); err != nil {
		return nil, content, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	return &schema, body, nil
}

// RenderSKILLMarkdown renders a SkillSchema and body back into SKILL.md format
// with YAML frontmatter delimiters.
func RenderSKILLMarkdown(schema *SkillSchema, body string) (string, error) {
	if schema == nil {
		return body, nil
	}

	out, err := yaml.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema to YAML: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(frontmatterDelim)
	sb.WriteString("\n")
	sb.Write(out)
	sb.WriteString(frontmatterDelim)
	sb.WriteString("\n")

	if body != "" {
		sb.WriteString(body)
	}

	return sb.String(), nil
}

// ValidateSchema checks a SkillSchema for required fields and valid values.
// Returns a slice of ValidationErrors (empty if valid).
func ValidateSchema(schema *SkillSchema) []ValidationError {
	var errs []ValidationError

	if schema.Name == "" {
		errs = append(errs, ValidationError{Field: "name", Message: "name is required"})
	}

	if schema.Version == "" {
		errs = append(errs, ValidationError{Field: "version", Message: "version is required"})
	}

	if schema.Description == "" {
		errs = append(errs, ValidationError{Field: "description", Message: "description is required"})
	}

	for _, p := range schema.Platforms {
		if !ValidPlatforms[p] {
			errs = append(errs, ValidationError{
				Field:   "platforms",
				Message: fmt.Sprintf("invalid platform %q; valid: cli, web, telegram, slack, discord", p),
			})
		}
	}

	for i, cv := range schema.ConfigVars {
		fieldPrefix := fmt.Sprintf("config_vars[%d]", i)

		if cv.Name == "" {
			errs = append(errs, ValidationError{
				Field:   fieldPrefix + ".name",
				Message: "config var name is required",
			})
		}

		if !ValidConfigTypes[cv.Type] {
			errs = append(errs, ValidationError{
				Field:   fieldPrefix + ".type",
				Message: fmt.Sprintf("invalid config var type %q; valid: string, int, bool, enum", cv.Type),
			})
		}

		if cv.Type == "enum" && len(cv.EnumValues) == 0 {
			errs = append(errs, ValidationError{
				Field:   fieldPrefix + ".enum_values",
				Message: "enum type requires at least one enum_values entry",
			})
		}
	}

	for i, sc := range schema.SlashCommands {
		fieldPrefix := fmt.Sprintf("slash_commands[%d]", i)

		if sc.Name == "" {
			errs = append(errs, ValidationError{
				Field:   fieldPrefix + ".name",
				Message: "slash command name is required",
			})
		}

		if sc.Template == "" {
			errs = append(errs, ValidationError{
				Field:   fieldPrefix + ".template",
				Message: "slash command template is required",
			})
		}
	}

	return errs
}

// IsPlatformAllowed returns true if the skill is allowed on the given platform.
// If the schema has no platforms list (empty), the skill is allowed on all platforms.
func IsPlatformAllowed(schema *SkillSchema, platform string) bool {
	if len(schema.Platforms) == 0 {
		return true
	}
	for _, p := range schema.Platforms {
		if p == platform {
			return true
		}
	}
	return false
}

// ResolveConfigVars merges user-provided config values with schema defaults,
// performing type checking and required-field validation.
// Returns the resolved map and any errors encountered.
func ResolveConfigVars(schema *SkillSchema, provided map[string]any) (map[string]any, []error) {
	resolved := make(map[string]any, len(schema.ConfigVars))
	var errs []error

	for _, cv := range schema.ConfigVars {
		val, ok := provided[cv.Name]
		if !ok {
			if cv.Required && cv.Default == nil {
				errs = append(errs, fmt.Errorf("required config var %q not provided and has no default", cv.Name))
				continue
			}
			if cv.Default != nil {
				resolved[cv.Name] = cv.Default
			} else {
				resolved[cv.Name] = zeroValue(cv.Type)
			}
			continue
		}

		coerced, err := coerceType(val, cv)
		if err != nil {
			errs = append(errs, fmt.Errorf("config var %q: %w", cv.Name, err))
			continue
		}
		resolved[cv.Name] = coerced
	}

	return resolved, errs
}

// GenerateSlashCommands produces GeneratedCommand entries from a SkillSchema.
// Each SlashCommand in the schema becomes a GeneratedCommand with the template
// resolved against config var defaults.
func GenerateSlashCommands(schema *SkillSchema) []GeneratedCommand {
	cmds := make([]GeneratedCommand, 0, len(schema.SlashCommands))

	defaults := make(map[string]any, len(schema.ConfigVars))
	for _, cv := range schema.ConfigVars {
		if cv.Default != nil {
			defaults[cv.Name] = cv.Default
		}
	}

	for _, sc := range schema.SlashCommands {
		rendered := sc.Template
		for k, v := range defaults {
			rendered = strings.ReplaceAll(rendered, "{{"+k+"}}", fmt.Sprintf("%v", v))
		}

		cmds = append(cmds, GeneratedCommand{
			Name:           sc.Name,
			Description:    sc.Description,
			PromptTemplate: rendered,
			ConfigTemplate: sc.Template,
		})
	}

	return cmds
}

func coerceType(val any, cv ConfigVar) (any, error) {
	switch cv.Type {
	case "string":
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", val)
		}
		return s, nil

	case "int":
		switch v := val.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		case string:
			i, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %q to int: %w", v, err)
			}
			return i, nil
		default:
			return nil, fmt.Errorf("expected int, got %T", val)
		}

	case "bool":
		switch v := val.(type) {
		case bool:
			return v, nil
		case string:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %q to bool: %w", v, err)
			}
			return b, nil
		default:
			return nil, fmt.Errorf("expected bool, got %T", val)
		}

	case "enum":
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("expected string for enum, got %T", val)
		}
		for _, ev := range cv.EnumValues {
			if s == ev {
				return s, nil
			}
		}
		return nil, fmt.Errorf("value %q not in enum_values %v", s, cv.EnumValues)

	default:
		return val, nil
	}
}

func zeroValue(typ string) any {
	switch typ {
	case "string":
		return ""
	case "int":
		return 0
	case "bool":
		return false
	default:
		return nil
	}
}
