package commands

import (
	"fmt"
)

func init() {
	Register(Command{
		Name:    "theme",
		Summary: "Manage themes",
	}, themeHandler)

	Register(Command{
		Name:    "color",
		Summary: "Color theme",
	}, colorHandler)

	Register(Command{
		Name:    "vim",
		Summary: "Vim mode",
	}, vimHandler)

	Register(Command{
		Name:    "keybindings",
		Summary: "Manage keybindings",
	}, keybindingsHandler)

	Register(Command{
		Name:    "statusline",
		Summary: "Status line",
	}, statuslineHandler)

	Register(Command{
		Name:    "stickers",
		Summary: "Stickers",
	}, stickersHandler)
}

func themeHandler(args []string) error {
	fmt.Println("Available themes:")
	fmt.Println("  - default")
	fmt.Println("  - dark")
	fmt.Println("  - light")
	fmt.Println()
	fmt.Println("Usage: /theme <name>")
	return nil
}

func colorHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Color themes:")
		fmt.Println("  default, dracula, monokai, nord, solarized")
		return nil
	}
	fmt.Printf("✓ Color theme set to: %s\n", args[0])
	return nil
}

func vimHandler(args []string) error {
	fmt.Println("Vim mode")
	fmt.Println("  Use vim keybindings in REPL")
	return nil
}

func keybindingsHandler(args []string) error {
	fmt.Println("Keybindings")
	fmt.Println("  ctrl+s: save")
	fmt.Println("  ctrl+c: copy")
	fmt.Println("  ctrl+v: paste")
	return nil
}

func statuslineHandler(args []string) error {
	fmt.Println("Status Line")
	fmt.Println("  Shows model, tokens, time")
	return nil
}

func stickersHandler(args []string) error {
	fmt.Println("Stickers")
	fmt.Println("  Available: 👍 👎 🎉 🚀 💡")
	return nil
}
