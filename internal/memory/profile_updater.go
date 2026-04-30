package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/instructkr/smartclaw/internal/memory/layers"
)

// ProfileUpdater orchestrates the full user profile update cycle at session
// end or on demand. It collects observations, runs dialectic synthesis,
// formats the result into USER.md-compatible markdown, and persists it
// within the 3,575 character budget.
type ProfileUpdater struct {
	engine    *UserModelingEngine
	promptMem *layers.PromptMemory
	tracker   *PreferenceTracker

	// UpdateThreshold is the minimum number of new observations required
	// before an update is triggered. Default: 5.
	UpdateThreshold int
}

// NewProfileUpdater creates a new profile updater.
func NewProfileUpdater(engine *UserModelingEngine, pm *layers.PromptMemory, tracker *PreferenceTracker) *ProfileUpdater {
	return &ProfileUpdater{
		engine:          engine,
		promptMem:       pm,
		tracker:         tracker,
		UpdateThreshold: 5,
	}
}

// UpdateProfile runs the full update cycle:
//  1. Runs dialectic synthesis to get the latest snapshot
//  2. Formats the snapshot into USER.md-compatible markdown
//  3. Trims to the 3,575 character budget if necessary
//  4. Persists via PromptMemory
func (pu *ProfileUpdater) UpdateProfile(ctx context.Context) error {
	if pu.engine == nil {
		return fmt.Errorf("profile updater: engine not available")
	}

	snapshot, err := pu.engine.SynthesizeModel(ctx)
	if err != nil {
		return fmt.Errorf("profile updater: synthesize model: %w", err)
	}

	formatted := pu.FormatProfile(snapshot)

	// Enforce the character budget.
	trimmed := pu.TrimToBudget(formatted, layers.MaxPromptMemoryChars)

	if pu.promptMem != nil {
		if err := pu.promptMem.UpdateUserProfile(trimmed); err != nil {
			return fmt.Errorf("profile updater: update user profile: %w", err)
		}
	}

	slog.Info("profile updater: user profile updated",
		"chars", len(trimmed),
		"preferences", len(snapshot.Preferences),
		"patterns", len(snapshot.TopPatterns),
	)

	return nil
}

// ShouldUpdate returns true if there are enough new observations since
// the last update to warrant a profile refresh.
func (pu *ProfileUpdater) ShouldUpdate(observationsSinceLastUpdate int) bool {
	return observationsSinceLastUpdate >= pu.UpdateThreshold
}

// FormatProfile formats a UserModelSnapshot into USER.md-compatible
// markdown with sections: ## Code Style, ## Communication Preferences,
// ## Work Patterns, ## Knowledge.
func (pu *ProfileUpdater) FormatProfile(snapshot *UserModelSnapshot) string {
	if snapshot == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# User Profile\n\n")

	// Code Style section (preferences with code_style prefix or generic).
	codeStyleKeys := make(map[string]string)
	otherPrefs := make(map[string]string)
	for k, v := range snapshot.Preferences {
		if k == "indentation" || k == "naming" || k == "language" ||
			k == "framework" || k == "editor" {
			codeStyleKeys[k] = v
		} else {
			otherPrefs[k] = v
		}
	}

	if len(codeStyleKeys) > 0 {
		sb.WriteString("## Code Style\n")
		for k, v := range codeStyleKeys {
			sb.WriteString(formatKV(k, v))
		}
		sb.WriteString("\n")
	}

	// Communication Preferences section.
	if snapshot.CommunicationStyle != "" || len(otherPrefs) > 0 {
		sb.WriteString("## Communication Preferences\n")
		if snapshot.CommunicationStyle != "" {
			sb.WriteString(formatKV("style", snapshot.CommunicationStyle))
		}
		for k, v := range otherPrefs {
			sb.WriteString(formatKV(k, v))
		}
		sb.WriteString("\n")
	}

	// Work Patterns section.
	if len(snapshot.TopPatterns) > 0 {
		sb.WriteString("## Work Patterns\n")
		for _, p := range snapshot.TopPatterns {
			sb.WriteString(fmt.Sprintf("- %s (freq: %d)\n", p.Pattern, p.Frequency))
		}
		sb.WriteString("\n")
	}

	// Knowledge section.
	if len(snapshot.KnowledgeBackground) > 0 {
		sb.WriteString("## Knowledge\n")
		for _, k := range snapshot.KnowledgeBackground {
			sb.WriteString(fmt.Sprintf("- %s\n", k))
		}
		sb.WriteString("\n")
	}

	// Conflicts section (only unresolved).
	var unresolved []ObservationConflict
	for _, c := range snapshot.Conflicts {
		if !c.Resolved {
			unresolved = append(unresolved, c)
		}
	}
	if len(unresolved) > 0 {
		sb.WriteString("## Unresolved Conflicts\n")
		for _, c := range unresolved {
			sb.WriteString(fmt.Sprintf("- %s/%s: %q vs %q\n",
				c.Category, c.Key, c.Thesis, c.Antithesis))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// TrimToBudget trims content from the end (least important patterns) to
// fit within maxChars. It removes lines starting from the bottom until
// the content fits, preserving section headers where possible.
func (pu *ProfileUpdater) TrimToBudget(content string, maxChars int) string {
	if len(content) <= maxChars {
		return content
	}

	lines := strings.Split(content, "\n")

	// Remove lines from the end until we fit.
	// Skip empty trailing lines first.
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	for len(lines) > 1 {
		// Calculate total size including newlines.
		joined := strings.Join(lines, "\n")
		if len(joined) <= maxChars {
			return joined
		}

		// Remove the last non-empty line.
		lines = lines[:len(lines)-1]

		// Remove trailing empty lines.
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
	}

	result := strings.Join(lines, "\n")
	if len(result) > maxChars {
		result = result[:maxChars]
	}
	return result
}

// formatKV formats a key-value pair as a markdown list item.
func formatKV(key, value string) string {
	return fmt.Sprintf("- %s: %s\n", key, value)
}
