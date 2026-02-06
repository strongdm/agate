package cmd

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockCall records a single call to the exec function.
type mockCall struct {
	Args []string
}

// mockExec builds an ExecFunc that returns exit codes from a sequence
// and records all calls. "next" calls consume from nextCodes; "suggest"
// calls always return 0.
func mockExec(nextCodes []int) (ExecFunc, *[]mockCall) {
	var mu sync.Mutex
	var calls []mockCall
	idx := 0

	fn := func(args []string, stdout, stderr io.Writer) (int, error) {
		mu.Lock()
		calls = append(calls, mockCall{Args: append([]string{}, args...)})
		mu.Unlock()

		if len(args) > 0 && args[0] == "suggest" {
			return 0, nil
		}

		mu.Lock()
		code := nextCodes[idx]
		idx++
		mu.Unlock()
		return code, nil
	}
	return fn, &calls
}

func TestAutoRunner_StopsOnZero(t *testing.T) {
	exec, calls := mockExec([]int{0})
	var out bytes.Buffer
	runner := NewAutoRunner(exec, strings.NewReader(""), &out, &out)

	code := runner.Run("")
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}

	// Should have called "next" exactly once
	nextCalls := filterCalls(*calls, "next")
	if len(nextCalls) != 1 {
		t.Errorf("expected 1 next call, got %d", len(nextCalls))
	}

	if !strings.Contains(out.String(), "[auto] Done!") {
		t.Errorf("expected Done message, got: %s", out.String())
	}
}

func TestAutoRunner_LoopsOnExitOne(t *testing.T) {
	exec, calls := mockExec([]int{1, 1, 1, 0})
	var out bytes.Buffer
	runner := NewAutoRunner(exec, strings.NewReader(""), &out, &out)

	code := runner.Run("")
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}

	nextCalls := filterCalls(*calls, "next")
	if len(nextCalls) != 4 {
		t.Errorf("expected 4 next calls, got %d", len(nextCalls))
	}

	// Verify step numbers in output
	if !strings.Contains(out.String(), "[auto] Step 4") {
		t.Errorf("expected Step 4 in output, got: %s", out.String())
	}
}

func TestAutoRunner_RetriesOnError(t *testing.T) {
	// Single error followed by success — should retry and recover
	exec, calls := mockExec([]int{1, 2, 1, 0})
	var stdout, stderr bytes.Buffer
	runner := NewAutoRunner(exec, strings.NewReader(""), &stdout, &stderr)

	code := runner.Run("")
	if code != 0 {
		t.Errorf("expected exit 0 (recovered), got %d", code)
	}

	nextCalls := filterCalls(*calls, "next")
	if len(nextCalls) != 4 {
		t.Errorf("expected 4 next calls, got %d", len(nextCalls))
	}

	if !strings.Contains(stderr.String(), "retrying") {
		t.Errorf("expected retry message in stderr, got: %s", stderr.String())
	}
}

func TestAutoRunner_StopsAfterMaxErrors(t *testing.T) {
	// 3 consecutive errors should stop
	exec, calls := mockExec([]int{1, 2, 2, 2})
	var stdout, stderr bytes.Buffer
	runner := NewAutoRunner(exec, strings.NewReader(""), &stdout, &stderr)

	code := runner.Run("")
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}

	nextCalls := filterCalls(*calls, "next")
	if len(nextCalls) != 4 {
		t.Errorf("expected 4 next calls (1 ok + 3 errors), got %d", len(nextCalls))
	}

	if !strings.Contains(stderr.String(), "consecutive errors") {
		t.Errorf("expected consecutive errors message, got stderr: %s", stderr.String())
	}
}

func TestAutoRunner_ErrorCounterResetsOnSuccess(t *testing.T) {
	// 2 errors, then success, then 2 errors, then success — should not stop
	exec, calls := mockExec([]int{2, 2, 1, 2, 2, 1, 0})
	var stdout, stderr bytes.Buffer
	runner := NewAutoRunner(exec, strings.NewReader(""), &stdout, &stderr)

	code := runner.Run("")
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}

	nextCalls := filterCalls(*calls, "next")
	if len(nextCalls) != 7 {
		t.Errorf("expected 7 next calls, got %d", len(nextCalls))
	}
}

