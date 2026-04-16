package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update SmartClaw to the latest version",
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/instructkr/smartclaw/releases/latest", nil)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("check for updates: %w", err)
	}
	defer resp.Body.Close()

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parse release info: %w", err)
	}

	if release.TagName == "" {
		return fmt.Errorf("no release found")
	}

	fmt.Printf("Current: %s\nLatest:  %s\n", Version, release.TagName)

	if release.TagName == Version {
		fmt.Println("Already up to date!")
		return nil
	}

	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()
	fmt.Println("To update, run one of:")
	fmt.Println("  macOS/Linux: curl -fsSL https://raw.githubusercontent.com/instructkr/smartclaw/main/scripts/install.sh | bash")
	fmt.Println("  Windows:     irm https://raw.githubusercontent.com/instructkr/smartclaw/main/scripts/install.ps1 | iex")
	fmt.Println("  Homebrew:    brew upgrade smartclaw")
	fmt.Printf("  Manual:      %s\n", release.HTMLURL)

	return nil
}
