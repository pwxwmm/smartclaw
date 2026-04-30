package commands

import (
	"fmt"
	"strings"

	"github.com/instructkr/smartclaw/internal/plugins"
)

func init() {
	Register(Command{
		Name:    "plugin",
		Summary: "Manage plugins (install, list, remove, enable, disable)",
		Usage:   "/plugin [list|install|remove|enable|disable|info|search|stats]",
	}, cmdPluginHandler)
}

type PluginRegistryInterface interface {
	List() []*plugins.Plugin
	ListDynamic() []plugins.PluginInterface
	Get(name string) *plugins.Plugin
	Install(source string) (*plugins.Plugin, error)
	Uninstall(name string) error
	Enable(name string) error
	Disable(name string) error
	Search(query string) ([]plugins.MarketplacePlugin, error)
	GetStats() plugins.RegistryStats
	GetPluginSource(name string) plugins.RegistrySource
}

var globalPluginRegistry PluginRegistryInterface

func SetGlobalPluginRegistry(registry PluginRegistryInterface) {
	globalPluginRegistry = registry
}

func cmdPluginHandler(args []string) error {
	if globalPluginRegistry == nil {
		return pluginNoRegistry()
	}

	if len(args) == 0 {
		return pluginListHandler()
	}

	switch args[0] {
	case "list", "ls":
		return pluginListHandler()
	case "install", "add":
		return pluginInstallHandler(args[1:])
	case "remove", "rm", "uninstall":
		return pluginRemoveHandler(args[1:])
	case "enable":
		return pluginEnableHandler(args[1:])
	case "disable":
		return pluginDisableHandler(args[1:])
	case "info":
		return pluginInfoHandler(args[1:])
	case "search":
		return pluginSearchHandler(args[1:])
	case "stats":
		return pluginStatsHandler()
	default:
		fmt.Printf("Unknown plugin subcommand: %s\n", args[0])
		fmt.Println("Usage: /plugin [list|install|remove|enable|disable|info|search|stats]")
		return nil
	}
}

func pluginNoRegistry() error {
	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Println("│         Plugin Manager                    │")
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()
	fmt.Println("  Plugin registry not initialized")
	fmt.Println()
	fmt.Println("  Subcommands:")
	fmt.Println("    /plugin list              List all plugins")
	fmt.Println("    /plugin install <source>  Install a plugin")
	fmt.Println("    /plugin remove <name>     Uninstall a plugin")
	fmt.Println("    /plugin enable <name>     Enable a plugin")
	fmt.Println("    /plugin disable <name>    Disable a plugin")
	fmt.Println("    /plugin info <name>       Show plugin details")
	fmt.Println("    /plugin search <query>    Search marketplace")
	fmt.Println("    /plugin stats             Show registry statistics")
	return nil
}

func pluginListHandler() error {
	pluginsList := globalPluginRegistry.List()
	dynamicList := globalPluginRegistry.ListDynamic()

	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Println("│         Installed Plugins                 │")
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()

	total := len(pluginsList) + len(dynamicList)
	if total == 0 {
		fmt.Println("  No plugins installed")
		fmt.Println()
		fmt.Println("  Install one with:")
		fmt.Println("    /plugin install <url|github:org/repo|/path>")
		return nil
	}

	fmt.Printf("  %-20s %-10s %-10s %-8s %-12s\n", "Name", "Version", "Type", "Enabled", "Source")
	fmt.Printf("  %-20s %-10s %-10s %-8s %-12s\n", "--------------------", "----------", "----------", "--------", "------------")

	for _, p := range pluginsList {
		enabled := "\u2713 Yes"
		if !p.Enabled {
			enabled = "\u2717 No"
		}

		pType := pluginType(p)
		source := string(globalPluginRegistry.GetPluginSource(p.Name))

		name := p.Name
		if len(name) > 18 {
			name = name[:15] + "..."
		}

		fmt.Printf("  %-20s %-10s %-10s %-8s %-12s\n", name, p.Version, pType, enabled, source)
	}

	for _, dp := range dynamicList {
		info := plugins.GetPluginInfo(dp)
		enabled := "\u2713 Yes"

		pType := "dynamic"
		if len(info.Capabilities) > 0 {
			pType = string(info.Capabilities[0])
		}

		name := info.Name
		if len(name) > 18 {
			name = name[:15] + "..."
		}

		fmt.Printf("  %-20s %-10s %-10s %-8s %-12s\n", name, info.Version, pType, enabled, "dynamic")
	}

	fmt.Println()
	fmt.Printf("  Total: %d plugins (%d dynamic)\n", total, len(dynamicList))
	return nil
}

func pluginInstallHandler(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: /plugin install <source>")
		fmt.Println()
		fmt.Println("Sources:")
		fmt.Println("  URL:      https://example.com/plugin.tar.gz")
		fmt.Println("  GitHub:   github:owner/repo")
		fmt.Println("  Local:    /path/to/plugin/directory")
		return nil
	}

	source := args[0]
	fmt.Printf("  Installing plugin from %s...\n", source)

	p, err := globalPluginRegistry.Install(source)
	if err != nil {
		fmt.Printf("  \u2717 Install failed: %v\n", err)
		return nil
	}

	fmt.Printf("  \u2713 Plugin installed: %s v%s\n", p.Name, p.Version)
	if p.Description != "" {
		fmt.Printf("    %s\n", p.Description)
	}
	return nil
}

