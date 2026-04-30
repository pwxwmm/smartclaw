package memory

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// PreferenceSnapshot captures the current session's accumulated preferences
// across code style, communication style, and workflow patterns.
type PreferenceSnapshot struct {
	CodeStyle          map[string]string
	CommunicationStyle map[string]string
	WorkflowPatterns   []string
}

// PreferenceTracker observes user behavior across three categories:
//   - Code Style: indentation, naming conventions, language preferences
//   - Communication Style: verbosity, format preferences, explanation depth
//   - Common Patterns: frequently used workflows, tool usage patterns
//
// It delegates persistent storage to the UserModelingEngine.
type PreferenceTracker struct {
	engine          *UserModelingEngine
	sessionPatterns map[string]int // temporary pattern counter within a session
	mu              sync.Mutex

	// Workflow pattern observation threshold. When a tool+args pattern
	// occurs this many times within a session, it gets recorded as an
	// observation. Default: 3.
	WorkflowThreshold int
}

// NewPreferenceTracker creates a new tracker backed by the given
// UserModelingEngine.
func NewPreferenceTracker(engine *UserModelingEngine) *PreferenceTracker {
	return &PreferenceTracker{
		engine:          engine,
		sessionPatterns: make(map[string]int),
		WorkflowThreshold: 3,
	}
}

// ObserveCodeStyle analyzes a code snippet for style indicators and records
// them as observations with category="code_style".
func (pt *PreferenceTracker) ObserveCodeStyle(ctx context.Context, sessionID, codeSnippet string) error {
	if pt.engine == nil || codeSnippet == "" {
		return nil
	}

	observations := pt.detectCodeStyle(codeSnippet)
	for _, obs := range observations {
		if err := pt.engine.RecordObservation(ctx, "code_style", obs.key, obs.value, obs.confidence, sessionID); err != nil {
			return fmt.Errorf("preference tracker: observe code style: %w", err)
		}
	}
	return nil
}

// ObserveCommunication analyzes a user message for communication style
// indicators and records them as observations with category="communication_style".
func (pt *PreferenceTracker) ObserveCommunication(ctx context.Context, sessionID, userMessage string) error {
	if pt.engine == nil || userMessage == "" {
		return nil
	}

	observations := pt.detectCommunicationStyle(userMessage)
	for _, obs := range observations {
		if err := pt.engine.RecordObservation(ctx, "communication_style", obs.key, obs.value, obs.confidence, sessionID); err != nil {
			return fmt.Errorf("preference tracker: observe communication: %w", err)
		}
	}
	return nil
}

// ObserveWorkflow tracks tool usage patterns. When a pattern exceeds the
// threshold, it is recorded as an observation with category="workflow_pattern".
func (pt *PreferenceTracker) ObserveWorkflow(ctx context.Context, sessionID, toolName string, args map[string]any) error {
	if pt.engine == nil || toolName == "" {
		return nil
	}

	pattern := pt.buildWorkflowPattern(toolName, args)

	pt.mu.Lock()
	pt.sessionPatterns[pattern]++
	count := pt.sessionPatterns[pattern]
	shouldRecord := count >= pt.WorkflowThreshold
	if shouldRecord {
		pt.sessionPatterns[pattern] = -999 // sentinel to prevent re-recording
	}
	pt.mu.Unlock()

	if shouldRecord {
		if err := pt.engine.RecordObservation(ctx, "workflow_pattern", toolName, pattern, 0.7, sessionID); err != nil {
			return fmt.Errorf("preference tracker: observe workflow: %w", err)
		}
	}
	return nil
}

// GetCurrentPreferences returns the current session's accumulated preferences
// derived from the engine's snapshot.
func (pt *PreferenceTracker) GetCurrentPreferences() *PreferenceSnapshot {
	snapshot := pt.engine.GetSnapshot()
	if snapshot == nil {
		return &PreferenceSnapshot{
			CodeStyle:          make(map[string]string),
			CommunicationStyle: make(map[string]string),
		}
	}

	codeStyle := make(map[string]string)
	commStyle := make(map[string]string)
	var workflows []string

	for k, v := range snapshot.Preferences {
		codeStyle[k] = v
	}

	if snapshot.CommunicationStyle != "" {
		commStyle["style"] = snapshot.CommunicationStyle
	}

	for _, p := range snapshot.TopPatterns {
		workflows = append(workflows, p.Pattern)
	}

	pt.mu.Lock()
	for pattern, count := range pt.sessionPatterns {
		if count > 0 && count < pt.WorkflowThreshold {
			workflows = append(workflows, pattern)
		}
	}
	pt.mu.Unlock()

	return &PreferenceSnapshot{
		CodeStyle:          codeStyle,
		CommunicationStyle: commStyle,
		WorkflowPatterns:   workflows,
	}
}

