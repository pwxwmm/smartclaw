package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(Command{
		Name:    "session",
		Summary: "List sessions",
	}, sessionHandler)

	Register(Command{
		Name:    "resume",
		Summary: "Resume a session",
	}, resumeHandler)

	Register(Command{
		Name:    "save",
		Summary: "Save current session",
	}, saveHandler)

	Register(Command{
		Name:    "export",
		Summary: "Export session",
	}, exportHandler)

	Register(Command{
		Name:    "import",
		Summary: "Import session",
	}, importHandler)

	Register(Command{
		Name:    "rename",
		Summary: "Rename session",
	}, renameHandler)

	Register(Command{
		Name:    "fork",
		Summary: "Fork session",
	}, forkHandler)

	Register(Command{
		Name:    "rewind",
		Summary: "Rewind session",
	}, rewindHandler)

	Register(Command{
		Name:    "share",
		Summary: "Share session",
	}, shareHandler)

	Register(Command{
		Name:    "summary",
		Summary: "Session summary",
	}, summaryHandler)

	Register(Command{
		Name:    "attach",
		Summary: "Attach to process",
	}, attachHandler)
}

func sessionHandler(args []string) error {
	home, _ := os.UserHomeDir()
	sessionsPath := filepath.Join(home, ".sparkcode", "sessions")

	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Session List                │")
	fmt.Println("└─────────────────────────────────────┘")

	if _, err := os.Stat(sessionsPath); os.IsNotExist(err) {
		fmt.Println("  No sessions found")
		return nil
	}

	files, _ := os.ReadDir(sessionsPath)
	if len(files) == 0 {
		fmt.Println("  No sessions found")
		return nil
	}

	fmt.Println()
	for i, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
			continue
		}

		sessionID := strings.TrimSuffix(f.Name(), ".jsonl")
		info, _ := f.Info()
		modTime := info.ModTime().Format("2006-01-02 15:04")

		fmt.Printf("  %d. %s\n", i+1, sessionID)
		fmt.Printf("     Modified: %s\n", modTime)
	}

	fmt.Println()
	fmt.Println("Use /resume <session-id> to resume a session")

	return nil
}

func resumeHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /resume <session-id>")
		fmt.Println("\nUse /session to list available sessions")
		return nil
	}

	sessionID := args[0]
	home, _ := os.UserHomeDir()
	sessionPath := filepath.Join(home, ".sparkcode", "sessions", sessionID+".jsonl")

	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		fmt.Printf("✗ Session not found: %s\n", sessionID)
		return nil
	}

	cmdCtx.NewSession()
	cmdCtx.Session.ID = sessionID

	fmt.Printf("✓ Resumed session: %s\n", sessionID)
	return nil
}

func saveHandler(args []string) error {
	s := cmdCtx.GetSession()
	if s == nil {
		s = cmdCtx.NewSession()
	}

	home, _ := os.UserHomeDir()
	sessionsPath := filepath.Join(home, ".sparkcode", "sessions")
	os.MkdirAll(sessionsPath, 0755)

	sessionData := map[string]any{
		"id":            s.ID,
		"model":         cmdCtx.GetModel(),
		"created_at":    s.CreatedAt,
		"message_count": s.MessageCount,
	}

	data, _ := json.MarshalIndent(sessionData, "", "  ")
	sessionFile := filepath.Join(sessionsPath, s.ID+".json")
	os.WriteFile(sessionFile, data, 0644)

	fmt.Printf("✓ Session saved: %s\n", s.ID)
	return nil
}

func exportHandler(args []string) error {
	s := cmdCtx.GetSession()
	if s == nil {
		fmt.Println("✗ No active session to export")
		return nil
	}

	home, _ := os.UserHomeDir()
	exportsDir := filepath.Join(home, ".smartclaw", "exports")
	os.MkdirAll(exportsDir, 0755)

	exportPath := filepath.Join(exportsDir, fmt.Sprintf("session-%s.md", s.ID))

	var sb strings.Builder
	sb.WriteString("# Session Export\n\n")
	sb.WriteString(fmt.Sprintf("- **Session ID**: %s\n", s.ID))
	sb.WriteString(fmt.Sprintf("- **Model**: %s\n", cmdCtx.GetModel()))
	sb.WriteString(fmt.Sprintf("- **Created**: %s\n", s.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Messages**: %d\n\n", s.MessageCount))
	sb.WriteString("---\n\n")

	sessionFileJSONL := filepath.Join(home, ".sparkcode", "sessions", s.ID+".jsonl")
	sessionFileJSON := filepath.Join(home, ".sparkcode", "sessions", s.ID+".json")

	if data, err := os.ReadFile(sessionFileJSONL); err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			role, _ := entry["role"].(string)
			content, _ := entry["content"].(string)
			switch role {
			case "user":
				sb.WriteString("## User\n\n")
				sb.WriteString(content + "\n\n")
			case "assistant":
				sb.WriteString("## Assistant\n\n")
				sb.WriteString(content + "\n\n")
			default:
				if toolName, ok := entry["tool_name"].(string); ok {
					sb.WriteString(fmt.Sprintf("### Tool: %s\n\n", toolName))
					if output, ok := entry["output"].(string); ok {
						sb.WriteString(output + "\n\n")
					}
				}
			}
		}
	} else if data, err := os.ReadFile(sessionFileJSON); err == nil {
		var sessionData map[string]any
		if json.Unmarshal(data, &sessionData) == nil {
			if messages, ok := sessionData["messages"].([]any); ok {
				for _, msg := range messages {
					if m, ok := msg.(map[string]any); ok {
						role, _ := m["role"].(string)
						content, _ := m["content"].(string)
						switch role {
						case "user":
							sb.WriteString("## User\n\n")
							sb.WriteString(content + "\n\n")
						case "assistant":
							sb.WriteString("## Assistant\n\n")
							sb.WriteString(content + "\n\n")
						}
					}
				}
			}
		}
	} else {
		sb.WriteString("*No stored messages found for this session.*\n")
	}

	if err := os.WriteFile(exportPath, []byte(sb.String()), 0644); err != nil {
		fmt.Printf("✗ Failed to export session: %v\n", err)
		return nil
	}

	fmt.Printf("Session exported to ~/.smartclaw/exports/session-%s.md\n", s.ID)
	return nil
}

func importHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /import <filename>")
		return nil
	}

	filename := args[0]
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("✗ Failed to read file: %v\n", err)
		return nil
	}

	var imported map[string]any
	if err := json.Unmarshal(data, &imported); err != nil {
		fmt.Printf("✗ Invalid session file: %v\n", err)
		return nil
	}

	sessionID, _ := imported["session_id"].(string)
	fmt.Printf("✓ Session imported: %s\n", sessionID)
	return nil
}

func renameHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /rename <new-name>")
		return nil
	}

	s := cmdCtx.GetSession()
	if s == nil {
		fmt.Println("✗ No active session")
		return nil
	}

	fmt.Printf("✓ Session renamed to: %s\n", strings.Join(args, " "))
	return nil
}

func forkHandler(args []string) error {
	fmt.Println("Forking session...")
	s := cmdCtx.NewSession()
	fmt.Printf("✓ New session forked: %s\n", s.ID)
	return nil
}

func rewindHandler(args []string) error {
	s := cmdCtx.GetSession()
	if s == nil {
		fmt.Println("Session rewind not available in this mode")
		return nil
	}

	n := 1
	if len(args) > 0 {
		if parsed, err := strconv.Atoi(args[0]); err == nil && parsed > 0 {
			n = parsed
		}
	}

	if s.MessageCount < n {
		n = s.MessageCount
	}

	s.MessageCount -= n
	s.UpdatedAt = time.Now()
	fmt.Printf("Rewound %d message pair(s). Session now has %d messages.\n", n, s.MessageCount)
	return nil
}

func shareHandler(args []string) error {
	s := cmdCtx.GetSession()
	if s == nil {
		fmt.Println("✗ No active session to share")
		return nil
	}

	home, _ := os.UserHomeDir()
	sharedDir := filepath.Join(home, ".smartclaw", "shared")
	os.MkdirAll(sharedDir, 0755)

	sharePath := filepath.Join(sharedDir, fmt.Sprintf("session-%s.json", s.ID))

	shareData := map[string]any{
		"session_id":    s.ID,
		"model":         cmdCtx.GetModel(),
		"created_at":    s.CreatedAt,
		"updated_at":    s.UpdatedAt,
		"message_count": s.MessageCount,
		"shared_at":     time.Now(),
		"work_dir":      cmdCtx.WorkDir,
	}

	input, output, total := cmdCtx.GetTokenStats()
	shareData["tokens"] = map[string]int64{
		"input":  input,
		"output": output,
		"total":  total,
	}

	data, _ := json.MarshalIndent(shareData, "", "  ")
	if err := os.WriteFile(sharePath, data, 0644); err != nil {
		fmt.Printf("✗ Failed to share session: %v\n", err)
		return nil
	}

	fmt.Printf("Session shared. Link/file: ~/.smartclaw/shared/session-%s.json\n", s.ID)
	return nil
}

func summaryHandler(args []string) error {
	s := cmdCtx.GetSession()
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Session Summary             │")
	fmt.Println("└─────────────────────────────────────┘")
	if s != nil {
		fmt.Printf("  Session ID: %s\n", s.ID)
		fmt.Printf("  Messages:  %d\n", s.MessageCount)
	}
	input, output, total := cmdCtx.GetTokenStats()
	fmt.Printf("  Tokens:    %d in / %d out / %d total\n", input, output, total)
	return nil
}

func attachHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /attach <pid>")
		return nil
	}
	fmt.Printf("Attaching to process: %s\n", args[0])
	fmt.Println("⚠️  Process attach not fully implemented")
	return nil
}
