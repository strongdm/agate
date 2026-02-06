package agent

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/strongdm/agate/internal/logging"
)

// InvocationContext contains all context needed for a unified agent invocation
type InvocationContext struct {
	Sprint       int
	Phase        string
	TaskIndex    int
	CheckboxText string // Actual task text from sprint file
	Skill        string
	AgentName    string // "claude" or "codex"
}

// ExecutorOptions contains options for the unified executor
type ExecutorOptions struct {
	// Logger for creating log files
	Logger *logging.Logger

	// StreamWriter receives real-time agent output when set (for -tail flag)
	StreamWriter io.Writer

	// WorkDir is the working directory for agent execution
	WorkDir string
}

// ExecutorResult contains the result of an execution
type ExecutorResult struct {
	AgentName string
	Output    string
	Error     error
	LogPath   string
	Duration  time.Duration
}

// Executor provides a unified entry point for all agent invocations
type Executor struct {
	agents map[string]Agent
}

// NewExecutor creates a new executor with available agents
func NewExecutor() *Executor {
	e := &Executor{
		agents: make(map[string]Agent),
	}

	// Register available agents
	claude := NewClaudeAgent()
	if claude.Available() {
		e.agents["claude"] = claude
	}

	codex := NewCodexAgent()
	if codex.Available() {
		e.agents["codex"] = codex
	}

	return e
}

// GetAgent returns an agent by name
func (e *Executor) GetAgent(name string) (Agent, bool) {
	agent, ok := e.agents[name]
	return agent, ok
}

// GetAvailableAgentNames returns names of all available agents
func (e *Executor) GetAvailableAgentNames() []string {
	names := make([]string, 0, len(e.agents))
	for name := range e.agents {
		names = append(names, name)
	}
	return names
}

// HasAgents returns true if at least one agent is available
func (e *Executor) HasAgents() bool {
	return len(e.agents) > 0
}

// Execute runs an agent with unified logging, state tracking, and optional streaming
func (e *Executor) Execute(ctx context.Context, invCtx InvocationContext, prompt string, opts ExecutorOptions) ExecutorResult {
	startTime := time.Now()

	result := ExecutorResult{
		AgentName: invCtx.AgentName,
	}

	// Get the requested agent
	agent, ok := e.agents[invCtx.AgentName]
	if !ok {
		// Try to find any available agent
		for name, a := range e.agents {
			agent = a
			invCtx.AgentName = name
			result.AgentName = name
			break
		}
		if agent == nil {
			result.Error = NoAgentsError{}
			return result
		}
	}

	// Create prompt summary for display (truncate checkbox text)
	promptSummary := invCtx.CheckboxText
	if promptSummary == "" {
		promptSummary = fmt.Sprintf("Task %d", invCtx.TaskIndex)
	}
	if len(promptSummary) > 60 {
		promptSummary = promptSummary[:57] + "..."
	}

	// Start logging if logger is provided
	var logFile *logging.LogFile
	if opts.Logger != nil {
		var err error
		logFile, err = opts.Logger.StartInvocation(
			invCtx.Phase,
			invCtx.CheckboxText,
			invCtx.TaskIndex,
			invCtx.AgentName,
			invCtx.Skill,
			promptSummary,
		)
		if err != nil {
			fmt.Printf("Warning: failed to start log: %v\n", err)
		} else {
			logFile.SetPrompt(prompt)
			result.LogPath = logFile.Path
		}
	}

	// Execute the agent (with streaming if supported and requested)
	var output string
	var execErr error

	if opts.StreamWriter != nil {
		// Try streaming execution if agent supports it
		if streamAgent, ok := agent.(StreamingAgent); ok {
			output, execErr = streamAgent.ExecuteWithStream(ctx, prompt, opts.WorkDir, opts.StreamWriter)
		} else {
			// Fall back to regular execution
			output, execErr = agent.Execute(ctx, prompt, opts.WorkDir)
		}
	} else {
		output, execErr = agent.Execute(ctx, prompt, opts.WorkDir)
	}

	result.Output = output
	result.Error = execErr
	result.Duration = time.Since(startTime)

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

// ExecuteWithAgent is a convenience method that selects the best available agent
func (e *Executor) ExecuteWithAgent(ctx context.Context, invCtx InvocationContext, prompt string, opts ExecutorOptions, preferredAgent string) ExecutorResult {
	// Use preferred agent if specified and available
	if preferredAgent != "" {
		if _, ok := e.agents[preferredAgent]; ok {
			invCtx.AgentName = preferredAgent
		}
	}

	// If no agent specified, prefer codex for implementation
	if invCtx.AgentName == "" {
		if _, ok := e.agents["codex"]; ok {
			invCtx.AgentName = "codex"
		} else if _, ok := e.agents["claude"]; ok {
			invCtx.AgentName = "claude"
		}
	}

	return e.Execute(ctx, invCtx, prompt, opts)
}
