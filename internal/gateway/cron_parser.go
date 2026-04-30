package gateway

import (
	"fmt"
	"regexp"
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

// ParseNaturalLanguage converts natural language schedule descriptions into
// standard 5-field cron expressions using pure regex/rule-based parsing (no LLM).
//
// Supported patterns:
//   - "every day at 9am"      → "0 9 * * *"
//   - "every weekday at 9:30" → "30 9 * * 1-5"
//   - "every monday at noon"  → "0 12 * * 1"
//   - "every 30 minutes"      → "*/30 * * * *"
//   - "hourly"                → "0 * * * *"
//   - "daily at 3pm"          → "0 15 * * *"
//   - "every 6 hours"         → "0 */6 * * *"
//   - "weekdays at 8:30am"    → "30 8 * * 1-5"
//   - "at midnight"           → "0 0 * * *"
//   - "twice a day"           → "0 0,12 * * *"
//   - "every weekend at 10am" → "0 10 * * 0,6"
func ParseNaturalLanguage(input string) (string, error) {
	s := strings.TrimSpace(strings.ToLower(input))

	switch s {
	case "hourly":
		return "0 * * * *", nil
	case "daily", "every day":
		return "0 0 * * *", nil
	case "weekly", "every week":
		return "0 0 * * 0", nil
	case "monthly", "every month":
		return "0 0 1 * *", nil
	case "yearly", "annually", "every year":
		return "0 0 1 1 *", nil
	case "midnight", "at midnight":
		return "0 0 * * *", nil
	case "noon", "at noon":
		return "0 12 * * *", nil
	case "twice a day":
		return "0 0,12 * * *", nil
	case "twice an hour":
		return "0,30 * * * *", nil
	case "weekdays", "every weekday":
		return "0 0 * * 1-5", nil
	case "weekends", "every weekend":
		return "0 0 * * 0,6", nil
	}

	if m := regexp.MustCompile(`^every\s+(\d+)\s+minutes?$`).FindStringSubmatch(s); m != nil {
		return fmt.Sprintf("*/%s * * * *", m[1]), nil
	}

	if m := regexp.MustCompile(`^every\s+(\d+)\s+hours?$`).FindStringSubmatch(s); m != nil {
		return fmt.Sprintf("0 */%s * * *", m[1]), nil
	}

	if m := regexp.MustCompile(`^every\s+(\d+)\s+days?$`).FindStringSubmatch(s); m != nil {
		return fmt.Sprintf("0 0 */%s * *", m[1]), nil
	}

	if m := regexp.MustCompile(`^(?:every\s+)?weekdays?\s+at\s+(.+)$`).FindStringSubmatch(s); m != nil {
		hour, min, err := parseTimeOfDay(m[1])
		if err != nil {
			return "", fmt.Errorf("parse time: %w", err)
		}
		return fmt.Sprintf("%d %d * * 1-5", min, hour), nil
	}

	if m := regexp.MustCompile(`^(?:every\s+)?weekends?\s+at\s+(.+)$`).FindStringSubmatch(s); m != nil {
		hour, min, err := parseTimeOfDay(m[1])
		if err != nil {
			return "", fmt.Errorf("parse time: %w", err)
		}
		return fmt.Sprintf("%d %d * * 0,6", min, hour), nil
	}

	dayMap := map[string]string{
		"sunday": "0", "sun": "0",
		"monday": "1", "mon": "1",
		"tuesday": "2", "tue": "2",
		"wednesday": "3", "wed": "3",
		"thursday": "4", "thu": "4",
		"friday": "5", "fri": "5",
		"saturday": "6", "sat": "6",
	}

	dayKeys := make([]string, 0, len(dayMap))
	for k := range dayMap {
		dayKeys = append(dayKeys, k)
	}
	dayPattern := `(?:every\s+)?(` + strings.Join(dayKeys, `|`) + `)s?\s+at\s+(.+)`

	if m := regexp.MustCompile("^"+dayPattern+"$").FindStringSubmatch(s); m != nil {
		dayNum, ok := dayMap[m[1]]
		if !ok {
			return "", fmt.Errorf("unknown day: %s", m[1])
		}
		hour, min, err := parseTimeOfDay(m[2])
		if err != nil {
			return "", fmt.Errorf("parse time: %w", err)
		}
		return fmt.Sprintf("%d %d * * %s", min, hour, dayNum), nil
	}

	if m := regexp.MustCompile(`^(?:every\s+day|daily)\s+at\s+(.+)$`).FindStringSubmatch(s); m != nil {
		hour, min, err := parseTimeOfDay(m[1])
		if err != nil {
			return "", fmt.Errorf("parse time: %w", err)
		}
		return fmt.Sprintf("%d %d * * *", min, hour), nil
	}

	if m := regexp.MustCompile(`^at\s+(.+)$`).FindStringSubmatch(s); m != nil {
		hour, min, err := parseTimeOfDay(m[1])
		if err != nil {
			return "", fmt.Errorf("parse time: %w", err)
		}
		return fmt.Sprintf("%d %d * * *", min, hour), nil
	}

	if m := regexp.MustCompile(`^every\s+(\w+)$`).FindStringSubmatch(s); m != nil {
		if dow, ok := dayMap[m[1]]; ok {
			return fmt.Sprintf("0 0 * * %s", dow), nil
		}
	}

	if isCronLike(s) {
		if err := ValidateCronExpression(s); err == nil {
			return s, nil
		}
	}

	return "", fmt.Errorf("unrecognized schedule format: %q", input)
}

// parseTimeOfDay parses a time string like "9am", "9:30am", "15:00", "3pm",
// "noon", "midnight" and returns hour (0-23) and minute (0-59).
func parseTimeOfDay(s string) (hour, minute int, err error) {
	s = strings.TrimSpace(s)

	switch s {
	case "midnight":
		return 0, 0, nil
	case "noon":
		return 12, 0, nil
	}

	if m := regexp.MustCompile(`^(\d{1,2})(?::(\d{2}))?\s*(am|pm)$`).FindStringSubmatch(s); m != nil {
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		if min > 59 {
			return 0, 0, fmt.Errorf("invalid minute: %d", min)
		}
		switch {
		case m[3] == "am" && h == 12:
			h = 0
		case m[3] == "pm" && h != 12:
			h += 12
		}
		if h > 23 {
			return 0, 0, fmt.Errorf("invalid hour: %d", h)
		}
		return h, min, nil
	}

	if m := regexp.MustCompile(`^(\d{1,2})(?::(\d{2}))?$`).FindStringSubmatch(s); m != nil {
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		if h > 23 || min > 59 {
			return 0, 0, fmt.Errorf("invalid time: %02d:%02d", h, min)
		}
		return h, min, nil
	}

	return 0, 0, fmt.Errorf("cannot parse time: %q", s)
}

// isCronLike returns true when s looks like a 5-field cron expression
// (digits, asterisks, slashes, commas, hyphens in each field).
func isCronLike(s string) bool {
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return false
	}
	validField := regexp.MustCompile(`^[\d*/,\-]+$`)
	for _, f := range fields {
		if !validField.MatchString(f) {
			return false
		}
	}
	return true
}

