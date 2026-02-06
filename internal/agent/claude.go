package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// ClaudeAgent implements Agent for Claude CLI
type ClaudeAgent struct {
	cliPath string
}

// NewClaudeAgent creates a new Claude agent
func NewClaudeAgent() *ClaudeAgent {
	path, _ := exec.LookPath("claude")
	return &ClaudeAgent{cliPath: path}
}

// Name returns the agent name
func (a *ClaudeAgent) Name() string {
	return "claude"
}

// Available checks if Claude CLI is installed
func (a *ClaudeAgent) Available() bool {
	return a.cliPath != ""
}

// Execute runs a prompt using Claude CLI
func (a *ClaudeAgent) Execute(ctx context.Context, prompt string, workDir string) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("claude CLI not available")
	}

	// Use claude CLI in YOLO mode (--dangerously-skip-permissions) with --print flag
	cmd := exec.CommandContext(ctx, a.cliPath, "--dangerously-skip-permissions", "--print", "-p", prompt)
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
		return "", fmt.Errorf("claude execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecuteWithStream runs a prompt and streams output to the writer
func (a *ClaudeAgent) ExecuteWithStream(ctx context.Context, prompt string, workDir string, output io.Writer) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("claude CLI not available")
	}

	// Use claude CLI in YOLO mode (--dangerously-skip-permissions) with --print flag
	cmd := exec.CommandContext(ctx, a.cliPath, "--dangerously-skip-permissions", "--print", "-p", prompt)
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
		return "", fmt.Errorf("claude execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecuteSafe runs a prompt without YOLO mode (safe for planning)
func (a *ClaudeAgent) ExecuteSafe(ctx context.Context, prompt string, workDir string) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("claude CLI not available")
	}

	// Use claude CLI WITHOUT --dangerously-skip-permissions
	cmd := exec.CommandContext(ctx, a.cliPath, "--print", "-p", prompt)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("claude execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecuteSafeWithStream runs a prompt in safe mode and streams output
func (a *ClaudeAgent) ExecuteSafeWithStream(ctx context.Context, prompt string, workDir string, output io.Writer) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("claude CLI not available")
	}

	// Use claude CLI WITHOUT --dangerously-skip-permissions
	cmd := exec.CommandContext(ctx, a.cliPath, "--print", "-p", prompt)
	cmd.Dir = workDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer

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
		return "", fmt.Errorf("claude execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}
