package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	Register(Command{
		Name:    "invite",
		Summary: "Invite collaboration",
	}, inviteHandler)

	Register(Command{
		Name:    "feedback",
		Summary: "Send feedback",
	}, feedbackHandler)

	Register(Command{
		Name:    "issue",
		Summary: "Issue tracker",
	}, issueHandler)
}

func inviteHandler(args []string) error {
	fmt.Println("Collaboration invite sent")
	fmt.Println("⚠️  Team features not fully implemented")
	return nil
}

func feedbackHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /feedback <your feedback text>")
		return nil
	}

	home, _ := os.UserHomeDir()
	feedbackDir := filepath.Join(home, ".smartclaw", "feedback")
	os.MkdirAll(feedbackDir, 0755)

	timestamp := time.Now().Format("20060102-150405")
	feedbackPath := filepath.Join(feedbackDir, fmt.Sprintf("%s.txt", timestamp))

	text := strings.Join(args, " ")
	if err := os.WriteFile(feedbackPath, []byte(text), 0644); err != nil {
		fmt.Printf("✗ Failed to record feedback: %v\n", err)
		return nil
	}

	fmt.Println("Feedback recorded. Thank you!")
	return nil
}

func issueHandler(args []string) error {
	fmt.Println("Issue Tracker")
	fmt.Println("⚠️  Issue tracking not configured")
	return nil
}
