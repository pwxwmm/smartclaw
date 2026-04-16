package diffengine

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// VerifyResult holds the outcome of verifying a file after diff application.
type VerifyResult struct {
	FilePath string
	Valid    bool
	Issues   []string
}

// VerifyGoFile verifies a Go file by parsing it for syntax errors,
// checking indentation consistency, and checking for unmatched braces/parens.
func VerifyGoFile(filePath string) *VerifyResult {
	result := &VerifyResult{FilePath: filePath, Valid: true}

	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("read error: %v", err))
		return result
	}

	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, filePath, content, parser.AllErrors|parser.ParseComments)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("syntax error: %v", err))
	}

	checkIndentation(string(content), result)
	checkBraces(string(content), result)

	return result
}

// VerifyGenericFile does basic sanity checks on a non-Go file.
func VerifyGenericFile(filePath string) *VerifyResult {
	result := &VerifyResult{FilePath: filePath, Valid: true}

	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("read error: %v", err))
		return result
	}

	if len(content) == 0 {
		result.Valid = false
		result.Issues = append(result.Issues, "file is empty")
		return result
	}

	if isBinaryContent(content) {
		result.Valid = false
		result.Issues = append(result.Issues, "file appears to contain binary content")
	}

	return result
}

// Rollback restores a file to its original content.
func Rollback(filePath string, originalContent string) error {
	if err := os.WriteFile(filePath, []byte(originalContent), 0644); err != nil {
		return fmt.Errorf("rollback failed for %s: %w", filePath, err)
	}
	return nil
}

// VerifyFile picks the appropriate verification based on file extension.
func VerifyFile(filePath string) *VerifyResult {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return VerifyGoFile(filePath)
	default:
		return VerifyGenericFile(filePath)
	}
}

func checkIndentation(content string, result *VerifyResult) {
	lines := strings.Split(content, "\n")
	tabsMixed := false
	spacesMixed := false

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		indentEnd := indentEndOfLine(line)
		hasTab := false
		if indentEnd > 0 {
			hasTab = strings.Contains(line[:indentEnd], "\t")
		}
		indent := leadingWhitespace(line)
		hasSpace := strings.Contains(indent, " ") && len(indent) > 0

		if hasTab && hasSpace {
			tabsMixed = true
			spacesMixed = true
		} else if hasTab {
			tabsMixed = true
		} else if hasSpace {
			spacesMixed = true
		}
	}

	if tabsMixed && spacesMixed {
		result.Issues = append(result.Issues, "mixed tabs and spaces in indentation")
	}
}

func leadingWhitespace(line string) string {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[:i]
}

func indentEndOfLine(line string) int {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i
}

func checkBraces(content string, result *VerifyResult) {
	braceDepth := 0
	parenDepth := 0
	bracketDepth := 0
	inString := false
	stringChar := byte(0)
	inComment := false
	inLineComment := false

	for i := 0; i < len(content); i++ {
		c := content[i]

		if inLineComment {
			if c == '\n' {
				inLineComment = false
			}
			continue
		}

		if inComment {
			if i+1 < len(content) && c == '*' && content[i+1] == '/' {
				inComment = false
				i++
			}
			continue
		}

		if inString {
			if c == '\\' {
				i++
				continue
			}
			if c == stringChar {
				inString = false
			}
			continue
		}

		if c == '/' && i+1 < len(content) {
			if content[i+1] == '/' {
				inLineComment = true
				i++
				continue
			}
			if content[i+1] == '*' {
				inComment = true
				i++
				continue
			}
		}

		if c == '"' || c == '\'' || c == '`' {
			inString = true
			stringChar = c
			continue
		}

		switch c {
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		}
	}

	if braceDepth != 0 {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("unmatched braces: depth=%d", braceDepth))
	}
	if parenDepth != 0 {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("unmatched parens: depth=%d", parenDepth))
	}
	if bracketDepth != 0 {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("unmatched brackets: depth=%d", bracketDepth))
	}
}

func isBinaryContent(content []byte) bool {
	nonPrintable := 0
	checkLen := len(content)
	if checkLen > 8192 {
		checkLen = 8192
	}

	for i := 0; i < checkLen; i++ {
		c := content[i]
		if c == 0 {
			return true
		}
		if c < 32 && c != '\n' && c != '\r' && c != '\t' {
			nonPrintable++
		}
	}

	if checkLen > 0 && nonPrintable*100/checkLen > 30 {
		return true
	}
	return false
}
