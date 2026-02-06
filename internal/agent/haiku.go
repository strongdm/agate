package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// HaikuAgent implements Agent for Claude CLI with Haiku model
// This is a fast, cheap agent good for testing and iteration
type HaikuAgent struct {
	cliPath string
}

// NewHaikuAgent creates a new Haiku agent
func NewHaikuAgent() *HaikuAgent {
	path, _ := exec.LookPath("claude")
	return &HaikuAgent{cliPath: path}
}

// Name returns the agent name
func (a *HaikuAgent) Name() string {
	return "haiku"
}

// Available checks if Claude CLI is installed (haiku uses same CLI)
func (a *HaikuAgent) Available() bool {
	return a.cliPath != ""
}

// Execute runs a prompt using Claude CLI with haiku model
func (a *HaikuAgent) Execute(ctx context.Context, prompt string, workDir string) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("claude CLI not available")
	}

	// Use claude CLI in YOLO mode with --model haiku flag
	cmd := exec.CommandContext(ctx, a.cliPath, "--dangerously-skip-permissions", "--model", "haiku", "--print", "-p", prompt)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("haiku execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecuteWithStream runs a prompt and streams output to the writer
func (a *HaikuAgent) ExecuteWithStream(ctx context.Context, prompt string, workDir string, output io.Writer) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("claude CLI not available")
	}

	// Use claude CLI in YOLO mode with --model haiku flag
	cmd := exec.CommandContext(ctx, a.cliPath, "--dangerously-skip-permissions", "--model", "haiku", "--print", "-p", prompt)
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
		return "", fmt.Errorf("haiku execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecuteSafe runs a prompt without YOLO mode (safe for planning)
func (a *HaikuAgent) ExecuteSafe(ctx context.Context, prompt string, workDir string) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("claude CLI not available")
	}

	// Use claude CLI WITHOUT --dangerously-skip-permissions
	cmd := exec.CommandContext(ctx, a.cliPath, "--model", "haiku", "--print", "-p", prompt)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("haiku execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecuteSafeWithStream runs a prompt in safe mode and streams output
func (a *HaikuAgent) ExecuteSafeWithStream(ctx context.Context, prompt string, workDir string, output io.Writer) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("claude CLI not available")
	}

	// Use claude CLI WITHOUT --dangerously-skip-permissions
	cmd := exec.CommandContext(ctx, a.cliPath, "--model", "haiku", "--print", "-p", prompt)
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
		return "", fmt.Errorf("haiku execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}
