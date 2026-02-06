package cmd

import (
	"fmt"
	"os"

	"github.com/strongdm/agate/internal/workflow"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current progress and relevant files",
	Long: `Show the current state of the project including:
  - Goal status
  - Design completion
  - Generated skills
  - Sprint progress
  - Next recommended action

Exit codes:
  0   - All work complete (all sprints done)
  1   - More work remains (run 'agate next')
  2   - Error occurred
  255 - Human action required (create GOAL.md, answer interview)`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		PrintError("failed to get current directory: %v", err)
		SetExitCode(2)
		return err
	}

	output, result, err := workflow.StatusWithResult(cwd)
	if err != nil {
		PrintError("%v", err)
		SetExitCode(2)
		return err
	}

	fmt.Print(output)
	SetExitCode(workflow.GetExitCode(result))
	return nil
}
