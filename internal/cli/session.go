package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/instructkr/smartclaw/internal/runtime"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage sessions",
	Long: `List, resume, or delete conversation sessions.

Sessions are stored in ~/.smartclaw/sessions/ by default.`,
}

var sessionListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all sessions",
	Run:     runSessionList,
}

var sessionShowCmd = &cobra.Command{
	Use:   "show <session-id>",
	Short: "Show session details",
	Args:  cobra.ExactArgs(1),
	Run:   runSessionShow,
}

var sessionDeleteCmd = &cobra.Command{
	Use:     "delete <session-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a session",
	Args:    cobra.ExactArgs(1),
	Run:     runSessionDelete,
}

var sessionCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Delete all sessions",
	Run:   runSessionClean,
}

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionShowCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionCleanCmd)
}

func runSessionList(cmd *cobra.Command, args []string) {
	sessionMgr, err := runtime.NewSessionManager()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	sessions, err := sessionMgr.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	fmt.Println("Sessions:")
	fmt.Println("---------")
	for _, session := range sessions {
		fmt.Printf("  %s  %s  %d messages\n",
			session.ID[:8],
			session.CreatedAt.Format("2006-01-02 15:04"),
			len(session.Messages))
	}
}

func runSessionShow(cmd *cobra.Command, args []string) {
	sessionID := args[0]

	sessionMgr, err := runtime.NewSessionManager()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	session, err := sessionMgr.Load(sessionID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Printf("Session: %s\n", session.ID)
	fmt.Printf("Created: %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Messages: %d\n", len(session.Messages))
	fmt.Println()
	fmt.Println("Messages:")
	for i, msg := range session.Messages {
		contentStr := fmt.Sprintf("%v", msg.Content)
		fmt.Printf("  %d. [%s] %s\n", i+1, msg.Role, truncate(contentStr, 50))
	}
}

func runSessionDelete(cmd *cobra.Command, args []string) {
	sessionID := args[0]

	sessionMgr, err := runtime.NewSessionManager()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if err := sessionMgr.Delete(sessionID); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted session: %s\n", sessionID)
}

func runSessionClean(cmd *cobra.Command, args []string) {
	sessionMgr, err := runtime.NewSessionManager()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	sessions, err := sessionMgr.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	for _, session := range sessions {
		sessionMgr.Delete(session.ID)
	}

	fmt.Printf("Deleted %d sessions\n", len(sessions))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
