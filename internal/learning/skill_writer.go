package learning

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type SkillWriter struct {
	skillsDir string
}

func NewSkillWriter(skillsDir string) *SkillWriter {
	if skillsDir == "" {
		home, _ := os.UserHomeDir()
		skillsDir = filepath.Join(home, ".smartclaw", "skills")
	}
	return &SkillWriter{skillsDir: skillsDir}
}

func (sw *SkillWriter) WriteSkill(skill *ExtractedSkill) error {
	skillDir := filepath.Join(sw.skillsDir, skill.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("skill writer: mkdir %q: %w", skillDir, err)
	}

	existingPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(existingPath); err == nil {
		backupPath := filepath.Join(skillDir, "SKILL.md.bak")
		if err := os.Rename(existingPath, backupPath); err != nil {
			slog.Warn("failed to backup existing skill", "error", err, "path", existingPath)
		}
	}

	content := formatSkillMarkdown(skill)
	if err := os.WriteFile(existingPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("skill writer: write %q: %w", existingPath, err)
	}

	return nil
}

func (sw *SkillWriter) GetSkillsDir() string {
	return sw.skillsDir
}

func formatSkillMarkdown(skill *ExtractedSkill) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s Skill\n\n", toTitle(skill.Name)))
	sb.WriteString(skill.Description + "\n\n")

	sb.WriteString("## Triggers\n")
	for _, t := range skill.Triggers {
		sb.WriteString(fmt.Sprintf("- %s\n", t))
	}
	sb.WriteString("\n")

	sb.WriteString("## Tools\n")
	for _, t := range skill.Tools {
		sb.WriteString(fmt.Sprintf("- %s\n", t))
	}
	sb.WriteString("\n")

	sb.WriteString("## Instructions\n\n")
	for i, step := range skill.Steps {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
	}
	sb.WriteString("\n")

	sb.WriteString("## Tags\n")
	sb.WriteString(strings.Join(skill.Tags, ", ") + "\n")

	return sb.String()
}

func toTitle(kebab string) string {
	parts := strings.Split(kebab, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func readSkillFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read skill file: %w", err)
	}
	return string(data), nil
}

func ParseExistingSkill(name, content string) *ExtractedSkill {
	skill := &ExtractedSkill{
		Name:  name,
		Steps: []string{},
		Tags:  []string{},
	}

	lines := strings.Split(content, "\n")
	section := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "## Triggers"):
			section = "triggers"
		case strings.HasPrefix(trimmed, "## Tools"):
			section = "tools"
		case strings.HasPrefix(trimmed, "## Instructions"):
			section = "instructions"
		case strings.HasPrefix(trimmed, "## Tags"):
			section = "tags"
		case strings.HasPrefix(trimmed, "## "):
			section = ""
		case trimmed == "":
			continue
		default:
			switch section {
			case "triggers":
				if strings.HasPrefix(trimmed, "- ") {
					skill.Triggers = append(skill.Triggers, strings.TrimPrefix(trimmed, "- "))
				}
			case "tools":
				if strings.HasPrefix(trimmed, "- ") {
					skill.Tools = append(skill.Tools, strings.TrimPrefix(trimmed, "- "))
				}
			case "instructions":
				if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' {
					step := trimmed
					if idx := strings.Index(trimmed, ". "); idx >= 0 {
						step = strings.TrimSpace(trimmed[idx+2:])
					}
					skill.Steps = append(skill.Steps, step)
				}
			case "tags":
				if strings.Contains(trimmed, ",") {
					for _, tag := range strings.Split(trimmed, ",") {
						t := strings.TrimSpace(tag)
						if t != "" {
							skill.Tags = append(skill.Tags, t)
						}
					}
				} else if trimmed != "" {
					skill.Tags = append(skill.Tags, trimmed)
				}
			default:
				if skill.Description == "" && trimmed != "" {
					skill.Description = trimmed
				}
			}
		}
	}

	if skill.Description == "" {
		skill.Description = "Learned skill: " + name
	}
	if len(skill.Steps) == 0 {
		skill.Steps = []string{"Execute the skill as originally defined"}
	}
	if len(skill.Tools) == 0 {
		skill.Tools = []string{"bash"}
	}

	return skill
}
