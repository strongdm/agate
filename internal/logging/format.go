package logging

import (
	"fmt"
	"strings"
	"time"
)

// FormatInvocation formats an invocation as markdown
func FormatInvocation(inv *Invocation) string {
	var sb strings.Builder

	sb.WriteString("# Agent Invocation Log\n\n")

	// Metadata table
	sb.WriteString("## Metadata\n\n")
	sb.WriteString("| Field | Value |\n")
	sb.WriteString("|-------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Timestamp | %s |\n", inv.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("| Sprint | %d |\n", inv.Sprint))
	sb.WriteString(fmt.Sprintf("| Phase | %s |\n", inv.Phase))
	sb.WriteString(fmt.Sprintf("| Task | %d - %s |\n", inv.TaskIndex, inv.Task))
	sb.WriteString(fmt.Sprintf("| Agent | %s |\n", inv.Agent))
	sb.WriteString(fmt.Sprintf("| Skill | %s |\n", inv.Skill))
	sb.WriteString(fmt.Sprintf("| Duration | %.2fs |\n", inv.Duration.Seconds()))
	sb.WriteString(fmt.Sprintf("| Status | %s |\n", inv.Status))
	sb.WriteString("\n")

	// Prompt section
	sb.WriteString("## Prompt\n\n")
	sb.WriteString("```\n")
	sb.WriteString(inv.Prompt)
	if !strings.HasSuffix(inv.Prompt, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("```\n\n")

	// Response section
	sb.WriteString("## Response\n\n")
	sb.WriteString("```\n")
	sb.WriteString(inv.Response)
	if !strings.HasSuffix(inv.Response, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("```\n\n")

	// Files written
	if len(inv.FilesWritten) > 0 {
		sb.WriteString("## Files Written\n\n")
		for _, f := range inv.FilesWritten {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}

	// Error if any
	if inv.Error != nil {
		sb.WriteString("## Error\n\n")
		sb.WriteString("```\n")
		sb.WriteString(inv.Error.Error())
		sb.WriteString("\n```\n\n")
	}

	// Notes
	if inv.Notes != "" {
		sb.WriteString("## Notes\n\n")
		sb.WriteString(inv.Notes)
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatRetro formats a retrospective summary
func FormatRetro(sprintNumber int, summary string, skillUpdates map[string]string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Sprint %d Retrospective\n\n", sprintNumber))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	sb.WriteString("## Summary\n\n")
	sb.WriteString(summary)
	sb.WriteString("\n\n")

	if len(skillUpdates) > 0 {
		sb.WriteString("## Skill Updates Applied\n\n")
		for skill, update := range skillUpdates {
			sb.WriteString(fmt.Sprintf("### %s\n\n", skill))
			sb.WriteString(update)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// FormatInterview formats interview questions
func FormatInterview(questions []InterviewQuestion) string {
	var sb strings.Builder

	sb.WriteString("# Project Interview\n\n")
	sb.WriteString("Check boxes and/or fill in blockquotes, then check the completion box at the bottom.\n\n")

	for i, q := range questions {
		sb.WriteString(fmt.Sprintf("### Q%d: %s\n\n", i+1, q.Title))
		sb.WriteString(q.Question)
		sb.WriteString("\n\n")
		if len(q.Options) > 0 {
			for _, opt := range q.Options {
				sb.WriteString(fmt.Sprintf("- [ ] %s\n", opt))
			}
			sb.WriteString("- [ ] No preference\n")
			sb.WriteString("\n> Notes:\n")
		} else {
			sb.WriteString("> Answer:\n")
		}
		sb.WriteString("\n---\n\n")
	}

	sb.WriteString("- [ ] All questions answered (check when complete)\n")

	return sb.String()
}

// InterviewQuestion represents a single interview question
type InterviewQuestion struct {
	Title    string
	Question string
	Options  []string
}

// ParseInterviewStatus checks if the interview is complete
// Supports both new checkbox format and legacy "Status: COMPLETE" format
func ParseInterviewStatus(content string) bool {
	// New format: checkbox "- [x] All questions answered"
	if strings.Contains(content, "- [x] All questions answered") ||
		strings.Contains(content, "- [X] All questions answered") {
		return true
	}

	// Legacy format: "Status: COMPLETE"
	if strings.Contains(content, "Status: COMPLETE") {
		return true
	}

	return false
}

// ParseInterviewAnswers extracts answers from interview file.
// Supports the new checkbox/blockquote format and the legacy **Answer**: format.
func ParseInterviewAnswers(content string) map[string]string {
	answers := make(map[string]string)

	lines := strings.Split(content, "\n")
	var currentQuestion string
	var checked []string

	flushQuestion := func() {
		if currentQuestion != "" && len(checked) > 0 {
			if existing, ok := answers[currentQuestion]; ok && existing != "" {
				answers[currentQuestion] = strings.Join(checked, ", ") + "\n" + existing
			} else {
				answers[currentQuestion] = strings.Join(checked, ", ")
			}
		}
		checked = nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "### Q") {
			flushQuestion()
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				currentQuestion = parts[1]
			}
		} else if currentQuestion != "" {
			trimmed := strings.TrimSpace(line)

			// Checked checkbox
			if strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ") {
				val := trimmed[6:]
				if val != "No preference" {
					checked = append(checked, val)
				}
			}

			// Blockquote answer/notes
			if strings.HasPrefix(trimmed, "> Notes:") || strings.HasPrefix(trimmed, "> Answer:") {
				var prefix string
				if strings.HasPrefix(trimmed, "> Notes:") {
					prefix = "> Notes:"
				} else {
					prefix = "> Answer:"
				}
				text := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
				if text != "" {
					if existing, ok := answers[currentQuestion]; ok && existing != "" {
						answers[currentQuestion] = existing + "\n" + text
					} else {
						answers[currentQuestion] = text
					}
				}
			}

			// Legacy **Answer**: format
			if strings.HasPrefix(trimmed, "**Answer**:") {
				text := strings.TrimSpace(strings.TrimPrefix(trimmed, "**Answer**:"))
				if text != "" {
					answers[currentQuestion] = text
				}
			}
		}
	}

	flushQuestion()
	return answers
}
