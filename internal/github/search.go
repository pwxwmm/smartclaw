package github

import (
	"context"
	"fmt"
	"strings"
)

type CodeResult struct {
	Path    string
	Repo    string
	HTMLURL string
}

func (c *Client) SearchCode(ctx context.Context, query string) ([]*CodeResult, error) {
	out, err := c.runGH(ctx, "api", "search/code",
		"-q", ".items[] | {path: .path, repo: .repository.full_name, url: .html_url}",
		"-f", "q="+query)
	if err != nil {
		return nil, fmt.Errorf("search code: %w", err)
	}

	return parseCodeResults(out), nil
}

func parseCodeResults(output string) []*CodeResult {
	var results []*CodeResult

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		cr := parseCodeResultLine(line)
		if cr != nil {
			results = append(results, cr)
		}
	}

	return results
}

func parseCodeResultLine(line string) *CodeResult {
	if len(line) < 2 || line[0] != '{' {
		return nil
	}

	var path, repo, url string
	remaining := line[1 : len(line)-1]

	for _, field := range strings.Split(remaining, ",") {
		kv := strings.SplitN(strings.TrimSpace(field), ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.Trim(strings.TrimSpace(kv[0]), `"`)
		val := strings.Trim(strings.TrimSpace(kv[1]), `"`)

		switch key {
		case "path":
			path = val
		case "repo":
			repo = val
		case "url":
			url = val
		}
	}

	if path == "" && repo == "" && url == "" {
		return nil
	}

	return &CodeResult{Path: path, Repo: repo, HTMLURL: url}
}
