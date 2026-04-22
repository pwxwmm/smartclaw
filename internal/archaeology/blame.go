package archaeology

import (
	"github.com/instructkr/smartclaw/internal/git"
)

func ParseBlamePorcelain(output string, maxLines int) []git.BlameInfo {
	return git.ParseBlamePorcelain(output, maxLines)
}
