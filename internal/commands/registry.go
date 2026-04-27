package commands

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Command struct {
	Name        string
	Summary     string
	Usage       string
	Aliases     []string
	Description string
}

type CommandHandler func(args []string) error

type CommandRegistry struct {
	commands map[string]Command
	handlers map[string]CommandHandler
}

func NewRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]Command),
		handlers: make(map[string]CommandHandler),
	}
}

func (r *CommandRegistry) Register(cmd Command, handler CommandHandler) {
	r.commands[cmd.Name] = cmd
	r.handlers[cmd.Name] = handler
	for _, alias := range cmd.Aliases {
		r.commands[alias] = cmd
		r.handlers[alias] = handler
	}
}

func (r *CommandRegistry) Get(name string) Command {
	return r.commands[name]
}

func (r *CommandRegistry) Execute(name string, args []string) error {
	handler, exists := r.handlers[name]
	if !exists {
		return fmt.Errorf("unknown command: /%s", name)
	}
	return handler(args)
}

func (r *CommandRegistry) Has(name string) bool {
	_, exists := r.handlers[name]
	return exists
}

func (r *CommandRegistry) All() []Command {
	seen := make(map[string]bool)
	var result []Command
	for _, cmd := range r.commands {
		if !seen[cmd.Name] {
			seen[cmd.Name] = true
			result = append(result, cmd)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (r *CommandRegistry) Help() string {
	var lines []string
	lines = append(lines, "Available commands:")

	commands := r.All()
	for _, cmd := range commands {
		lines = append(lines, fmt.Sprintf("  /%-15s %s", cmd.Name, cmd.Summary))
	}

	return strings.Join(lines, "\n")
}

var defaultRegistry *CommandRegistry
var defaultRegistryOnce sync.Once

func ensureDefaultRegistry() {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewRegistry()
	})
}

func GetRegistry() *CommandRegistry {
	ensureDefaultRegistry()
	return defaultRegistry
}

func Register(cmd Command, handler CommandHandler) {
	ensureDefaultRegistry()
	defaultRegistry.Register(cmd, handler)
}

func Execute(name string, args []string) error {
	ensureDefaultRegistry()
	return defaultRegistry.Execute(name, args)
}
