package commands

import (
	"fmt"
	"strings"
)

func init() {
	Register(Command{
		Name:    "plan",
		Summary: "Manage persistent plans",
		Usage:   "plan <list|create|show|status|delete> [args]",
	}, planHandler)

	Register(Command{
		Name:    "think",
		Summary: "Think mode",
	}, thinkHandler)

	Register(Command{
		Name:    "deepthink",
		Summary: "Enable deep thinking",
	}, deepThinkHandler)

	Register(Command{
		Name:    "ultraplan",
		Summary: "Ultra planning",
	}, ultraplanHandler)

	Register(Command{
		Name:    "thinkback",
		Summary: "Think back",
	}, thinkbackHandler)

	Register(Command{
		Name:    "autonomous",
		Summary: "Execute autonomous task loop",
		Usage:   "autonomous <task> [--max-steps N] [--no-verify] [--create-pr]",
		Aliases: []string{"auto"},
	}, autonomousCmdHandler)

	Register(Command{
		Name:    "playbook",
		Summary: "Manage playbooks",
		Usage:   "playbook <list|execute|create> [name] [params]",
		Aliases: []string{"pb"},
	}, playbookCmdHandler)
}

func planHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: plan <list|create|show|status|delete> [args]")
		return nil
	}
	switch args[0] {
	case "list":
		fmt.Println("Listing plans from .smartclaw/plans/...")
	case "create":
		if len(args) < 2 {
			return fmt.Errorf("usage: plan create <title>")
		}
		fmt.Printf("Creating plan: %s\n", strings.Join(args[1:], " "))
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("usage: plan show <id>")
		}
		fmt.Printf("Showing plan: %s\n", args[1])
	case "status":
		if len(args) < 3 {
			return fmt.Errorf("usage: plan status <id> <draft|active|completed|abandoned>")
		}
		fmt.Printf("Setting plan %s to status %s\n", args[1], args[2])
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("usage: plan delete <id>")
		}
		fmt.Printf("Deleting plan: %s\n", args[1])
	default:
		return fmt.Errorf("unknown subcommand: %s", args[0])
	}
	return nil
}

func thinkHandler(args []string) error {
	fmt.Println("Think mode enabled")
	fmt.Println("  Claude will reason before responding")
	return nil
}

func deepThinkHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Deep thinking mode enabled")
		fmt.Println("  Uses extended thinking for complex problems")
		return nil
	}
	fmt.Println("✓ Deep think mode: " + args[0])
	return nil
}

func ultraplanHandler(args []string) error {
	fmt.Println("Ultra Plan mode")
	fmt.Println("  Advanced planning enabled")
	return nil
}

func thinkbackHandler(args []string) error {
	fmt.Println("Think Back mode")
	fmt.Println("  Review reasoning history")
	return nil
}

func autonomousCmdHandler(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: autonomous <task_description>")
	}
	taskDesc := strings.Join(args, " ")
	fmt.Printf("Starting autonomous task: %s\n", taskDesc)
	fmt.Println("Note: Full autonomous execution requires the runtime engine. Use the autonomous_execute tool for integrated execution.")
	return nil
}

func playbookCmdHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: playbook <list|execute|create> [name] [params]")
		return nil
	}
	switch args[0] {
	case "list":
		fmt.Println("Use the playbook_list tool to see available playbooks.")
	case "execute":
		if len(args) < 2 {
			return fmt.Errorf("usage: playbook execute <name> [key=value...]")
		}
		fmt.Printf("Execute playbook: %s (use playbook_execute tool)\n", args[1])
	case "create":
		if len(args) < 2 {
			return fmt.Errorf("usage: playbook create <name>")
		}
		fmt.Printf("Create playbook: %s (use playbook_create tool)\n", args[1])
	default:
		return fmt.Errorf("unknown subcommand: %s", args[0])
	}
	return nil
}
