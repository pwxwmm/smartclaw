package commands

import (
	"fmt"
	"os/exec"
	"strings"
)

func init() {
	Register(Command{
		Name:    "git-status",
		Summary: "Show git status",
		Aliases: []string{"gs"},
	}, gitStatusHandler)

	Register(Command{
		Name:    "git-diff",
		Summary: "Show git diff",
		Aliases: []string{"gd"},
	}, gitDiffHandler)

	Register(Command{
		Name:    "git-commit",
		Summary: "Commit changes",
		Aliases: []string{"gc"},
	}, gitCommitHandler)

	Register(Command{
		Name:    "git-branch",
		Summary: "List branches",
		Aliases: []string{"gb"},
	}, gitBranchHandler)

	Register(Command{
		Name:    "git-log",
		Summary: "Show git log",
		Aliases: []string{"gl"},
	}, gitLogHandler)

	Register(Command{
		Name:    "diff",
		Summary: "Show git diff",
	}, diffHandler)

	Register(Command{
		Name:    "commit",
		Summary: "Git commit",
	}, commitHandler)
}

func gitStatusHandler(args []string) error {
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Git error: %v\n", err)
		return nil
	}

	if len(output) == 0 {
		fmt.Println("✓ Working tree clean")
	} else {
		fmt.Println(string(output))
	}
	return nil
}

func gitDiffHandler(args []string) error {
	cmd := exec.Command("git", "diff")
	if len(args) > 0 {
		cmd.Args = append(cmd.Args, args...)
	}
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Git error: %v\n", err)
		return nil
	}

	fmt.Println(string(output))
	return nil
}

func gitCommitHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /git-commit <message>")
		return nil
	}

	message := strings.Join(args, " ")
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Git error: %v\n%s\n", err, string(output))
		return nil
	}

	fmt.Printf("✓ Committed: %s\n", message)
	return nil
}

func gitBranchHandler(args []string) error {
	cmd := exec.Command("git", "branch", "-a")
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Git error: %v\n", err)
		return nil
	}

	fmt.Println(string(output))
	return nil
}

func gitLogHandler(args []string) error {
	cmd := exec.Command("git", "log", "--oneline", "-20")
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Git error: %v\n", err)
		return nil
	}

	fmt.Println(string(output))
	return nil
}

func diffHandler(args []string) error {
	return gitDiffHandler(args)
}

func commitHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /commit <message>")
		return nil
	}
	msg := strings.Join(args, " ")
	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Error: %v\n%s\n", err, string(output))
		return nil
	}
	fmt.Printf("✓ Committed: %s\n", msg)
	return nil
}