// ValidateCronExpression validates a standard 5-field cron expression using
// robfig/cron. Returns nil if valid.
func ValidateCronExpression(expr string) error {
	_, err := cron.ParseStandard(expr)
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return nil
}

// DescribeCronExpression converts a standard 5-field cron expression into a
// human-readable description string.
func DescribeCronExpression(expr string) string {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return fmt.Sprintf("cron: %s", expr)
	}

	minute := fields[0]
	hour := fields[1]
	dayOfMonth := fields[2]
	month := fields[3]
	dayOfWeek := fields[4]

	var parts []string

	parts = append(parts, describeFieldTime(hour, minute))

	if dayOfWeek != "*" {
		parts = append(parts, describeDayOfWeek(dayOfWeek))
	}

	if month != "*" {
		parts = append(parts, describeMonth(month))
	}

	if dayOfMonth != "*" && dayOfWeek == "*" {
		parts = append(parts, describeDayOfMonth(dayOfMonth))
	}

	return strings.Join(parts, ", ")
}

func describeFieldTime(hour, minute string) string {
	h, hErr := strconv.Atoi(hour)
	m, mErr := strconv.Atoi(minute)

	if strings.HasPrefix(minute, "*/") && hour == "*" {
		return fmt.Sprintf("every %s minutes", strings.TrimPrefix(minute, "*/"))
	}

	if strings.HasPrefix(hour, "*/") && (minute == "0" || minute == "*/1") {
		return fmt.Sprintf("every %s hours", strings.TrimPrefix(hour, "*/"))
	}

	if strings.Contains(hour, ",") && minute == "0" {
		count := len(strings.Split(hour, ","))
		if count == 2 {
			return "twice a day"
		}
		return fmt.Sprintf("%d times a day", count)
	}

	if strings.Contains(minute, ",") && hour == "*" {
		count := len(strings.Split(minute, ","))
		if count == 2 {
			return "twice an hour"
		}
		return fmt.Sprintf("%d times an hour", count)
	}

	if hErr == nil && mErr == nil {
		return fmt.Sprintf("at %02d:%02d", h, m)
	}

	if minute == "*" && hour == "*" {
		return "every minute"
	}

	if hour == "*" && mErr == nil {
		return fmt.Sprintf("every hour at minute %d", m)
	}

	return fmt.Sprintf("%s:%s", hour, minute)
}

