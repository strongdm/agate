package agent

import (
	"testing"
)

func TestCheckCLI(t *testing.T) {
	// "ls" should exist on any Unix system
	if !CheckCLI("ls") {
		t.Error("expected ls to be available")
	}

	// Nonexistent command
	if CheckCLI("nonexistent-agate-test-command-12345") {
		t.Error("expected nonexistent command to not be available")
	}
}

func TestNoAgentsError(t *testing.T) {
	err := NoAgentsError{}
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestMergeResults(t *testing.T) {
	results := []Result{
		{AgentName: "claude", Output: "Claude's output", Error: nil},
		{AgentName: "codex", Output: "Codex's output", Error: nil},
	}

	merged := MergeResults(results)
	if merged == "" {
		t.Error("expected non-empty merged output")
	}

	// Check both agent names appear
	if !contains(merged, "Claude") {
		t.Error("expected Claude in merged output")
	}
	if !contains(merged, "Codex") {
		t.Error("expected Codex in merged output")
	}
}

func TestMergeResults_WithErrors(t *testing.T) {
	results := []Result{
		{AgentName: "claude", Output: "Claude's output", Error: nil},
		{AgentName: "codex", Output: "", Error: &NoAgentsError{}},
	}

	merged := MergeResults(results)
	// Should only include successful result
	if !contains(merged, "Claude") {
		t.Error("expected Claude in merged output")
	}
	if contains(merged, "Codex") {
		t.Error("expected Codex to be excluded from merged output")
	}
}

func TestCountSuccessful(t *testing.T) {
	results := []Result{
		{AgentName: "claude", Error: nil},
		{AgentName: "codex", Error: &NoAgentsError{}},
		{AgentName: "other", Error: nil},
	}

	if got := CountSuccessful(results); got != 2 {
		t.Errorf("expected 2 successful, got %d", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
