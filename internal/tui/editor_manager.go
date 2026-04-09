package tui

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type EditorType string

const (
	EditorVim     EditorType = "vim"
	EditorNeovim  EditorType = "nvim"
	EditorNano    EditorType = "nano"
	EditorEmacs   EditorType = "emacs"
	EditorCode    EditorType = "code"
	EditorSublime EditorType = "subl"
	EditorAtom    EditorType = "atom"
	EditorDefault EditorType = ""
)

type EditorManager struct {
	editor      EditorType
	workDir     string
	tempDir     string
	lastFile    string
	autoCleanup bool
}

func NewEditorManager(workDir string) *EditorManager {
	tempDir := filepath.Join(workDir, ".smartclaw_tmp")
	os.MkdirAll(tempDir, 0755)

	return &EditorManager{
		editor:      EditorDefault,
		workDir:     workDir,
		tempDir:     tempDir,
		autoCleanup: true,
	}
}

func (em *EditorManager) DetectEditor() EditorType {
	// Check $EDITOR environment variable
	if editor := os.Getenv("EDITOR"); editor != "" {
		switch {
		case strings.Contains(editor, "vim"):
			return EditorVim
		case strings.Contains(editor, "nvim"):
			return EditorNeovim
		case strings.Contains(editor, "nano"):
			return EditorNano
		case strings.Contains(editor, "emacs"):
			return EditorEmacs
		case strings.Contains(editor, "code"):
			return EditorCode
		}
	}

	// Check $VISUAL environment variable
	if visual := os.Getenv("VISUAL"); visual != "" {
		switch {
		case strings.Contains(visual, "vim"):
			return EditorVim
		case strings.Contains(visual, "nvim"):
			return EditorNeovim
		case strings.Contains(visual, "code"):
			return EditorCode
		}
	}

	// Check common editors in PATH
	editors := []struct {
		name   EditorType
		binary string
	}{
		{EditorNeovim, "nvim"},
		{EditorVim, "vim"},
		{EditorNano, "nano"},
		{EditorCode, "code"},
		{EditorEmacs, "emacs"},
	}

	for _, e := range editors {
		if _, err := exec.LookPath(e.binary); err == nil {
			return e.name
		}
	}

	return EditorVim // Default fallback
}

func (em *EditorManager) SetEditor(editor EditorType) {
	em.editor = editor
}

func (em *EditorManager) GetEditor() EditorType {
	if em.editor == EditorDefault {
		return em.DetectEditor()
	}
	return em.editor
}

func (em *EditorManager) GetEditorBinary() string {
	switch em.GetEditor() {
	case EditorNeovim:
		return "nvim"
	case EditorVim:
		return "vim"
	case EditorNano:
		return "nano"
	case EditorEmacs:
		return "emacs"
	case EditorCode:
		return "code"
	case EditorSublime:
		return "subl"
	case EditorAtom:
		return "atom"
	default:
		return "vim"
	}
}

func (em *EditorManager) CreateTempFile(content string, ext string) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("editor_%s%s", timestamp, ext)
	filePath := filepath.Join(em.tempDir, filename)

	if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	em.lastFile = filePath
	return filePath, nil
}

func (em *EditorManager) ReadFile(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(content), nil
}

func (em *EditorManager) Edit(filePath string) error {
	editor := em.GetEditorBinary()

	cmd := exec.Command(editor, filePath)
	cmd.Dir = em.workDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	return nil
}

func (em *EditorManager) EditContent(content string, ext string) (string, error) {
	// Create temp file
	filePath, err := em.CreateTempFile(content, ext)
	if err != nil {
		return "", err
	}

	// Open in editor
	if err := em.Edit(filePath); err != nil {
		return "", err
	}

	// Read edited content
	newContent, err := em.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Cleanup
	if em.autoCleanup {
		os.Remove(filePath)
	}

	return newContent, nil
}

func (em *EditorManager) EditMultiline() (string, error) {
	return em.EditContent("", ".txt")
}

func (em *EditorManager) EditFile(filePath string) (string, error) {
	// Make path absolute if relative
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(em.workDir, filePath)
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", filePath)
	}

	// Open in editor
	if err := em.Edit(filePath); err != nil {
		return "", err
	}

	// Read content
	return em.ReadFile(filePath)
}

func (em *EditorManager) Cleanup() {
	if em.autoCleanup {
		files, _ := filepath.Glob(filepath.Join(em.tempDir, "editor_*"))
		for _, file := range files {
			os.Remove(file)
		}
	}
}

func (em *EditorManager) FormatEditorInfo(editor EditorType) string {
	var sb strings.Builder

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("📝 Editor Configuration\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	sb.WriteString(fmt.Sprintf("Current Editor: %s\n", em.GetEditorBinary()))

	envEditor := os.Getenv("EDITOR")
	envVisual := os.Getenv("VISUAL")

	if envEditor != "" {
		sb.WriteString(fmt.Sprintf("$EDITOR: %s\n", envEditor))
	}
	if envVisual != "" {
		sb.WriteString(fmt.Sprintf("$VISUAL: %s\n", envVisual))
	}

	sb.WriteString("\n💡 Set Editor:\n")
	sb.WriteString("  /editor vim     - Use Vim\n")
	sb.WriteString("  /editor nvim    - Use Neovim\n")
	sb.WriteString("  /editor nano    - Use Nano\n")
	sb.WriteString("  /editor code    - Use VS Code\n")
	sb.WriteString("  /editor emacs   - Use Emacs\n")

	sb.WriteString("\n📝 Edit Commands:\n")
	sb.WriteString("  /edit           - Open editor for new content\n")
	sb.WriteString("  /edit <file>    - Edit existing file\n")
	sb.WriteString("  /multilines     - Edit multiline input\n")

	return sb.String()
}

func (em *EditorManager) ListAvailableEditors() string {
	var sb strings.Builder

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("📝 Available Editors\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	editors := []struct {
		name   EditorType
		binary string
		desc   string
	}{
		{EditorNeovim, "nvim", "Hyperextensible Vim-based editor"},
		{EditorVim, "vim", "Highly configurable text editor"},
		{EditorNano, "nano", "Simple, user-friendly editor"},
		{EditorCode, "code", "Visual Studio Code"},
		{EditorEmacs, "emacs", "Extensible, customizable editor"},
		{EditorSublime, "subl", "Sublime Text"},
		{EditorAtom, "atom", "Atom Editor"},
	}

	for _, e := range editors {
		_, err := exec.LookPath(e.binary)
		available := "✓"
		if err != nil {
			available = "✗"
		}
		sb.WriteString(fmt.Sprintf("%s %-10s %s\n", available, e.name, e.desc))
	}

	sb.WriteString("\n💡 Tip: Install an editor or set $EDITOR environment variable\n")

	return sb.String()
}
