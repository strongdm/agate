package agent

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/strongdm/agate/internal/logging"
)

// Agent represents an AI agent that can execute prompts
type Agent interface {
	// Name returns the agent's name
	Name() string

	// Available checks if the agent is available (CLI installed)
	Available() bool

	// Execute runs a prompt and returns the result
	Execute(ctx context.Context, prompt string, workDir string) (string, error)
}

// StreamingAgent is an agent that supports streaming output
type StreamingAgent interface {
	Agent

	// ExecuteWithStream runs a prompt and streams output to the writer
	// Returns the full output and any error
	ExecuteWithStream(ctx context.Context, prompt string, workDir string, output io.Writer) (string, error)
}

// SafeModeAgent is an agent that supports safe mode execution (no YOLO/file writes)
type SafeModeAgent interface {
	Agent

	// ExecuteSafe runs a prompt without YOLO mode (no --dangerously-skip-permissions)
	// Use for planning phases where file writes are not needed
	ExecuteSafe(ctx context.Context, prompt string, workDir string) (string, error)

	// ExecuteSafeWithStream runs a prompt in safe mode and streams output
	ExecuteSafeWithStream(ctx context.Context, prompt string, workDir string, output io.Writer) (string, error)
}

// Result represents the result of an agent execution
type Result struct {
	AgentName string
	Output    string
	Error     error
	LogPath   string // Path to the log file for this invocation
}

// AgentInfo contains metadata about an agent for display purposes
type AgentInfo struct {
	Name  string
	Model string
	Notes string
}

// agentRegistry maps agent names to their metadata
var agentRegistry = map[string]AgentInfo{
	"haiku":  {Name: "haiku", Model: "Claude 3.5 Haiku", Notes: "Fast, cheap, good for testing"},
	"claude": {Name: "claude", Model: "Claude Opus 4.5", Notes: "Most capable, default"},
	"codex":  {Name: "codex", Model: "GPT 5.2", Notes: "OpenAI alternative"},
	"dummy":  {Name: "dummy", Model: "No-op", Notes: "For workflow testing"},
}

// GetAgentInfo returns metadata for an agent by name
func GetAgentInfo(name string) (AgentInfo, bool) {
	info, ok := agentRegistry[name]
	return info, ok
}

// GetAllAgentInfo returns metadata for all known agents
func GetAllAgentInfo() []AgentInfo {
	return []AgentInfo{
		agentRegistry["claude"],
		agentRegistry["haiku"],
		agentRegistry["codex"],
		agentRegistry["dummy"],
	}
}

// ExecuteOptions contains options for agent execution with logging
type ExecuteOptions struct {
	Logger        *logging.Logger
	Phase         string
	Task          string
	TaskIndex     int
	Skill         string
	PromptSummary string
	// StreamWriter for real-time output streaming (optional, for -tail flag)
	StreamWriter io.Writer
	// SafeMode disables YOLO mode (--dangerously-skip-permissions) for agents
	// Use this for planning phases where file writes are not needed
	SafeMode bool
}

// CheckCLI checks if a CLI tool is available
func CheckCLI(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// GetAvailableAgents returns all available agents
func GetAvailableAgents() []Agent {
	var agents []Agent

	// Claude first (default agent)
	claude := NewClaudeAgent()
	if claude.Available() {
		agents = append(agents, claude)
	}

	// Haiku second (fast, cheap alternative)
	haiku := NewHaikuAgent()
	if haiku.Available() {
		agents = append(agents, haiku)
	}

	codex := NewCodexAgent()
	if codex.Available() {
		agents = append(agents, codex)
	}

	// Always include dummy agent for testing
	agents = append(agents, NewDummyAgent())

	return agents
}

// GetAgentByName returns a specific agent by name
func GetAgentByName(name string) Agent {
	switch name {
	case "haiku":
		return NewHaikuAgent()
	case "claude":
		return NewClaudeAgent()
	case "codex":
		return NewCodexAgent()
	case "dummy":
		return NewDummyAgent()
	default:
		return nil
	}
}

// GetAgentNames returns the names of available agents
func GetAgentNames() []string {
	agents := GetAvailableAgents()
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name()
	}
	return names
}

// NoAgentsError is returned when no agents are available
type NoAgentsError struct{}

func (e NoAgentsError) Error() string {
	return "no AI agents available. Please install claude CLI (https://claude.ai/cli) or codex CLI (https://openai.com/codex)"
}

// EnsureAgentsAvailable returns an error if no agents are available
func EnsureAgentsAvailable() error {
	if len(GetAvailableAgents()) == 0 {
		return NoAgentsError{}
	}
	return nil
}

// FormatInstallInstructions returns instructions for installing agents
func FormatInstallInstructions() string {
	return `No AI agents found. Please install at least one:

Claude CLI:
  npm install -g @anthropic-ai/claude-code
  claude auth login

Codex CLI:
  npm install -g @openai/codex
  codex auth login
`
}

// ExecuteWithLogging runs an agent with full logging support
func ExecuteWithLogging(ctx context.Context, agent Agent, prompt string, workDir string, opts ExecuteOptions) Result {
	result := Result{
		AgentName: agent.Name(),
	}

	// Start logging if logger is provided
	var logFile *logging.LogFile
	if opts.Logger != nil {
		var err error
		logFile, err = opts.Logger.StartInvocation(
			opts.Phase,
			opts.Task,
			opts.TaskIndex,
			agent.Name(),
			opts.Skill,
			opts.PromptSummary,
		)
		if err != nil {
			// Log error but continue execution
			fmt.Printf("Warning: failed to start log: %v\n", err)
		} else {
			logFile.SetPrompt(prompt)
			result.LogPath = logFile.Path
		}
	}

	// Execute the agent (with streaming if supported and requested)
	var output string
	var execErr error

	// Always create a counting writer to show KB progress
	var baseWriter io.Writer = io.Discard
	if opts.StreamWriter != nil {
		baseWriter = opts.StreamWriter
	}
	countingWriter := NewCountingWriter(baseWriter, true)

	if opts.SafeMode {
		if safeAgent, ok := agent.(SafeModeAgent); ok {
			output, execErr = safeAgent.ExecuteSafeWithStream(ctx, prompt, workDir, countingWriter)
		} else {
			output, execErr = agent.Execute(ctx, prompt, workDir)
		}
	} else {
		if streamAgent, ok := agent.(StreamingAgent); ok {
			output, execErr = streamAgent.ExecuteWithStream(ctx, prompt, workDir, countingWriter)
		} else {
			output, execErr = agent.Execute(ctx, prompt, workDir)
		}
	}
	countingWriter.PrintFinal()

	result.Output = output
	result.Error = execErr

	// Complete logging
	if logFile != nil {
		logFile.SetResponse(output)
		if execErr != nil {
			logFile.SetError(execErr)
		} else {
			logFile.SetStatus("success")
		}
		if closeErr := logFile.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close log: %v\n", closeErr)
		}
	}

	return result
}
