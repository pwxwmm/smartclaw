package server

import (
	"strings"
	"unicode"
)

const maxQueryLimit = 1000

func clampLimit(limit, defaultLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxQueryLimit {
		return maxQueryLimit
	}
	return limit
}

func isValidAlphanumeric(s string, maxLen int, extraChars string) bool {
	if len(s) > maxLen {
		return false
	}
	for _, c := range s {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && !strings.ContainsRune(extraChars, c) {
			return false
		}
	}
	return true
}