// codeStyleObservation is an intermediate result from code style detection.
type codeStyleObservation struct {
	key        string
	value      string
	confidence float64
}

// detectCodeStyle performs rule-based analysis of a code snippet.
func (pt *PreferenceTracker) detectCodeStyle(snippet string) []codeStyleObservation {
	var observations []codeStyleObservation

	// Detect indentation style.
	if indentObs := pt.detectIndentation(snippet); indentObs != nil {
		observations = append(observations, *indentObs)
	}

	// Detect naming convention.
	if namingObs := pt.detectNamingConvention(snippet); namingObs != nil {
		observations = append(observations, *namingObs)
	}

	// Detect programming language.
	if langObs := pt.detectLanguage(snippet); langObs != nil {
		observations = append(observations, *langObs)
	}

	return observations
}

// detectIndentation determines whether the code uses tabs or spaces.
func (pt *PreferenceTracker) detectIndentation(snippet string) *codeStyleObservation {
	tabCount := 0
	spaceIndentCount := 0
	spaceWidth := 0

	lines := strings.Split(snippet, "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if line[0] == '\t' {
			tabCount++
		} else if line[0] == ' ' {
			spaceIndentCount++
			// Count leading spaces.
			w := 0
			for _, ch := range line {
				if ch == ' ' {
					w++
				} else {
					break
				}
			}
			if w > 0 && (spaceWidth == 0 || w == spaceWidth) {
				spaceWidth = w
			}
		}
	}

	if tabCount == 0 && spaceIndentCount == 0 {
		return nil
	}

	if tabCount > spaceIndentCount {
		return &codeStyleObservation{
			key:        "indentation",
			value:      "tabs",
			confidence: 0.8,
		}
	}

	if spaceIndentCount > tabCount {
		width := "4"
		if spaceWidth == 2 {
			width = "2"
		} else if spaceWidth == 8 {
			width = "8"
		}
		return &codeStyleObservation{
			key:        "indentation",
			value:      "spaces:" + width,
			confidence: 0.8,
		}
	}

	return nil
}

// camelCaseRegex matches camelCase or PascalCase identifiers.
var camelCaseRegex = regexp.MustCompile(`\b[a-z][a-zA-Z0-9]*\b|\b[A-Z][a-zA-Z0-9]*\b`)

// snakeCaseRegex matches snake_case identifiers.
var snakeCaseRegex = regexp.MustCompile(`\b[a-z][a-z0-9]*(_[a-z0-9]+)+\b`)

// detectNamingConvention identifies the dominant naming style in code.
func (pt *PreferenceTracker) detectNamingConvention(snippet string) *codeStyleObservation {
	snakeMatches := snakeCaseRegex.FindAllString(snippet, -1)
	camelMatches := camelCaseRegex.FindAllString(snippet, -1)

	// Filter camelMatches to exclude short words and snake_case words.
	var camelFiltered []string
	snakeSet := make(map[string]bool)
	for _, s := range snakeMatches {
		snakeSet[s] = true
	}
	for _, m := range camelMatches {
		if len(m) <= 2 || snakeSet[m] {
			continue
		}
		// Check for mixed case (at least one uppercase after the first char).
		hasUpper := false
		for _, ch := range m[1:] {
			if unicode.IsUpper(ch) {
				hasUpper = true
				break
			}
		}
		if hasUpper {
			camelFiltered = append(camelFiltered, m)
		}
	}

	snakeCount := len(snakeMatches)
	camelCount := len(camelFiltered)

	if snakeCount == 0 && camelCount == 0 {
		return nil
	}

	if snakeCount > camelCount {
		return &codeStyleObservation{
			key:        "naming",
			value:      "snake_case",
			confidence: 0.7,
		}
	}

	if camelCount > snakeCount {
		return &codeStyleObservation{
			key:        "naming",
			value:      "camelCase",
			confidence: 0.7,
		}
	}

	return nil
}

