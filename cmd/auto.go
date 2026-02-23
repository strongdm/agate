package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/strongdm/agate/internal/logging"
)

var autoAgent string

var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Run next repeatedly until complete",
	Long: `Run 'agate next' in a loop until the project is complete.

Behavior by exit code from each step:
  0   - Done, stop looping
  1   - More work, continue looping
  255 - Human action needed, stop looping
  Other - Error, stop looping

On exit 255, the loop stops so you can take action (create GOAL.md,
answer interview questions, etc.) then re-run 'agate auto'.

Any text typed on stdin between steps is sent as a suggestion
via 'agate suggest' before the next step.

Exit codes:
  0   - All work complete
  255 - Human action required`,
	RunE: runAuto,
}

func init() {
	autoCmd.Flags().StringVarP(&autoAgent, "agent", "a", "", "Select agent: haiku, claude, codex, dummy")
	rootCmd.AddCommand(autoCmd)
}

func runAuto(cmd *cobra.Command, args []string) error {
	runner := NewAutoRunner(realExec, os.Stdin, os.Stdout, os.Stderr)
	code := runner.Run(autoAgent)
	SetExitCode(code)
	return nil
}

// ExecFunc runs an agate subcommand as a subprocess.
// args are the subcommand arguments (e.g. ["next", "--agent", "claude"]).
// stdout and stderr receive the child process output.
// Returns the exit code (0+ on normal exit) or -1 with an error on exec failure.
type ExecFunc func(args []string, stdout, stderr io.Writer) (exitCode int, err error)

// realExec runs an agate subcommand via os/exec.
func realExec(args []string, stdout, stderr io.Writer) (int, error) {
	binary, err := os.Executable()
	if err != nil {
		binary = os.Args[0]
	}
	c := exec.Command(binary, args...)
	c.Stdout = stdout
	c.Stderr = stderr
	err = c.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

// AutoRunner implements the auto command loop.
// All interaction with agate subcommands goes through Exec,
// making the loop fully testable without real subprocesses.
type AutoRunner struct {
	Exec   ExecFunc
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// NewAutoRunner creates an AutoRunner.
func NewAutoRunner(execFn ExecFunc, stdin io.Reader, stdout, stderr io.Writer) *AutoRunner {
	return &AutoRunner{
		Exec:   execFn,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
}

// Run executes the auto loop. Returns the process exit code.
func (r *AutoRunner) Run(agent string) int {
	// Read stdin lines in background so we can pick up unsolicited input
	inputCh := make(chan string, 100)
	go func() {
		defer close(inputCh)
		scanner := bufio.NewScanner(r.Stdin)
		for scanner.Scan() {
			inputCh <- scanner.Text()
		}
	}()

	const maxConsecutiveErrors = 3

	step := 0
	consecutiveErrors := 0
	for {
		// Drain any pending input and send as suggestions
		r.drainSuggestions(inputCh)

		step++
		fmt.Fprintf(r.Stdout, "%s Step %d\n", logging.BoldCyan("[auto]"), step)

		args := []string{"next"}
		if agent != "" {
			args = append(args, "--agent", agent)
		}

		exitCode, err := r.Exec(args, r.Stdout, r.Stderr)
		if err != nil {
			fmt.Fprintf(r.Stderr, "%s %s\n", logging.BoldCyan("[auto]"), logging.Yellow(fmt.Sprintf("Failed to execute: %v", err)))
			return 2
		}

		switch exitCode {
		case 0:
			fmt.Fprintf(r.Stdout, "%s %s\n", logging.BoldCyan("[auto]"), logging.Green("Done!"))
			return 0
		case 1:
			// More work, loop
			consecutiveErrors = 0
			continue
		case 255:
			// Human action needed — exit so user can act
			fmt.Fprintf(r.Stdout, "%s %s\n", logging.BoldCyan("[auto]"), logging.Yellow("Human action required, exiting."))
			return 255
		default:
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				fmt.Fprintf(r.Stderr, "%s %s\n", logging.BoldCyan("[auto]"), logging.Yellow(fmt.Sprintf("Stopped after %d consecutive errors (last exit code %d)", consecutiveErrors, exitCode)))
				return exitCode
			}
			fmt.Fprintf(r.Stderr, "%s %s\n", logging.BoldCyan("[auto]"), logging.Yellow(fmt.Sprintf("Error (exit code %d), retrying (%d/%d)...", exitCode, consecutiveErrors, maxConsecutiveErrors)))
			continue
		}
	}
}

// drainSuggestions sends any pending stdin lines as suggestions (non-blocking).
func (r *AutoRunner) drainSuggestions(ch <-chan string) {
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return
			}
			line = strings.TrimSpace(line)
			if line != "" {
				r.sendSuggest(line)
			}
		default:
			return
		}
	}
}

// sendSuggest runs 'agate suggest' with the given text.
func (r *AutoRunner) sendSuggest(text string) {
	fmt.Fprintf(r.Stdout, "%s Sending suggestion: %s\n", logging.BoldCyan("[auto]"), text)
	_, err := r.Exec([]string{"suggest", text}, r.Stdout, r.Stderr)
	if err != nil {
		fmt.Fprintf(r.Stderr, "%s %s\n", logging.BoldCyan("[auto]"), logging.Yellow(fmt.Sprintf("Warning: suggest failed: %v", err)))
	}
}
