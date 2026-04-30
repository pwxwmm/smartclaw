package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/store"
)

func init() {
	Register(Command{
		Name:    "memory",
		Summary: "Show memory context",
	}, memoryHandler)

	Register(Command{
		Name:    "skills",
		Summary: "List available skills, check health, or improve failing skills",
	}, skillsHandler)

	Register(Command{
		Name:    "observe",
		Summary: "Observe mode",
	}, observeHandler)
}

func memoryHandler(args []string) error {
	home, _ := os.UserHomeDir()
	memoryPath := filepath.Join(home, ".sparkcode", "memory")

	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Memory Context              │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Printf("  Memory path: %s\n", memoryPath)
	fmt.Println()

	if _, err := os.Stat(memoryPath); os.IsNotExist(err) {
		fmt.Println("  No memory files found")
		fmt.Println()
		fmt.Println("  To add memory context:")
		fmt.Println("    1. Create ~/.smartclaw/memory/ directory")
		fmt.Println("    2. Add .md files with context")
		return nil
	}

	files, _ := os.ReadDir(memoryPath)
	if len(files) == 0 {
		fmt.Println("  Memory directory is empty")
		return nil
	}

	fmt.Println("  Memory files:")
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			info, _ := f.Info()
			fmt.Printf("    - %s (%d bytes)\n", f.Name(), info.Size())
		}
	}

	return nil
}

func skillsHandler(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "health":
			return skillsHealthHandler(args[1:])
		case "improve":
			return skillsImproveHandler(args[1:])
		}
	}

	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Available Skills            │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()

	home, _ := os.UserHomeDir()
	skillsPath := filepath.Join(home, ".sparkcode", "skills")

	if _, err := os.Stat(skillsPath); os.IsNotExist(err) {
		fmt.Println("  No custom skills found")
	}

	fmt.Println("  Bundled skills:")
	fmt.Println("    - help")
	fmt.Println("    - commit")
	fmt.Println("    - git-master")
	fmt.Println()
	fmt.Println("  Subcommands:")
	fmt.Println("    /skills health    Show skill health report")
	fmt.Println("    /skills improve <name>  Manually trigger skill improvement")
	return nil
}

func skillsHealthHandler(args []string) error {
	s, err := store.NewStore()
	if err != nil {
		fmt.Println("  Skill health: store not available")
		return nil
	}
	defer s.Close()

	tracker := learning.NewSkillTracker(s)

	scores, err := tracker.GetAllScores()
	if err != nil {
		fmt.Printf("  Error getting skill scores: %v\n", err)
		return nil
	}

	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Skill Health Report         │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()

	if len(scores) == 0 {
		fmt.Println("  No tracked skills found")
		fmt.Println("  Skills will appear here after being used")
		return nil
	}

	report, _ := tracker.GetHealthReport()
	trendMap := make(map[string]string)
	if report != nil {
		for _, entry := range report.Skills {
			trendMap[entry.SkillID] = entry.Trend
		}
	}

	healthy, degraded, failing, unused := 0, 0, 0, 0
	for _, score := range scores {
		if score.TotalInvocations < 2 {
			unused++
		} else if score.Score >= 0.7 {
			healthy++
		} else if score.Score >= 0.4 {
			degraded++
		} else {
			failing++
		}
	}

	fmt.Printf("  Summary: %d healthy, %d degraded, %d failing, %d unused\n",
		healthy, degraded, failing, unused)
	fmt.Println()

	var skillNames []string
	for name := range scores {
		skillNames = append(skillNames, name)
	}
	sort.Strings(skillNames)

	for _, name := range skillNames {
		score := scores[name]
		trend := trendMap[name]
		if trend == "" {
			trend = "stable"
		}

		var icon string
		var rec string
		if score.TotalInvocations < 2 {
			icon = "○"
			rec = "Insufficient data"
		} else if score.Score >= 0.7 {
			icon = "✓"
			rec = "Performing well"
		} else if score.Score >= 0.4 {
			icon = "⚠"
			rec = "Consider improving"
		} else {
			icon = "✗"
			rec = "Improve or retire"
		}

		fmt.Printf("  %s %-25s  %.0f%% success  [%s]  %s\n",
			icon, name, score.Score*100, trend, rec)
		fmt.Printf("     Invocations: %d  |  Successes: %d  |  Failures: %d\n",
			score.TotalInvocations, score.Successes, score.Failures)
	}

	return nil
}

func skillsImproveHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /skills improve <skill-name>")
		fmt.Println()
		fmt.Println("Manually triggers improvement for a specific skill.")
		fmt.Println("Use /skills health to identify failing skills.")
		return nil
	}

	skillName := args[0]

	s, err := store.NewStore()
	if err != nil {
		fmt.Println("  Skill improve: store not available")
		return nil
	}
	defer s.Close()

	tracker := learning.NewSkillTracker(s)
	tempImprover := learning.NewSkillImprover(nil)

	if !tempImprover.ShouldImprove(tracker, skillName) {
		score, _ := tracker.GetEffectivenessScore(skillName)
		if score.TotalInvocations < 3 {
			fmt.Printf("  Skill %q has insufficient data for improvement (%d invocations, need 3+)\n", skillName, score.TotalInvocations)
		} else {
			fmt.Printf("  Skill %q is performing well (%.0f%% success) — no improvement needed\n", skillName, score.Score*100)
		}
		return nil
	}

	fmt.Printf("  Skill %q qualifies for improvement (success rate below 50%%)\n", skillName)

	apiKey := cmdCtx.GetAPIKey()
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		fmt.Println("  No API key available — cannot perform LLM-guided improvement")
		fmt.Println("  Set API key via /set-api-key or ANTHROPIC_API_KEY environment variable")
		return nil
	}

	baseURL, model := loadCLIAPISettings()
	apiClient := api.NewClientWithModel(apiKey, baseURL, model)

	llmAdapter := learning.NewAPIClientAdapter(apiClient, "")
	improver := learning.NewSkillImprover(llmAdapter)
	writer := learning.NewSkillWriter("")

	originalSkill, err := loadSkillForImprovement(skillName)
	if err != nil {
		fmt.Printf("  Failed to load skill %q: %v\n", skillName, err)
		return nil
	}

	fmt.Printf("  Improving skill %q...\n", skillName)

	improved, err := improver.Improve(context.Background(), skillName, []string{"Manual improvement triggered via /skills improve"}, originalSkill)
	if err != nil {
		fmt.Printf("  Skill improvement failed: %v\n", err)
		return nil
	}

	if err := improver.ApplyImprovement(writer, improved); err != nil {
		fmt.Printf("  Failed to apply improvement: %v\n", err)
		return nil
	}

	fmt.Printf("  ✓ Skill %q improved to v%d\n", skillName, improved.Version)
	fmt.Printf("    Changes: %s\n", improved.ChangeSummary)

	return nil
}

func loadCLIAPISettings() (baseURL, model string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(home, ".smartclaw", "config.json"))
	if err != nil {
		return
	}
	var cfg struct {
		BaseURL string `json:"base_url"`
		Model   string `json:"model"`
	}
	if json.Unmarshal(data, &cfg) == nil {
		baseURL = cfg.BaseURL
		model = cfg.Model
	}
	return
}

func loadSkillForImprovement(name string) (*learning.ExtractedSkill, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	skillPath := filepath.Join(home, ".smartclaw", "skills", name, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return &learning.ExtractedSkill{
			Name:        name,
			Description: "Skill loaded for improvement",
			Steps:       []string{"Execute the skill"},
			Tools:       []string{"bash"},
			Tags:        []string{"learned"},
		}, nil
	}
	return learning.ParseExistingSkill(name, string(data)), nil
}

func healthIcon(h learning.SkillHealth) string {
	switch h {
	case learning.HealthHealthy:
		return "✓"
	case learning.HealthDegraded:
		return "⚠"
	case learning.HealthFailing:
		return "✗"
	case learning.HealthUnused:
		return "○"
	default:
		return "?"
	}
}

func observeHandler(args []string) error {
	fmt.Println("Observe mode enabled")
	fmt.Println("  Watching for file changes...")
	return nil
}
