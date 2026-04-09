package skills

import (
	"context"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/services"
)

type MemoryIntegration struct {
	sessionMemory *services.SessionMemoryService
	teamMemory    *services.TeamMemorySync
}

func NewMemoryIntegration(sessionMemory *services.SessionMemoryService, teamMemory *services.TeamMemorySync) *MemoryIntegration {
	return &MemoryIntegration{
		sessionMemory: sessionMemory,
		teamMemory:    teamMemory,
	}
}

func (mi *MemoryIntegration) RecordSkillUsage(ctx context.Context, sessionID, skillName, context string) error {
	if mi.sessionMemory == nil {
		return nil
	}

	metadataKey := "skill_usage_" + skillName
	mem := mi.sessionMemory.Get(sessionID)
	if mem == nil {
		return nil
	}

	usageList, _ := mem.Metadata[metadataKey].([]string)
	usageList = append(usageList, context)
	mem.Metadata[metadataKey] = usageList

	return nil
}

func (mi *MemoryIntegration) GetSkillContext(ctx context.Context, sessionID, skillName string) (string, error) {
	var contextParts []string

	if mi.sessionMemory != nil && sessionID != "" {
		sessionCtx := mi.sessionMemory.GetContextForPrompt(sessionID, 2000)
		if sessionCtx != "" {
			contextParts = append(contextParts, "Session context:\n"+sessionCtx)
		}

		mem := mi.sessionMemory.Get(sessionID)
		if mem != nil {
			if usageList, ok := mem.Metadata["skill_usage_"+skillName].([]string); ok && len(usageList) > 0 {
				recentUsage := usageList
				if len(recentUsage) > 5 {
					recentUsage = recentUsage[len(recentUsage)-5:]
				}
				contextParts = append(contextParts, "Recent skill usage:\n"+strings.Join(recentUsage, "\n"))
			}
		}
	}

	if mi.teamMemory != nil {
		memories, err := mi.teamMemory.SearchMemories(ctx, skillName)
		if err == nil && len(memories) > 0 {
			var teamContext []string
			for _, m := range memories {
				if len(teamContext) >= 3 {
					break
				}
				teamContext = append(teamContext, m.Title+": "+m.Content[:min(200, len(m.Content))])
			}
			if len(teamContext) > 0 {
				contextParts = append(contextParts, "Team knowledge:\n"+strings.Join(teamContext, "\n"))
			}
		}
	}

	if len(contextParts) == 0 {
		return "", nil
	}

	return strings.Join(contextParts, "\n\n"), nil
}

func (mi *MemoryIntegration) GetRelevantSkills(ctx context.Context, sessionID string, skills []*Skill) []*Skill {
	if mi.sessionMemory == nil || sessionID == "" {
		return skills
	}

	mem := mi.sessionMemory.Get(sessionID)
	if mem == nil {
		return skills
	}

	var recentMessages string
	for i := len(mem.Messages) - 1; i >= 0 && len(recentMessages) < 1000; i-- {
		recentMessages = mem.Messages[i].Content + " " + recentMessages
	}

	relevanceScores := make(map[string]int)
	for _, skill := range skills {
		score := 0

		for _, tag := range skill.Tags {
			if strings.Contains(strings.ToLower(recentMessages), strings.ToLower(tag)) {
				score += 10
			}
		}

		for _, tool := range skill.Tools {
			if strings.Contains(strings.ToLower(recentMessages), strings.ToLower(tool)) {
				score += 5
			}
		}

		for _, cmd := range skill.Commands {
			if strings.Contains(strings.ToLower(recentMessages), strings.ToLower(cmd)) {
				score += 8
			}
		}

		if usageList, ok := mem.Metadata["skill_usage_"+skill.Name].([]string); ok {
			score += len(usageList) * 3
		}

		relevanceScores[skill.Name] = score
	}

	type scoredSkill struct {
		skill *Skill
		score int
	}

	var scoredSkills []scoredSkill
	for _, s := range skills {
		scoredSkills = append(scoredSkills, scoredSkill{skill: s, score: relevanceScores[s.Name]})
	}

	for i := 0; i < len(scoredSkills)-1; i++ {
		for j := i + 1; j < len(scoredSkills); j++ {
			if scoredSkills[j].score > scoredSkills[i].score {
				scoredSkills[i], scoredSkills[j] = scoredSkills[j], scoredSkills[i]
			}
		}
	}

	result := make([]*Skill, 0, len(scoredSkills))
	for _, ss := range scoredSkills {
		result = append(result, ss.skill)
	}

	return result
}

func (mi *MemoryIntegration) ShareSkillPattern(ctx context.Context, skillName, pattern string) error {
	if mi.teamMemory == nil {
		return nil
	}

	memory := &services.Memory{
		Type:       services.MemoryTypePattern,
		Visibility: services.VisibilityTeam,
		Title:      "Skill pattern: " + skillName,
		Content:    pattern,
		Tags:       []string{"skill", skillName, "pattern"},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	return mi.teamMemory.ShareMemory(ctx, memory)
}

func (mi *MemoryIntegration) LearnFromSession(ctx context.Context, sessionID string, skills []*Skill) error {
	if mi.sessionMemory == nil {
		return nil
	}

	mem := mi.sessionMemory.Get(sessionID)
	if mem == nil || len(mem.Messages) == 0 {
		return nil
	}

	patterns := extractPatterns(mem.Messages)

	for _, skill := range skills {
		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(pattern), strings.ToLower(skill.Name)) {
				if err := mi.ShareSkillPattern(ctx, skill.Name, pattern); err == nil {
					break
				}
			}
		}
	}

	return nil
}

func extractPatterns(messages []services.MemoryMessage) []string {
	var patterns []string
	var currentPattern strings.Builder

	for _, msg := range messages {
		if msg.Role == "user" && len(msg.Content) > 50 {
			if currentPattern.Len() > 0 {
				currentPattern.WriteString(" -> ")
			}
			currentPattern.WriteString(truncatePattern(msg.Content, 100))
		} else if msg.Role == "assistant" && currentPattern.Len() > 0 {
			currentPattern.WriteString(" => ")
			currentPattern.WriteString(truncatePattern(msg.Content, 100))
			patterns = append(patterns, currentPattern.String())
			currentPattern.Reset()
		}
	}

	return patterns
}

func truncatePattern(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