func pluginRemoveHandler(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: /plugin remove <name>")
		return nil
	}

	name := args[0]
	if err := globalPluginRegistry.Uninstall(name); err != nil {
		fmt.Printf("  \u2717 Failed to remove plugin %s: %v\n", name, err)
		return nil
	}

	fmt.Printf("  \u2713 Plugin %s removed\n", name)
	return nil
}

func pluginEnableHandler(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: /plugin enable <name>")
		return nil
	}

	name := args[0]
	if err := globalPluginRegistry.Enable(name); err != nil {
		fmt.Printf("  \u2717 Failed to enable plugin %s: %v\n", name, err)
		return nil
	}

	fmt.Printf("  \u2713 Plugin %s enabled\n", name)
	return nil
}

func pluginDisableHandler(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: /plugin disable <name>")
		return nil
	}

	name := args[0]
	if err := globalPluginRegistry.Disable(name); err != nil {
		fmt.Printf("  \u2717 Failed to disable plugin %s: %v\n", name, err)
		return nil
	}

	fmt.Printf("  \u2713 Plugin %s disabled\n", name)
	return nil
}

func pluginInfoHandler(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: /plugin info <name>")
		return nil
	}

	name := args[0]
	p := globalPluginRegistry.Get(name)
	if p == nil {
		fmt.Printf("  \u2717 Plugin not found: %s\n", name)
		return nil
	}

	source := globalPluginRegistry.GetPluginSource(name)

	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Printf("│  Plugin: %-31s │\n", p.Name)
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()

	fmt.Printf("  Name:        %s\n", p.Name)
	fmt.Printf("  Version:     %s\n", p.Version)
	if p.Description != "" {
		fmt.Printf("  Description: %s\n", p.Description)
	}
	if p.Author != "" {
		fmt.Printf("  Author:      %s\n", p.Author)
	}
	fmt.Printf("  Enabled:     %v\n", p.Enabled)
	fmt.Printf("  Source:      %s\n", source)
	if p.Main != "" {
		fmt.Printf("  Entry:       %s\n", p.Main)
	}
	if len(p.Tools) > 0 {
		fmt.Printf("  Tools:       %s\n", strings.Join(p.Tools, ", "))
	}
	if len(p.Hooks) > 0 {
		fmt.Printf("  Hooks:       %s\n", strings.Join(p.Hooks, ", "))
	}
	if len(p.Commands) > 0 {
		fmt.Printf("  Commands:    %s\n", strings.Join(p.Commands, ", "))
	}
	if len(p.Config) > 0 {
		fmt.Println("  Config:")
		for k, v := range p.Config {
			fmt.Printf("    %-20s %s\n", k, v)
		}
	}
	if !p.InstalledAt.IsZero() {
		fmt.Printf("  Installed:   %s\n", p.InstalledAt.Format("2006-01-02 15:04"))
	}
	if !p.UpdatedAt.IsZero() {
		fmt.Printf("  Updated:     %s\n", p.UpdatedAt.Format("2006-01-02 15:04"))
	}

	return nil
}

func pluginSearchHandler(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: /plugin search <query>")
		return nil
	}

	query := strings.Join(args, " ")
	results, err := globalPluginRegistry.Search(query)
	if err != nil {
		fmt.Printf("  \u2717 Search failed: %v\n", err)
		return nil
	}

	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Println("│         Plugin Search Results             │")
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()

	if len(results) == 0 {
		fmt.Printf("  No plugins found matching %q\n", query)
		return nil
	}

	for _, mp := range results {
		fmt.Printf("  %-25s %-10s %s\n", mp.Name, mp.Version, mp.Description)
		if mp.Author != "" {
			fmt.Printf("    by %s\n", mp.Author)
		}
		if len(mp.Tags) > 0 {
			fmt.Printf("    tags: %s\n", strings.Join(mp.Tags, ", "))
		}
	}

	fmt.Println()
	fmt.Printf("  Found %d plugins\n", len(results))
	return nil
}

func pluginStatsHandler() error {
	stats := globalPluginRegistry.GetStats()

	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Println("│         Plugin Registry Statistics        │")
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()

	fmt.Printf("  Total plugins:     %d\n", stats.TotalPlugins)
	fmt.Printf("  Enabled:           %d\n", stats.EnabledPlugins)
	fmt.Printf("  Disabled:          %d\n", stats.TotalPlugins-stats.EnabledPlugins)
	fmt.Println()
	fmt.Printf("  Dynamic plugins:   %d\n", stats.DynamicPlugins)
	fmt.Printf("  Convention plugins: %d\n", stats.ConventionPlugins)
	fmt.Println()
	fmt.Printf("  Tool plugins:      %d\n", stats.ToolPlugins)
	fmt.Printf("  Hook plugins:      %d\n", stats.HookPlugins)

	return nil
}

func pluginType(p *plugins.Plugin) string {
	if len(p.Tools) > 0 {
		return "tool"
	}
	if len(p.Hooks) > 0 {
		return "hook"
	}
	if len(p.Commands) > 0 {
		return "command"
	}
	return "general"
}
