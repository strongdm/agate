package workflow

import "fmt"

// Interrupt handling has been simplified in Sprint 006.
// Suggestions are processed ad-hoc via `agate suggest` - no persistence needed.

// AddInterrupt acknowledges a suggestion but does not persist it.
// This is a simplified version that just acknowledges the suggestion.
func AddInterrupt(projectDir string, prompt string) (string, error) {
	// We no longer persist suggestions - just acknowledge them
	return fmt.Sprintf("Suggestion noted: %s\n\nNote: Suggestions are no longer queued. Use this command right before 'agate next' if you want to influence the next task.", truncatePrompt(prompt, 60)), nil
}

func truncatePrompt(prompt string, maxLen int) string {
	if len(prompt) <= maxLen {
		return prompt
	}
	return prompt[:maxLen-3] + "..."
}
