package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type FileReference struct {
	Path      string
	StartLine int
	EndLine   int
	Content   string
	Error     string
}

func ParseFileReferences(input string, workDir string) ([]FileReference, string) {
	re := regexp.MustCompile(`@([^\s]+)`)
	matches := re.FindAllStringSubmatch(input, -1)

	var refs []FileReference
	processedInput := input

	for _, match := range matches {
		fullMatch := match[0]
		fileRef := match[1]

		ref := ParseFileReference(fileRef, workDir)
		refs = append(refs, ref)

		if ref.Error != "" {
			processedInput = strings.Replace(processedInput, fullMatch,
				fmt.Sprintf("[Error: %s - %s]", ref.Path, ref.Error), 1)
		} else {
			processedInput = strings.Replace(processedInput, fullMatch,
				fmt.Sprintf("\n📄 File: %s\n```\n%s\n```\n", ref.Path, ref.Content), 1)
		}
	}

	return refs, processedInput
}

func ParseFileReference(ref string, workDir string) FileReference {
	result := FileReference{
		Path:      ref,
		StartLine: 0,
		EndLine:   -1,
	}

	lineRangePattern := regexp.MustCompile(`^(.+):(\d+)(?:-(\d+))?$`)
	if matches := lineRangePattern.FindStringSubmatch(ref); matches != nil {
		result.Path = matches[1]
		if startLine, err := strconv.Atoi(matches[2]); err == nil {
			result.StartLine = startLine
			if matches[3] != "" {
				if endLine, err := strconv.Atoi(matches[3]); err == nil {
					result.EndLine = endLine
				}
			} else {
				result.EndLine = result.StartLine
			}
		}
	}

	if !filepath.IsAbs(result.Path) {
		if strings.HasPrefix(result.Path, "./") || strings.HasPrefix(result.Path, "../") {
			result.Path = filepath.Join(workDir, result.Path)
		} else {
			result.Path = filepath.Join(workDir, result.Path)
		}
	}

	result.Path = filepath.Clean(result.Path)

	content, err := ReadFileContent(result.Path, result.StartLine, result.EndLine)
	if err != nil {
		result.Error = err.Error()
	} else {
		result.Content = content
	}

	return result
}

func ReadFileContent(path string, startLine, endLine int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot open file: %v", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("cannot get file info: %v", err)
	}

	if info.Size() > 10*1024*1024 {
		return "", fmt.Errorf("file too large (max 10MB)")
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	lineNum := 1

	for scanner.Scan() {
		if startLine == 0 || (lineNum >= startLine && (endLine == -1 || lineNum <= endLine)) {
			lines = append(lines, scanner.Text())
		}
		lineNum++

		if endLine > 0 && lineNum > endLine {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %v", err)
	}

	return strings.Join(lines, "\n"), nil
}

func DetectFileReferences(input string) bool {
	return strings.Contains(input, "@")
}

func GetFilesInDirectory(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			files = append(files, entry.Name()+"/")
		} else {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func FilterCompletions(files []string, prefix string) []string {
	var result []string
	for _, f := range files {
		if strings.HasPrefix(strings.ToLower(f), strings.ToLower(prefix)) {
			result = append(result, f)
		}
	}
	return result
}

func ExtractFilePrefix(input string) (string, string) {
	lastAtIndex := strings.LastIndex(input, "@")
	if lastAtIndex == -1 {
		return "", ""
	}

	afterAt := input[lastAtIndex+1:]

	spaceIndex := strings.IndexAny(afterAt, " \t\n")
	if spaceIndex != -1 {
		afterAt = afterAt[:spaceIndex]
	}

	dir := filepath.Dir(afterAt)
	if dir == "." {
		dir = ""
	}

	file := filepath.Base(afterAt)
	if afterAt == "" || afterAt == "." || strings.HasSuffix(afterAt, "/") {
		file = ""
	}

	return dir, file
}
