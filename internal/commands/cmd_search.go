package commands

import (
	"fmt"
	"strings"

	"github.com/instructkr/smartclaw/internal/store"
)

func init() {
	Register(Command{
		Name:    "search",
		Summary: "Search across all historical conversations",
	}, searchHandler)
}

func searchHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /search <query>")
		fmt.Println()
		fmt.Println("Search across all historical conversations using FTS5 full-text search.")
		fmt.Println("Results show session ID, role, timestamp, content excerpt, and relevance rank.")
		return nil
	}

	query := strings.Join(args, " ")

	s, err := store.NewStore()
	if err != nil {
		fmt.Printf("Search unavailable: %v\n", err)
		return nil
	}
	defer s.Close()

	results, err := s.SearchMessagesAdvanced(query, store.SearchOptions{Limit: 10})
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		return nil
	}

	if len(results) == 0 {
		fmt.Printf("No conversations found matching %q\n", query)
		return nil
	}

	fmt.Printf("Search results for %q (%d matches)\n", query, len(results))
	fmt.Println(strings.Repeat("─", 60))

	for i, r := range results {
		content := r.Content
		if len(content) > 150 {
			content = content[:150] + "..."
		}

		snippet := r.Snippet
		if snippet != "" && len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}

		ts := r.Timestamp.Format("2006-01-02 15:04")
		if r.Timestamp.IsZero() {
			ts = "unknown"
		}

		fmt.Printf("%d. [%s] %s in session %s\n", i+1, ts, r.Role, r.SessionID)
		if snippet != "" {
			fmt.Printf("   %s\n", snippet)
		} else {
			fmt.Printf("   %s\n", content)
		}
		fmt.Printf("   rank: %.4f\n", r.Rank)
		fmt.Println()
	}

	return nil
}
