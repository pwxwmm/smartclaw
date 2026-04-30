package commands

import (
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(Command{
		Name:    "schedule",
		Summary: "Manage scheduled cron tasks",
		Usage:   "/schedule [list|add|delete|toggle|run]",
		Aliases: []string{"cron"},
	}, cmdScheduleHandler)
}

type CronTaskInfo struct {
	ID          string
	Schedule    string
	Instruction string
	Enabled     bool
	LastRunAt   time.Time
	CreatedAt   time.Time
}

type ScheduleParser interface {
	ParseNaturalLanguage(input string) (string, error)
}

var globalCronTrigger interface {
	ListTasks() ([]CronTaskInfo, error)
	CreateTask(schedule, instruction string) (string, error)
	DeleteTask(id string) error
	ToggleTask(id string) error
	RunTask(id string) error
}

var globalScheduleParser ScheduleParser

func SetGlobalCronTrigger(trigger interface {
	ListTasks() ([]CronTaskInfo, error)
	CreateTask(schedule, instruction string) (string, error)
	DeleteTask(id string) error
	ToggleTask(id string) error
	RunTask(id string) error
}) {
	globalCronTrigger = trigger
}

func SetGlobalScheduleParser(parser ScheduleParser) {
	globalScheduleParser = parser
}

func cmdScheduleHandler(args []string) error {
	if globalCronTrigger == nil {
		return scheduleNoManager()
	}

	if len(args) == 0 {
		return scheduleListHandler()
	}

	switch args[0] {
	case "list", "ls":
		return scheduleListHandler()
	case "add", "create", "new":
		return scheduleAddHandler(args[1:])
	case "delete", "del", "rm", "remove":
		return scheduleDeleteHandler(args[1:])
	case "toggle", "enable", "disable":
		return scheduleToggleHandler(args[1:])
	case "run", "trigger", "exec":
		return scheduleRunHandler(args[1:])
	default:
		fmt.Printf("Unknown schedule subcommand: %s\n", args[0])
		fmt.Println("Usage: /schedule [list|add|delete|toggle|run]")
		return nil
	}
}

func scheduleNoManager() error {
	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Println("│         Scheduled Tasks                   │")
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()
	fmt.Println("  Cron scheduler not initialized")
	fmt.Println()
	fmt.Println("  Subcommands:")
	fmt.Println("    /schedule list              List all cron tasks")
	fmt.Println("    /schedule add <sched> <cmd> Create a new cron task")
	fmt.Println("    /schedule delete <id>       Delete a task")
	fmt.Println("    /schedule toggle <id>       Toggle enabled/disabled")
	fmt.Println("    /schedule run <id>          Trigger immediate execution")
	return nil
}

func scheduleListHandler() error {
	tasks, err := globalCronTrigger.ListTasks()
	if err != nil {
		fmt.Printf("  \u2717 Error listing tasks: %v\n", err)
		return nil
	}

	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Println("│         Scheduled Tasks                   │")
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()

	if len(tasks) == 0 {
		fmt.Println("  No scheduled tasks")
		fmt.Println()
		fmt.Println("  Create one with:")
		fmt.Println("    /schedule add \"every day at 9am\" \"run daily report\"")
		return nil
	}

	fmt.Printf("  %-8s %-22s %-6s %-20s %-14s\n", "ID", "Schedule", "Stat", "Last Run", "Created")
	fmt.Printf("  %-8s %-22s %-6s %-20s %-14s\n", "--------", "----------------------", "------", "--------------------", "--------------")

	for _, t := range tasks {
		status := "\u2717 Off"
		if t.Enabled {
			status = "\u2713 On"
		}
		lastRun := "Never"
		if !t.LastRunAt.IsZero() {
			lastRun = t.LastRunAt.Format("Jan 02 15:04:05")
		}
		created := t.CreatedAt.Format("Jan 02 15:04")
		schedule := t.Schedule
		if len(schedule) > 20 {
			schedule = schedule[:17] + "..."
		}

		fmt.Printf("  %-8s %-22s %-6s %-20s %-14s\n",
			t.ID[:8], schedule, status, lastRun, created)
	}

	fmt.Println()
	fmt.Printf("  Total: %d tasks\n", len(tasks))
	return nil
}

func scheduleAddHandler(args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: /schedule add <schedule> <instruction>")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  /schedule add \"every day at 9am\" \"Generate daily report\"")
		fmt.Println("  /schedule add \"0 */6 * * *\" \"Check system health\"")
		fmt.Println("  /schedule add \"every monday at 8am\" \"Run weekly cleanup\"")
		return nil
	}

	schedule := args[0]
	instruction := strings.Join(args[1:], " ")

	parsedSchedule := schedule
	parts := strings.Fields(schedule)
	if len(parts) != 5 && globalScheduleParser != nil {
		parsed, err := globalScheduleParser.ParseNaturalLanguage(schedule)
		if err != nil {
			fmt.Printf("  \u2717 Failed to parse schedule: %v\n", err)
			fmt.Println("  Use standard cron format (5 fields) or natural language.")
			return nil
		}
		parsedSchedule = parsed
		fmt.Printf("  Parsed %q → %s\n", schedule, parsedSchedule)
	}

	id, err := globalCronTrigger.CreateTask(parsedSchedule, instruction)
	if err != nil {
		fmt.Printf("  \u2717 Failed to create task: %v\n", err)
		return nil
	}

	fmt.Printf("  \u2713 Task created: %s\n", id[:8])
	fmt.Printf("    Schedule:    %s\n", parsedSchedule)
	fmt.Printf("    Instruction: %s\n", truncateCron(instruction, 50))
	return nil
}

func scheduleDeleteHandler(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: /schedule delete <id>")
		return nil
	}

	id := args[0]
	if err := globalCronTrigger.DeleteTask(id); err != nil {
		fmt.Printf("  \u2717 Failed to delete task %s: %v\n", id, err)
		return nil
	}

	fmt.Printf("  \u2713 Task %s deleted\n", id)
	return nil
}

func scheduleToggleHandler(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: /schedule toggle <id>")
		return nil
	}

	id := args[0]
	if err := globalCronTrigger.ToggleTask(id); err != nil {
		fmt.Printf("  \u2717 Failed to toggle task %s: %v\n", id, err)
		return nil
	}

	fmt.Printf("  \u2713 Task %s toggled\n", id)
	return nil
}

func scheduleRunHandler(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: /schedule run <id>")
		return nil
	}

	id := args[0]
	if err := globalCronTrigger.RunTask(id); err != nil {
		fmt.Printf("  \u2717 Failed to trigger task %s: %v\n", id, err)
		return nil
	}

	fmt.Printf("  \u2713 Task %s triggered\n", id)
	return nil
}

func truncateCron(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
