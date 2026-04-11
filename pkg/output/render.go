package output

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Color codes for terminal output
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Bold    = "\033[1m"
)

// Renderer handles terminal output rendering
type Renderer struct {
	writer io.Writer
	color  bool
}

// NewRenderer creates a new renderer
func NewRenderer() *Renderer {
	return &Renderer{
		writer: os.Stdout,
		color:  true,
	}
}

// Print prints a message
func (r *Renderer) Print(msg string) {
	fmt.Fprint(r.writer, msg)
}

// Println prints a message with newline
func (r *Renderer) Println(msg string) {
	fmt.Fprintln(r.writer, msg)
}

// Printf prints a formatted message
func (r *Renderer) Printf(format string, args ...any) {
	fmt.Fprintf(r.writer, format, args...)
}

// Colorize applies color to text
func (r *Renderer) Colorize(color, text string) string {
	if !r.color {
		return text
	}
	return color + text + Reset
}

// Bold makes text bold
func (r *Renderer) Bold(text string) string {
	return r.Colorize(Bold, text)
}

// Green makes text green
func (r *Renderer) Green(text string) string {
	return r.Colorize(Green, text)
}

// Red makes text red
func (r *Renderer) Red(text string) string {
	return r.Colorize(Red, text)
}

// Yellow makes text yellow
func (r *Renderer) Yellow(text string) string {
	return r.Colorize(Yellow, text)
}

// Cyan makes text cyan
func (r *Renderer) Cyan(text string) string {
	return r.Colorize(Cyan, text)
}

// PrintHeader prints a header
func (r *Renderer) PrintHeader(title string) {
	width := 60
	padding := (width - len(title)) / 2

	r.Println("")
	r.Println(strings.Repeat("─", width))
	if padding > 0 {
		r.Printf("%s%s%s\n", strings.Repeat(" ", padding), r.Bold(title), Reset)
	} else {
		r.Println(r.Bold(title))
	}
	r.Println(strings.Repeat("─", width))
	r.Println("")
}

// PrintBox prints a message in a box
func (r *Renderer) PrintBox(title, content string) {
	lines := strings.Split(content, "\n")
	maxWidth := len(title)
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	r.Printf("┌─ %s ", r.Bold(title))
	r.Printf("%s┐\n", strings.Repeat("─", maxWidth-len(title)+1))

	for _, line := range lines {
		r.Printf("│ %s%s│\n", line, strings.Repeat(" ", maxWidth-len(line)+1))
	}

	r.Printf("└%s┘\n", strings.Repeat("─", maxWidth+2))
}

// PrintError prints an error message
func (r *Renderer) PrintError(msg string) {
	r.Printf("%sError:%s %s\n", r.Red("✗ "), Reset, msg)
}

// PrintSuccess prints a success message
func (r *Renderer) PrintSuccess(msg string) {
	r.Printf("%s%s\n", r.Green("✓ "), msg)
}

// PrintInfo prints an info message
func (r *Renderer) PrintInfo(msg string) {
	r.Printf("%s%s\n", r.Cyan("ℹ "), msg)
}

// PrintWarning prints a warning message
func (r *Renderer) PrintWarning(msg string) {
	r.Printf("%s%s\n", r.Yellow("⚠ "), msg)
}

// PrintToolCall prints a tool call
func (r *Renderer) PrintToolCall(name string, input any) {
	r.Printf("%s %s", r.Bold("Tool:"), r.Cyan(name))
	r.Print("\n")
}

// PrintToolResult prints a tool result
func (r *Renderer) PrintToolResult(result string, isError bool) {
	if isError {
		r.PrintError(result)
	} else {
		r.Println(result)
	}
}

// ClearLine clears the current line
func (r *Renderer) ClearLine() {
	r.Print("\r\033[K")
}

// PrintBanner prints the SmartCode banner
func (r *Renderer) PrintBanner() {
	banner := `
   ___      _           _      _____           _ 
  / __\___ | |__   __ _| |    / ____|         | |
 | |  / _ \| '_ \ / _' | |   | |     ___   __| | ___
 | | | (_) | |_) | (_| | |   | |    / _ \ / _' |/ _ \
 | |__\___/|_.__/ \__,_|_|   | |___| (_) | (_| |  __/
  \____/                     \_____\___/ \__,_|\___|
`
	if r.color {
		r.Println(r.Cyan(banner))
	} else {
		r.Println(banner)
	}
	r.Println(r.Bold("  SmartCode - Go Edition") + " 💡")
	r.Println(strings.Repeat("─", 50))
}
