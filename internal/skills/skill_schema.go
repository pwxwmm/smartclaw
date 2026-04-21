package skills

// SkillSchema represents the agentskills.io open standard for SKILL.md YAML frontmatter.
// It defines a skill's metadata, platform gating, configuration variables,
// and slash command auto-generation in a machine-readable format.
type SkillSchema struct {
	Name          string         `yaml:"name" json:"name"`
	Version       string         `yaml:"version" json:"version"`
	Description   string         `yaml:"description" json:"description"`
	Author        string         `yaml:"author" json:"author,omitempty"`
	Platforms     []string       `yaml:"platforms" json:"platforms,omitempty"`
	ConfigVars    []ConfigVar    `yaml:"config_vars" json:"config_vars,omitempty"`
	SlashCommands []SlashCommand `yaml:"slash_commands" json:"slash_commands,omitempty"`
	Tags          []string       `yaml:"tags" json:"tags,omitempty"`
	Tools         []string       `yaml:"tools" json:"tools,omitempty"`
	Triggers      []string       `yaml:"triggers" json:"triggers,omitempty"`
	Requires      []string       `yaml:"requires" json:"requires,omitempty"`
}

// ConfigVar defines a configuration variable for a skill.
// Config vars can be resolved at runtime with user-provided values
// or defaults, with type checking and validation.
type ConfigVar struct {
	Name        string   `yaml:"name" json:"name"`
	Type        string   `yaml:"type" json:"type"` // "string", "int", "bool", "enum"
	Default     any      `yaml:"default" json:"default,omitempty"`
	Description string   `yaml:"description" json:"description"`
	Required    bool     `yaml:"required" json:"required"`
	EnumValues  []string `yaml:"enum_values" json:"enum_values,omitempty"`
}

// SlashCommand defines an auto-generated slash command from the skill schema.
type SlashCommand struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Template    string `yaml:"template" json:"template"` // prompt template with {{var}} placeholders
}

// GeneratedCommand is the resolved form of a SlashCommand ready for registration.
type GeneratedCommand struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	PromptTemplate string `json:"prompt_template"`
	ConfigTemplate string `json:"config_template,omitempty"`
}

// ValidationError represents a single validation failure in a SkillSchema.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// ValidPlatforms is the set of all recognized platform identifiers.
var ValidPlatforms = map[string]bool{
	"cli":      true,
	"web":      true,
	"telegram": true,
	"slack":    true,
	"discord":  true,
}

// ValidConfigTypes is the set of all recognized config var types.
var ValidConfigTypes = map[string]bool{
	"string": true,
	"int":    true,
	"bool":   true,
	"enum":   true,
}
