package logging

import "github.com/fatih/color"

// Color sprint functions for consistent terminal output styling.
// These respect NO_COLOR and non-TTY environments automatically.
var (
	Green    = color.New(color.FgGreen).SprintFunc()
	Yellow   = color.New(color.FgYellow).SprintFunc()
	Cyan     = color.New(color.FgCyan).SprintFunc()
	Bold     = color.New(color.Bold).SprintFunc()
	BoldCyan = color.New(color.Bold, color.FgCyan).SprintFunc()
	Dim      = color.New(color.Faint).SprintFunc()
)