func TestAutoRunner_StopsOn255(t *testing.T) {
	exec, calls := mockExec([]int{255})
	var stdout, stderr bytes.Buffer
	runner := NewAutoRunner(exec, strings.NewReader(""), &stdout, &stderr)

	code := runner.Run("")
	if code != 255 {
		t.Errorf("expected exit 255, got %d", code)
	}

	// Should have called "next" exactly once, then stopped
	nextCalls := filterCalls(*calls, "next")
	if len(nextCalls) != 1 {
		t.Errorf("expected 1 next call, got %d", len(nextCalls))
	}

	if !strings.Contains(stdout.String(), "Human action required") {
		t.Errorf("expected human action message, got: %s", stdout.String())
	}
}

func TestAutoRunner_UnsolicitedInput(t *testing.T) {
	// Simulate: user types "use smaller functions" while step 1 is running.
	// We write to a pipe during the mock exec, then sleep briefly to let
	// the scanner goroutine process the line into the channel before
	// the main loop's drain runs.
	pr, pw := io.Pipe()
	nextIdx := 0

	exec := func(args []string, stdout, stderr io.Writer) (int, error) {
		if args[0] == "suggest" {
			return 0, nil
		}
		nextIdx++
		if nextIdx == 1 {
			// During step 1, user types input
			pw.Write([]byte("use smaller functions\n"))
			// Give scanner goroutine time to send to channel
			time.Sleep(10 * time.Millisecond)
			return 1, nil
		}
		pw.Close()
		return 0, nil
	}

	var calls []mockCall
	wrappedExec := func(args []string, stdout, stderr io.Writer) (int, error) {
		calls = append(calls, mockCall{Args: append([]string{}, args...)})
		return exec(args, stdout, stderr)
	}

	var out bytes.Buffer
	runner := NewAutoRunner(wrappedExec, pr, &out, &out)

	code := runner.Run("")
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}

	suggestCalls := filterCalls(calls, "suggest")
	if len(suggestCalls) != 1 {
		t.Errorf("expected 1 suggest call for unsolicited input, got %d: %v", len(suggestCalls), calls)
	}
	if len(suggestCalls) > 0 && suggestCalls[0].Args[1] != "use smaller functions" {
		t.Errorf("expected suggest text 'use smaller functions', got %q", suggestCalls[0].Args[1])
	}
}

func TestAutoRunner_PassesAgentFlag(t *testing.T) {
	exec, calls := mockExec([]int{0})
	var out bytes.Buffer
	runner := NewAutoRunner(exec, strings.NewReader(""), &out, &out)

	code := runner.Run("haiku")
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}

	nextCalls := filterCalls(*calls, "next")
	if len(nextCalls) != 1 {
		t.Fatalf("expected 1 next call, got %d", len(nextCalls))
	}

	args := nextCalls[0].Args
	if len(args) != 3 || args[1] != "--agent" || args[2] != "haiku" {
		t.Errorf("expected [next --agent haiku], got %v", args)
	}
}

func TestAutoRunner_StopsOnUnexpectedExitCode(t *testing.T) {
	// Exit code 130 (SIGINT) should stop the loop after max consecutive errors
	exec, _ := mockExec([]int{130, 130, 130})
	var stdout, stderr bytes.Buffer
	runner := NewAutoRunner(exec, strings.NewReader(""), &stdout, &stderr)

	code := runner.Run("")
	if code != 130 {
		t.Errorf("expected exit 130, got %d", code)
	}
}

// filterCalls returns calls whose first arg matches the given command.
func filterCalls(calls []mockCall, command string) []mockCall {
	var filtered []mockCall
	for _, c := range calls {
		if len(c.Args) > 0 && c.Args[0] == command {
			filtered = append(filtered, c)
		}
	}
	return filtered
}
