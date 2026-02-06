package cmd

import (
	"fmt"
	"os"

	"github.com/strongdm/agate/internal/workflow"
	"github.com/spf13/cobra"
)

var interruptCmd = &cobra.Command{
	Use:     "suggest 'prompt'",
	Aliases: []string{"interrupt"},
	Short:   "Send a suggestion to guide the next task",
	Long: `Send a suggestion to guide the next agent invocation.

The suggestion will be processed at the start of the next 'agate next' command.
Use this to:
  - Provide additional context
  - Suggest focus areas
  - Reorder tasks
  - Modify priorities

Alias: interrupt (for backwards compatibility)

Example:
  agate suggest 'focus on error handling first'
  agate suggest 'add input validation before processing'`,
	Args: cobra.ExactArgs(1),
	RunE: runInterrupt,
}

func init() {
	rootCmd.AddCommand(interruptCmd)
}

func runInterrupt(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		PrintError("failed to get current directory: %v", err)
		SetExitCode(2)
		return err
	}

	prompt := args[0]
	if prompt == "" {
		PrintError("interrupt prompt cannot be empty")
		SetExitCode(2)
		return fmt.Errorf("empty prompt")
	}

	result, err := workflow.AddInterrupt(cwd, prompt)
	if err != nil {
		PrintError("%v", err)
		SetExitCode(2)
		return err
	}

	fmt.Println(result)
	SetExitCode(0)
	return nil
}
