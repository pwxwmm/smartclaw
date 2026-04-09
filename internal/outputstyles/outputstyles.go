package outputstyles

import (
	"strings"
)

type OutputStyle struct {
	Name       string
	Prefix     string
	Suffix     string
	ColorCodes map[string]string
	UseEmoji   bool
}

var styles = map[string]OutputStyle{
	"default": {
		Name:       "default",
		Prefix:     "",
		Suffix:     "",
		UseEmoji:   false,
		ColorCodes: map[string]string{},
	},
	"minimal": {
		Name:     "minimal",
		Prefix:   "",
		Suffix:   "",
		UseEmoji: false,
	},
	"unicode": {
		Name:     "unicode",
		Prefix:   "",
		Suffix:   "",
		UseEmoji: true,
	},
	"ascii": {
		Name:     "ascii",
		Prefix:   "",
		Suffix:   "",
		UseEmoji: false,
	},
}

func GetStyle(name string) OutputStyle {
	if s, ok := styles[name]; ok {
		return s
	}
	return styles["default"]
}

func ListStyles() []string {
	names := make([]string, 0, len(styles))
	for name := range styles {
		names = append(names, name)
	}
	return names
}

func FormatMessage(style OutputStyle, msgType, message string) string {
	var sb strings.Builder
	sb.WriteString(style.Prefix)
	sb.WriteString(message)
	sb.WriteString(style.Suffix)
	return sb.String()
}

func GetEmoji(name string) string {
	emojis := map[string]string{
		"success":  "✓",
		"error":    "✗",
		"warning":  "⚠",
		"info":     "ℹ",
		"question": "?",
	}
	if e, ok := emojis[name]; ok {
		return e
	}
	return ""
}
