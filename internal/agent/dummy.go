package agent

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// DummyAgent is a no-op agent for testing workflows without invoking real LLMs
type DummyAgent struct{}

// NewDummyAgent creates a new dummy agent
func NewDummyAgent() *DummyAgent {
	return &DummyAgent{}
}

// Name returns the agent name
func (a *DummyAgent) Name() string {
	return "dummy"
}

// Available always returns true for dummy agent
func (a *DummyAgent) Available() bool {
	return true
}

// Execute returns a simple OK response
func (a *DummyAgent) Execute(ctx context.Context, prompt string, workDir string) (string, error) {
	// Extract what kind of task this is from the prompt for more realistic output
	// Use very specific patterns and check INSTRUCTION-based patterns first
	response := "OK - Dummy agent completed task"
	promptLower := strings.ToLower(prompt)

	// Check instruction-level patterns first (these are in ## Instructions section)
	if strings.Contains(promptLower, "generate questions that") || strings.Contains(promptLower, "generate 3-5 clarifying") {
		response = `QUESTION: Project Type
What type of project is this?
OPTIONS: CLI, Web App, Library, API

QUESTION: Testing
What testing approach?
OPTIONS: Unit tests, Integration tests, Both, None`
	} else if strings.Contains(promptLower, "create a sprint document") || strings.Contains(promptLower, "nested task checkboxes") {
		// Sprint generation
		response = `# Sprint 1

## Goal
Implement the core functionality.

## Tasks

- [ ] Set up project structure
  - [ ] go-coder: Create main.go with basic structure
  - [ ] _reviewer: Validate setup complete

- [ ] Implement core feature
  - [ ] go-coder: Write the main logic
  - [ ] go-reviewer: Review for Go idioms
  - [ ] _reviewer: Validate feature complete
`
	} else if strings.Contains(promptLower, "technical decisions document") || strings.Contains(promptLower, "adr style") {
		response = `# Technical Decisions

## Decision 1: Language Choice
Go was chosen for its simplicity and performance.

## Decision 2: No Dependencies
Using stdlib only keeps the project simple.
`
	} else if strings.Contains(promptLower, "create a design document") || strings.Contains(promptLower, "high-level component structure") {
		response = `# Design Overview

This is a dummy design document.

## Architecture
- Component A
- Component B

## Technology
- Go for implementation
`
	} else if strings.Contains(promptLower, "output any files") || strings.Contains(promptLower, "### file:") {
		// Implementation task
		response = `### File: main.go
` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
` + "```" + `

Implementation complete.
`
	} else if strings.Contains(promptLower, "if the implementation is good") || strings.Contains(promptLower, "respond with: approved") {
		response = "APPROVED - All requirements met."
	}

	return response, nil
}

// ExecuteWithStream implements StreamingAgent for dummy
func (a *DummyAgent) ExecuteWithStream(ctx context.Context, prompt string, workDir string, output io.Writer) (string, error) {
	result, err := a.Execute(ctx, prompt, workDir)
	if err != nil {
		return "", err
	}

	// Write to stream if provided
	if output != nil {
		fmt.Fprintln(output, result)
	}

	return result, nil
}