// detectLanguage attempts to identify the programming language from a snippet.
func (pt *PreferenceTracker) detectLanguage(snippet string) *codeStyleObservation {
	// Language indicators: keyword -> language mapping.
	indicators := []struct {
		pattern    string
		language   string
		confidence float64
	}{
		{"func ", "go", 0.7},
		{"package ", "go", 0.6},
		{":= ", "go", 0.7},
		{"import (", "go", 0.6},
		{"def ", "python", 0.6},
		{"self.", "python", 0.7},
		{"print(", "python", 0.5},
		{"fn ", "rust", 0.7},
		{"let mut ", "rust", 0.8},
		{"impl ", "rust", 0.7},
		{"pub fn ", "rust", 0.8},
		{"public class ", "java", 0.7},
		{"System.out.", "java", 0.8},
		{"private ", "java", 0.5},
		{"const ", "typescript", 0.5},
		{"interface ", "typescript", 0.5},
		{": string", "typescript", 0.6},
		{": number", "typescript", 0.7},
		{"-> ", "python", 0.4},
		{"if __name__", "python", 0.9},
	}

	languageScores := make(map[string]float64)
	for _, ind := range indicators {
		if strings.Contains(snippet, ind.pattern) {
			languageScores[ind.language] += ind.confidence
		}
	}

	if len(languageScores) == 0 {
		return nil
	}

	// Pick the language with the highest score.
	var bestLang string
	bestScore := 0.0
	for lang, score := range languageScores {
		if score > bestScore {
			bestScore = score
			bestLang = lang
		}
	}

	if bestLang == "" || bestScore < 0.6 {
		return nil
	}

	// Cap confidence at 0.9.
	confidence := bestScore
	if confidence > 0.9 {
		confidence = 0.9
	}

	return &codeStyleObservation{
		key:        "language",
		value:      bestLang,
		confidence: confidence,
	}
}

// detectCommunicationStyle analyzes a user message for communication patterns.
func (pt *PreferenceTracker) detectCommunicationStyle(message string) []codeStyleObservation {
	var observations []codeStyleObservation

	// Detect verbosity by word count.
	wordCount := len(strings.Fields(message))
	verbosity := "moderate"
	confidence := 0.5
	if wordCount < 10 {
		verbosity = "brief"
		confidence = 0.6
	} else if wordCount > 50 {
		verbosity = "verbose"
		confidence = 0.6
	}
	observations = append(observations, codeStyleObservation{
		key:        "verbosity",
		value:      verbosity,
		confidence: confidence,
	})

	// Detect question frequency.
	questionCount := strings.Count(message, "?")
	if questionCount > 2 {
		observations = append(observations, codeStyleObservation{
			key:        "question_style",
			value:      "inquisitive",
			confidence: 0.6,
		})
	}

	// Detect directive vs exploratory style.
	lower := strings.ToLower(message)
	directiveWords := []string{"do this", "make it", "fix", "implement", "create", "delete", "remove", "add"}
	exploratoryWords := []string{"what if", "how about", "maybe", "perhaps", "could we", "i wonder", "explore"}

	directiveScore := 0
	for _, w := range directiveWords {
		if strings.Contains(lower, w) {
			directiveScore++
		}
	}
	exploratoryScore := 0
	for _, w := range exploratoryWords {
		if strings.Contains(lower, w) {
			exploratoryScore++
		}
	}

	if directiveScore > exploratoryScore && directiveScore > 0 {
		observations = append(observations, codeStyleObservation{
			key:        "interaction_style",
			value:      "directive",
			confidence: 0.6,
		})
	} else if exploratoryScore > directiveScore && exploratoryScore > 0 {
		observations = append(observations, codeStyleObservation{
			key:        "interaction_style",
			value:      "exploratory",
			confidence: 0.6,
		})
	}

	// Detect format preference: markdown vs plain.
	if strings.Contains(message, "```") || strings.Contains(message, "## ") ||
		strings.Contains(message, "- ") || strings.Contains(message, "**") {
		observations = append(observations, codeStyleObservation{
			key:        "format",
			value:      "markdown",
			confidence: 0.7,
		})
	}

	return observations
}

// buildWorkflowPattern creates a string key from a tool invocation.
func (pt *PreferenceTracker) buildWorkflowPattern(toolName string, args map[string]any) string {
	var parts []string
	parts = append(parts, toolName)

	// Include the most relevant arg keys to differentiate patterns.
	relevantKeys := []string{"action", "type", "mode", "command", "path", "query"}
	for _, k := range relevantKeys {
		if v, ok := args[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}

	return strings.Join(parts, ":")
}
