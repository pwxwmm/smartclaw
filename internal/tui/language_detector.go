package tui

import (
	"regexp"
	"strings"
)

var codeBlockRegex = regexp.MustCompile("```\\s*\n([\\s\\S]*?)```")

var languagePatterns = []struct {
	lang    string
	pattern string
}{
	{"python", "(?s)^\\s*(import |from |def |class |if __name__|print\\(|#.*python)"},
	{"typescript", "(?s)^\\s*(interface |type |enum |namespace |import.*from|export.*from)"},
	{"javascript", "(?s)^\\s*(const |let |var |function |import |export |async |class )"},
	{"go", "(?s)^\\s*(package |func |type |struct |interface\\s*\\{|//.*)"},
	{"java", "(?s)^\\s*(package |import |public class|private |protected )"},
	{"bash", "(?s)^\\s*(#!/bin/|echo |export |if \\[|for |while |function )"},
	{"sql", "(?s)^\\s*(SELECT |INSERT |UPDATE |DELETE |CREATE |ALTER |DROP )"},
	{"json", "(?s)^\\s*\\{.*:\\s*"},
	{"yaml", "(?s)^\\s*[a-zA-Z_]+:"},
	{"html", "(?s)^\\s*<!DOCTYPE|<html|<head|<body|<div"},
	{"css", "(?s)^\\s*[.#]?[a-zA-Z][a-zA-Z0-9_-]*\\s*\\{"},
	{"rust", "(?s)^\\s*(fn |let |mut |pub |use |mod |struct |impl )"},
	{"c", "(?s)^\\s*(#include|int main|void |typedef |struct )"},
	{"cpp", "(?s)^\\s*(#include|namespace |class |template|std::)"},
}

func detectLanguage(code string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return ""
	}

	for _, lp := range languagePatterns {
		matched, _ := regexp.MatchString(lp.pattern, trimmed)
		if matched {
			return lp.lang
		}
	}

	return ""
}

func AddLanguageSpecifiers(markdown string) string {
	return codeBlockRegex.ReplaceAllStringFunc(markdown, func(match string) string {
		submatches := codeBlockRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		code := submatches[1]
		if code == "" {
			return match
		}

		lang := detectLanguage(code)
		if lang != "" {
			return "```" + lang + "\n" + code + "```"
		}

		return match
	})
}