var cronDayNames = map[string]string{
	"0": "Sunday", "1": "Monday", "2": "Tuesday", "3": "Wednesday",
	"4": "Thursday", "5": "Friday", "6": "Saturday",
}

func describeDayOfWeek(dow string) string {
	if dow == "1-5" {
		return "on weekdays"
	}
	if dow == "0,6" {
		return "on weekends"
	}

	if name, ok := cronDayNames[dow]; ok {
		return fmt.Sprintf("on %ss", name)
	}

	if strings.Contains(dow, ",") {
		var names []string
		for _, d := range strings.Split(dow, ",") {
			if name, ok := cronDayNames[d]; ok {
				names = append(names, name)
			} else {
				names = append(names, d)
			}
		}
		return "on " + strings.Join(names, " and ")
	}

	if strings.Contains(dow, "-") {
		parts := strings.Split(dow, "-")
		if len(parts) == 2 {
			start, ok1 := cronDayNames[parts[0]]
			end, ok2 := cronDayNames[parts[1]]
			if ok1 && ok2 {
				return fmt.Sprintf("on %ss through %ss", start, end)
			}
		}
	}

	return fmt.Sprintf("on day-of-week %s", dow)
}

func describeMonth(month string) string {
	monthNames := map[string]string{
		"1": "January", "2": "February", "3": "March", "4": "April",
		"5": "May", "6": "June", "7": "July", "8": "August",
		"9": "September", "10": "October", "11": "November", "12": "December",
	}
	if name, ok := monthNames[month]; ok {
		return fmt.Sprintf("in %s", name)
	}
	if strings.HasPrefix(month, "*/") {
		return fmt.Sprintf("every %s months", strings.TrimPrefix(month, "*/"))
	}
	return fmt.Sprintf("in month %s", month)
}

func describeDayOfMonth(dom string) string {
	if strings.HasPrefix(dom, "*/") {
		return fmt.Sprintf("every %s days", strings.TrimPrefix(dom, "*/"))
	}
	return fmt.Sprintf("on day %s of the month", dom)
}
