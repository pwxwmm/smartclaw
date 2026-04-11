package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type REPL struct {
	prompt      string
	history     []string
	historyFile string
	running     bool
}

func NewREPL(prompt string) *REPL {
	home, _ := os.UserHomeDir()
	historyFile := home + "/.smartclaw/history"

	return &REPL{
		prompt:      prompt,
		history:     []string{},
		historyFile: historyFile,
		running:     false,
	}
}

func (r *REPL) Start(ctx context.Context) error {
	r.running = true
	defer func() { r.running = false }()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-sigChan:
			fmt.Println("\nInterrupted. Type 'exit' to quit.")
		case <-ctx.Done():
			r.running = false
		}
	}()

	reader := bufio.NewReader(os.Stdin)

	for r.running {
		fmt.Print(r.prompt + " ")

		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		r.history = append(r.history, input)
		r.saveHistory()

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return nil
		}
	}

	return nil
}

func (r *REPL) saveHistory() error {
	file, err := os.OpenFile(r.historyFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, cmd := range r.history {
		fmt.Fprintln(file, cmd)
	}

	return nil
}

func (r *REPL) loadHistory() error {
	file, err := os.Open(r.historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		r.history = append(r.history, scanner.Text())
	}

	return scanner.Err()
}

func (r *REPL) GetHistory() []string {
	return r.history
}

func (r *REPL) ClearHistory() error {
	r.history = []string{}
	return os.Remove(r.historyFile)
}

type PromptInput struct {
	reader      *bufio.Reader
	promptColor string
}

func NewPromptInput() *PromptInput {
	return &PromptInput{
		reader:      bufio.NewReader(os.Stdin),
		promptColor: "\033[36m",
	}
}

func (p *PromptInput) ReadLine(prompt string) (string, error) {
	fmt.Print(prompt + " ")
	input, err := p.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func (p *PromptInput) ReadPassword(prompt string) (string, error) {
	fmt.Print(prompt + " ")

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	var password []byte
	buf := make([]byte, 1)

	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}

		if buf[0] == '\n' || buf[0] == '\r' {
			break
		}

		if buf[0] == 127 || buf[0] == 8 {
			if len(password) > 0 {
				password = password[:len(password)-1]
			}
		} else {
			password = append(password, buf[0])
		}
	}

	fmt.Println()
	return string(password), nil
}

func (p *PromptInput) Confirm(prompt string, defaultYes bool) (bool, error) {
	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}

	input, err := p.ReadLine(fmt.Sprintf("%s [%s]", prompt, hint))
	if err != nil {
		return false, err
	}

	input = strings.ToLower(input)
	if input == "" {
		return defaultYes, nil
	}

	return input == "y" || input == "yes", nil
}

func (p *PromptInput) Select(prompt string, options []string) (int, error) {
	fmt.Println(prompt)
	for i, opt := range options {
		fmt.Printf("  %d. %s\n", i+1, opt)
	}

	for {
		input, err := p.ReadLine("Select (number): ")
		if err != nil {
			return -1, err
		}

		var choice int
		if _, err := fmt.Sscanf(input, "%d", &choice); err == nil {
			if choice >= 1 && choice <= len(options) {
				return choice - 1, nil
			}
		}

		fmt.Println("Invalid selection. Please try again.")
	}
}

type OutputWriter struct {
	useColor bool
	useJSON  bool
}

func NewOutputWriter(useColor, useJSON bool) *OutputWriter {
	return &OutputWriter{
		useColor: useColor,
		useJSON:  useJSON,
	}
}

func (w *OutputWriter) Write(data any) error {
	if w.useJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(data)
	}

	switch v := data.(type) {
	case string:
		fmt.Println(v)
	case []byte:
		fmt.Println(string(v))
	case map[string]any:
		for k, val := range v {
			fmt.Printf("%s: %v\n", k, val)
		}
	default:
		fmt.Printf("%v\n", data)
	}

	return nil
}

func (w *OutputWriter) WriteError(err error) {
	if w.useColor {
		fmt.Printf("\033[31mError: %v\033[0m\n", err)
	} else {
		fmt.Printf("Error: %v\n", err)
	}
}

func (w *OutputWriter) WriteSuccess(msg string) {
	if w.useColor {
		fmt.Printf("\033[32m%s\033[0m\n", msg)
	} else {
		fmt.Println(msg)
	}
}

func (w *OutputWriter) WriteWarning(msg string) {
	if w.useColor {
		fmt.Printf("\033[33mWarning: %s\033[0m\n", msg)
	} else {
		fmt.Printf("Warning: %s\n", msg)
	}
}

func (w *OutputWriter) WriteInfo(msg string) {
	if w.useColor {
		fmt.Printf("\033[36m%s\033[0m\n", msg)
	} else {
		fmt.Println(msg)
	}
}
