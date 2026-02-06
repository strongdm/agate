package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/strongdm/agate/internal/logging"
)

// MultiAgent runs multiple agents in parallel
type MultiAgent struct {
	agents []Agent
}

// NewMultiAgent creates a new multi-agent executor
func NewMultiAgent(agents []Agent) *MultiAgent {
	return &MultiAgent{agents: agents}
}

// ExecuteAll runs the prompt on all agents in parallel and returns all results
func (m *MultiAgent) ExecuteAll(ctx context.Context, prompt string, workDir string) []Result {
	results := make([]Result, len(m.agents))
	var wg sync.WaitGroup

	for i, agent := range m.agents {
		wg.Add(1)
		go func(idx int, a Agent) {
			defer wg.Done()
			output, err := a.Execute(ctx, prompt, workDir)
			results[idx] = Result{
				AgentName: a.Name(),
				Output:    output,
				Error:     err,
			}
		}(i, agent)
	}

	wg.Wait()
	return results
}

// ExecuteFirst runs the prompt on all agents and returns the first successful result
func (m *MultiAgent) ExecuteFirst(ctx context.Context, prompt string, workDir string) (Result, error) {
	results := m.ExecuteAll(ctx, prompt, workDir)

	// Return first successful result
	for _, r := range results {
		if r.Error == nil {
			return r, nil
		}
	}

	// All failed, return combined error
	var errs []string
	for _, r := range results {
		if r.Error != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", r.AgentName, r.Error))
		}
	}
	return Result{}, fmt.Errorf("all agents failed:\n%s", strings.Join(errs, "\n"))
}

// MergeResults combines outputs from multiple agents
func MergeResults(results []Result) string {
	var parts []string

	for _, r := range results {
		if r.Error == nil && r.Output != "" {
			// Capitalize first letter of agent name
			name := r.AgentName
			if len(name) > 0 {
				name = strings.ToUpper(name[:1]) + name[1:]
			}
			parts = append(parts, fmt.Sprintf("## %s's Analysis\n\n%s", name, r.Output))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// GetSuccessfulResults filters results to only successful ones
func GetSuccessfulResults(results []Result) []Result {
	var successful []Result
	for _, r := range results {
		if r.Error == nil {
			successful = append(successful, r)
		}
	}
	return successful
}

// CountSuccessful returns the number of successful results
func CountSuccessful(results []Result) int {
	count := 0
	for _, r := range results {
		if r.Error == nil {
			count++
		}
	}
	return count
}

// ExecuteAllWithLogging runs the prompt on all agents in parallel with logging
func (m *MultiAgent) ExecuteAllWithLogging(ctx context.Context, prompt string, workDir string, opts ExecuteOptions) []Result {
	results := make([]Result, len(m.agents))
	var wg sync.WaitGroup

	for i, agent := range m.agents {
		wg.Add(1)
		go func(idx int, a Agent) {
			defer wg.Done()
			// Each agent gets its own logging context
			agentOpts := opts
			results[idx] = ExecuteWithLogging(ctx, a, prompt, workDir, agentOpts)
		}(i, agent)
	}

	wg.Wait()
	return results
}

// ExecuteFirstWithLogging runs the prompt on all agents and returns the first successful result with logging
func (m *MultiAgent) ExecuteFirstWithLogging(ctx context.Context, prompt string, workDir string, opts ExecuteOptions) (Result, error) {
	results := m.ExecuteAllWithLogging(ctx, prompt, workDir, opts)

	// Return first successful result
	for _, r := range results {
		if r.Error == nil {
			return r, nil
		}
	}

	// All failed, return combined error
	var errs []string
	for _, r := range results {
		if r.Error != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", r.AgentName, r.Error))
		}
	}
	return Result{}, fmt.Errorf("all agents failed:\n%s", strings.Join(errs, "\n"))
}

// ExecuteOnAgentWithLogging runs the prompt on a specific agent by name with logging
func ExecuteOnAgentWithLogging(ctx context.Context, agentName string, prompt string, workDir string, logger *logging.Logger, opts ExecuteOptions) (Result, error) {
	agents := GetAvailableAgents()
	for _, a := range agents {
		if a.Name() == agentName {
			opts.Logger = logger
			return ExecuteWithLogging(ctx, a, prompt, workDir, opts), nil
		}
	}
	return Result{}, fmt.Errorf("agent %q not found", agentName)
}
