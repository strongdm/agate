package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// CodexAgent implements Agent for Codex CLI
type CodexAgent struct {
	cliPath string
}

// NewCodexAgent creates a new Codex agent
func NewCodexAgent() *CodexAgent {
	path, _ := exec.LookPath("codex")
	return &CodexAgent{cliPath: path}
}

// Name returns the agent name
func (a *CodexAgent) Name() string {
	return "codex"
}

// Available checks if Codex CLI is installed
func (a *CodexAgent) Available() bool {
	return a.cliPath != ""
}

// Execute runs a prompt using Codex CLI
func (a *CodexAgent) Execute(ctx context.Context, prompt string, workDir string) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("codex CLI not available")
	}

	// Use codex CLI in full-auto mode
	cmd := exec.CommandContext(ctx, a.cliPath, "--full-auto", "exec", prompt)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Check if it's a context error
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("codex execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecuteWithStream runs a prompt and streams output to the writer
func (a *CodexAgent) ExecuteWithStream(ctx context.Context, prompt string, workDir string, output io.Writer) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("codex CLI not available")
	}

	// Use codex CLI in full-auto mode
	cmd := exec.CommandContext(ctx, a.cliPath, "--full-auto", "exec", prompt)
	cmd.Dir = workDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Tee stdout to both the buffer (for return) and the output writer (for streaming)
	if output != nil {
		cmd.Stdout = io.MultiWriter(&stdout, output)
	} else {
		cmd.Stdout = &stdout
	}
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("codex execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}
