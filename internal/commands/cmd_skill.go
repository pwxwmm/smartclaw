package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/instructkr/smartclaw/internal/skills"
	"github.com/instructkr/smartclaw/internal/store"
)

func init() {
	Register(Command{
		Name:    "skill",
		Summary: "Skill marketplace: install, publish, search, browse",
		Usage:   "/skill <install|publish|search|marketplace> [args]",
	}, skillHandler)
}

func skillHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("┌─────────────────────────────────────┐")
		fmt.Println("│       Skill Marketplace             │")
		fmt.Println("└─────────────────────────────────────┘")
		fmt.Println()
		fmt.Println("  Commands:")
		fmt.Println("    /skill install <name>   Install a skill from marketplace")
		fmt.Println("    /skill publish          Publish current skill to marketplace")
		fmt.Println("    /skill search <query>   Search marketplace for skills")
		fmt.Println("    /skill marketplace      Browse featured skills and categories")
		return nil
	}

	switch args[0] {
	case "install":
		return skillInstallHandler(args[1:])
	case "publish":
		return skillPublishHandler(args[1:])
	case "search":
		return skillSearchHandler(args[1:])
	case "marketplace":
		return skillMarketplaceHandler(args[1:])
	default:
		fmt.Printf("  Unknown subcommand: %s\n", args[0])
		fmt.Println("  Use: /skill install|publish|search|marketplace")
		return nil
	}
}

func skillInstallHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("  Usage: /skill install <name>")
		fmt.Println()
		fmt.Println("  Install a skill from the marketplace.")
		return nil
	}

	name := args[0]
	registry := getSkillRegistry()
	if registry == nil {
		fmt.Println("  Skill registry not available")
		return nil
	}

	mp := skills.NewMarketplace(registry)
	meta, err := mp.InstallSkill(name)
	if err != nil {
		fmt.Printf("  Failed to install skill %q: %v\n", name, err)
		return nil
	}

	fmt.Printf("  ✓ Installed skill: %s\n", meta.Name)
	if meta.Description != "" {
		fmt.Printf("    %s\n", meta.Description)
	}
	fmt.Printf("    Version: %s  Author: %s  Category: %s\n", meta.Version, meta.Author, meta.Category)
	return nil
}

func skillPublishHandler(args []string) error {
	registry := getSkillRegistry()
	if registry == nil {
		fmt.Println("  Skill registry not available")
		return nil
	}

	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		sm := skills.GetSkillManager()
		if sm == nil {
			fmt.Println("  No skill manager available. Specify skill name: /skill publish <name>")
			return nil
		}
		localSkills := sm.List()
		if len(localSkills) == 0 {
			fmt.Println("  No skills found to publish")
			return nil
		}

		fmt.Println("  Available skills to publish:")
		for i, s := range localSkills {
			if s.Source == "local" {
				fmt.Printf("    %d. %s - %s\n", i+1, s.Name, s.Description)
			}
		}
		fmt.Println()
		fmt.Println("  Usage: /skill publish <name>")
		return nil
	}

	mp := skills.NewMarketplace(registry)
	if err := mp.PublishSkill(name); err != nil {
		fmt.Printf("  Failed to publish skill %q: %v\n", name, err)
		return nil
	}

	fmt.Printf("  ✓ Published skill: %s\n", name)
	fmt.Println("  Your skill is now available in the marketplace")
	return nil
}

func skillSearchHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("  Usage: /skill search <query>")
		return nil
	}

	query := strings.Join(args, " ")
	registry := getSkillRegistry()
	if registry == nil {
		fmt.Println("  Skill registry not available")
		return nil
	}

	result, err := registry.Search(query, "", "", 1, 20)
	if err != nil {
		fmt.Printf("  Search failed: %v\n", err)
		return nil
	}

	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Printf("│  Search: %-27s│\n", truncate(query, 27))
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()

	if len(result.Skills) == 0 {
		fmt.Println("  No skills found")
		return nil
	}

	fmt.Printf("  Found %d skill(s)\n\n", result.Total)
	for _, meta := range result.Skills {
		icon := "○"
		switch meta.Source {
		case "bundled":
			icon = "●"
		case "local":
			icon = "◆"
		case "marketplace":
			icon = "★"
		}

		fmt.Printf("  %s %-25s  %s\n", icon, meta.Name, truncate(meta.Description, 40))
		fmt.Printf("     Category: %-12s  Downloads: %d  Rating: %.1f\n", meta.Category, meta.Downloads, meta.Rating)
		if len(meta.Tags) > 0 {
			fmt.Printf("     Tags: %s\n", strings.Join(meta.Tags, ", "))
		}
		fmt.Println()
	}

	return nil
}

func skillMarketplaceHandler(args []string) error {
	registry := getSkillRegistry()
	if registry == nil {
		fmt.Println("  Skill registry not available")
		return nil
	}

	mp := skills.NewMarketplace(registry)

	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│       Skill Marketplace             │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()

	categories := mp.GetCategories()
	fmt.Println("  Categories:")
	for _, cat := range categories {
		fmt.Printf("    • %s\n", cat)
	}
	fmt.Println()

	featured, err := mp.GetFeatured()
	if err != nil || len(featured) == 0 {
		fmt.Println("  No featured skills yet")
		return nil
	}

	fmt.Println("  Featured Skills:")
	for _, meta := range featured {
		star := "★"
		if meta.Rating < 4.0 {
			star = "☆"
		}
		fmt.Printf("    %s %-25s  %.1f/5  (%d downloads)\n", star, meta.Name, meta.Rating, meta.Downloads)
		if meta.Description != "" {
			fmt.Printf("       %s\n", truncate(meta.Description, 60))
		}
	}
	fmt.Println()

	installed, _ := registry.ListInstalled()
	if len(installed) > 0 {
		sort.Slice(installed, func(i, j int) bool {
			return installed[i].Name < installed[j].Name
		})
		fmt.Printf("  Installed Skills (%d):\n", len(installed))
		for _, meta := range installed {
			fmt.Printf("    ✓ %-25s  [%s]\n", meta.Name, meta.Source)
		}
	}

	return nil
}

func getSkillRegistry() *skills.Registry {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	skillsDir := filepath.Join(home, ".smartclaw", "skills")
	st, err := store.NewStore()
	if err != nil {
		return nil
	}

	registry := skills.NewRegistry(skillsDir, st)
	if err := registry.BuildIndex(); err != nil {
		return nil
	}

	return registry
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func fetchMarketplaceAPI(path string) (map[string]any, error) {
	baseURL := os.Getenv("SMARTCLAW_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	resp, err := http.Get(baseURL + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
