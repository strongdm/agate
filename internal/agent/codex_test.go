package agent

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestCodexAgent_Execute tests the codex agent using a stub binary
func TestCodexAgent_Execute(t *testing.T) {
	// Create a temp directory for our stub
	tmpDir, err := os.MkdirTemp("", "codex-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a stub script that echoes the arguments
	stubPath := filepath.Join(tmpDir, "codex")
	stubScript := `#!/bin/sh
# Stub codex binary for testing
echo "STUB_OUTPUT: args were: $@"
`
	if err := os.WriteFile(stubPath, []byte(stubScript), 0755); err != nil {
		t.Fatalf("failed to write stub: %v", err)
	}

	// Create a codex agent with the stub path
	agent := &CodexAgent{cliPath: stubPath}

	if !agent.Available() {
		t.Fatal("expected agent to be available with stub")
	}

	if agent.Name() != "codex" {
		t.Errorf("expected name 'codex', got '%s'", agent.Name())
	}

	// Execute with a test prompt
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := agent.Execute(ctx, "test prompt", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the stub was called with correct arguments
	if !bytes.Contains([]byte(output), []byte("--full-auto")) {
		t.Errorf("expected --full-auto flag in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("exec")) {
		t.Errorf("expected exec command in output, got: %s", output)
	}
}

// TestCodexAgent_ExecuteWithStream tests streaming output
func TestCodexAgent_ExecuteWithStream(t *testing.T) {
	// Create a temp directory for our stub
	tmpDir, err := os.MkdirTemp("", "codex-stream-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a stub script
	stubPath := filepath.Join(tmpDir, "codex")
	stubScript := `#!/bin/sh
echo "STREAMING_TEST_OUTPUT"
`
	if err := os.WriteFile(stubPath, []byte(stubScript), 0755); err != nil {
		t.Fatalf("failed to write stub: %v", err)
	}

	agent := &CodexAgent{cliPath: stubPath}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var streamBuf bytes.Buffer
	output, err := agent.ExecuteWithStream(ctx, "test prompt", tmpDir, &streamBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify output was captured
	if output == "" {
		t.Error("expected non-empty output")
	}

	// Verify stream received the output
	if streamBuf.Len() == 0 {
		t.Error("expected stream to receive output")
	}
}

// TestCodexAgent_NotAvailable tests behavior when codex is not available
func TestCodexAgent_NotAvailable(t *testing.T) {
	agent := &CodexAgent{cliPath: ""}

	if agent.Available() {
		t.Error("expected agent to not be available with empty path")
	}

	ctx := context.Background()
	_, err := agent.Execute(ctx, "test", "/tmp")
	if err == nil {
		t.Error("expected error when codex not available")
	}
}

// TestCodexAgent_RealBinaryNotRequired tests that tests work without codex installed
func TestCodexAgent_RealBinaryNotRequired(t *testing.T) {
	// This test verifies the stub approach works
	// by checking that we're NOT using the real codex binary

	// First, check if real codex exists
	realPath, _ := exec.LookPath("codex")

	// Create a different stub to prove isolation
	tmpDir, err := os.MkdirTemp("", "codex-isolation-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stubPath := filepath.Join(tmpDir, "codex")
	stubScript := `#!/bin/sh
echo "ISOLATION_TEST_MARKER"
`
	if err := os.WriteFile(stubPath, []byte(stubScript), 0755); err != nil {
		t.Fatalf("failed to write stub: %v", err)
	}

	// Use our stub, not the real binary
	agent := &CodexAgent{cliPath: stubPath}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := agent.Execute(ctx, "test", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we got OUR stub's output, not the real codex
	if !bytes.Contains([]byte(output), []byte("ISOLATION_TEST_MARKER")) {
		t.Errorf("expected stub output, got: %s (real codex path: %s)", output, realPath)
	}
}
