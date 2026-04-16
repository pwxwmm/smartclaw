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
