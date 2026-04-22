package archaeology

import (
	"github.com/instructkr/smartclaw/internal/git"
)

func ParseFileLog(output string) []git.FileLogEntry {
	return git.ParseFileLog(output)
}
