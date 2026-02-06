package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/strongdm/agate/internal/project"
	"github.com/spf13/cobra"
)

var version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "agate",
	Short: "AI orchestrator CLI",
	Long: `agate is a non-interactive AI orchestrator that transforms a GOAL.md
into working software through multi-agent collaboration.

Quick start:
  mkdir my-project && cd my-project
  echo "Build a hello world CLI in Go" > GOAL.md
  agate auto

Agents (use --agent with auto/next):
  claude  Claude Opus 4.5   - Most capable, default
  haiku   Claude 3.5 Haiku  - Fast, cheap, good for testing
  codex   GPT 5.2           - OpenAI alternative
  dummy   No-op             - For workflow testing`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Regenerate built-in skills on every command
		// This ensures _ prefixed skills are always up to date
		wd, err := os.Getwd()
		if err != nil {
			return // Silently skip if we can't get working directory
		}
		skillsDir := filepath.Join(wd, ".ai", "skills")
		// Only regenerate if the skills directory exists (project is initialized)
		if _, err := os.Stat(skillsDir); err == nil {
			if err := project.EnsureBuiltinSkills(skillsDir); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to regenerate built-in skills: %v\n", err)
			}
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("agate version {{.Version}}\n")

	// Disable the default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Silence Cobra's automatic error and usage printing for RunE errors.
	// Our commands handle their own error output via PrintError.
	// Cobra still prints errors for unknown commands, bad flags, etc.
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
}

// exitCode is used to track the desired exit code
var exitCode int

// SetExitCode sets the exit code to be used when the program exits
func SetExitCode(code int) {
	exitCode = code
}

// GetExitCode returns the current exit code
func GetExitCode() int {
	return exitCode
}

// ExitWithCode exits with the appropriate code based on workflow state
func ExitWithCode() {
	os.Exit(exitCode)
}

// PrintError prints an error message to stderr
func PrintError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}
