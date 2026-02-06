package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/strongdm/agate/internal/logging"
	"github.com/strongdm/agate/internal/workflow"
	"github.com/spf13/cobra"
)

var nextTail bool
var nextAgent string

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Advance one step in the sprint cycle",
	Long: `Advance exactly one step in the sprint cycle.

This command determines the current phase and executes the next step:
  - If no design exists: runs planning automatically
  - If in implementation phase: implements one task
  - If in review phase: reviews changes

Use --agent to select which AI agent to use:
  --agent haiku   Claude 3.5 Haiku (fast, cheap)
  --agent claude  Claude Opus 4.5 (most capable)
  --agent codex   GPT 5.2 (OpenAI)
  --agent dummy   No-op (for testing)

Exit codes:
  0   - All work complete (all sprints done)
  1   - Step completed, more work remains
  2   - Error occurred
  255 - Human action required (create GOAL.md, answer interview)`,
	RunE: runNext,
}

func init() {
	nextCmd.Flags().BoolVarP(&nextTail, "tail", "t", false, "Stream agent output to terminal in real-time")
	nextCmd.Flags().StringVarP(&nextAgent, "agent", "a", "", "Select agent: haiku, claude, codex, dummy")
	rootCmd.AddCommand(nextCmd)
}

func runNext(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		PrintError("failed to get current directory: %v", err)
		SetExitCode(2)
		return err
	}

	opts := workflow.NextOptions{
		PreferredAgent: nextAgent,
	}

	// Set up streaming if -tail is enabled
	if nextTail {
		// Create a split view for terminal output
		sv := logging.NewSplitView(os.Stdout, 2)
		if sv.IsTTY() {
			sv.Setup()
			defer sv.Teardown()
			sv.SetStatus(0, "Starting...")
		}
		opts.StreamOutput = sv
	}

	result, err := workflow.NextWithOptions(cwd, opts)
	if err != nil {
		// Check for explicit human-needed error (e.g. too many review failures)
		var humanErr *workflow.HumanNeededError
		if errors.As(err, &humanErr) {
			PrintError("%v", err)
			SetExitCode(workflow.ExitHumanNeeded)
			return err
		}
		// Check if this is a human-action error (no goal, etc.)
		// by inspecting current state
		fsys := os.DirFS(cwd)
		status := workflow.GetStatus(fsys)
		exitCode := workflow.GetExitCode(status)
		if exitCode == workflow.ExitHumanNeeded {
			PrintError("%v", err)
			SetExitCode(workflow.ExitHumanNeeded)
			return err
		}
		PrintError("%v", err)
		SetExitCode(2)
		return err
	}

	fmt.Println(result.Message)

	// Determine exit code from current state after the step
	fsys := os.DirFS(cwd)
	status := workflow.GetStatus(fsys)
	SetExitCode(workflow.GetExitCode(status))

	return nil
}
