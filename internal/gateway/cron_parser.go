package gateway

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// isScheduleDue determines whether a cron task should run at the given time.
// It supports two schedule formats:
//   - Interval: "every 5m", "every 1h", "every 30s", "every 1d"
//   - Standard 5-field cron: "*/5 * * * *" or "0 9 * * 1-5"
//   - Predefined: "@hourly", "@daily", "@weekly", "@monthly", "@yearly", "@every <duration>"
func isScheduleDue(schedule string, now time.Time, lastRun string) bool {
	if schedule == "" {
		return true
	}

	if strings.HasPrefix(schedule, "@") {
		return isPredefinedDue(schedule, now, lastRun)
	}

	if strings.HasPrefix(schedule, "every ") {
		return isIntervalDue(schedule, now, lastRun)
	}

	return isCronExpressionDue(schedule, now)
}

// isIntervalDue parses "every Ns", "every Nm", "every Nh", "every Nd" and checks
// whether enough time has elapsed since lastRun.
func isIntervalDue(schedule string, now time.Time, lastRun string) bool {
	interval, err := parseInterval(strings.TrimPrefix(schedule, "every "))
	if err != nil {
		return false
	}

	if lastRun == "" {
		return true
	}

	last, err := time.Parse(time.RFC3339, lastRun)
	if err != nil {
		return true
	}

	return now.Sub(last) >= interval
}

// parseInterval parses a simple duration string like "5m", "1h", "30s", "1d".
// Only supports whole numbers with a single unit suffix.
func parseInterval(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid interval: %q", s)
	}

	suffix := s[len(s)-1]
	numStr := s[:len(s)-1]

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid interval number: %q", numStr)
	}

	switch suffix {
	case 's':
		return time.Duration(num) * time.Second, nil
	case 'm':
		return time.Duration(num) * time.Minute, nil
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown interval unit: %c", suffix)
	}
}

// isPredefinedDue handles @hourly, @daily, @weekly, @monthly, @yearly, @every <duration>
func isPredefinedDue(schedule string, now time.Time, lastRun string) bool {
	lower := strings.ToLower(schedule)

	var interval time.Duration
	switch lower {
	case "@hourly":
		interval = 1 * time.Hour
	case "@daily":
		interval = 24 * time.Hour
	case "@weekly":
		interval = 7 * 24 * time.Hour
	case "@monthly":
		interval = 30 * 24 * time.Hour
	case "@yearly", "@annually":
		interval = 365 * 24 * time.Hour
	default:
		if strings.HasPrefix(lower, "@every ") {
			dur, err := parseInterval(strings.TrimPrefix(lower, "@every "))
			if err != nil {
				return false
			}
			interval = dur
		} else {
			return false
		}
	}

	if lastRun == "" {
		return true
	}

	last, err := time.Parse(time.RFC3339, lastRun)
	if err != nil {
		return true
	}

	return now.Sub(last) >= interval
}

// isCronExpressionDue evaluates a standard 5-field cron expression against the
// current time using robfig/cron for parsing.
func isCronExpressionDue(expr string, now time.Time) bool {
	schedule, err := cron.ParseStandard(expr)
	if err != nil {
		return false
	}
	next := schedule.Next(now.Truncate(time.Minute).Add(-time.Second))
	return next.Equal(now.Truncate(time.Minute))
}
