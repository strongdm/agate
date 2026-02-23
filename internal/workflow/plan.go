package workflow

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/strongdm/agate/internal/agent"
	"github.com/strongdm/agate/internal/logging"
	"github.com/strongdm/agate/internal/project"
)

// Result represents the result of a workflow operation
type Result struct {
	Message  string
	MoreWork bool
}

// PlanOptions contains options for the Plan workflow
type PlanOptions struct {
	// StreamOutput enables streaming agent output to this writer
	StreamOutput io.Writer
	// PreferredAgent overrides automatic agent selection
	PreferredAgent string
}

// PlanPhase represents the current planning phase
type PlanPhase string

const (
	PhaseInterview PlanPhase = "interview"
	PhaseDesign    PlanPhase = "design"
	PhaseDecisions PlanPhase = "decisions"
	PhaseSprint    PlanPhase = "sprint"
	PhaseExecution PlanPhase = "execution"
)

// InterviewPath returns the path to the interview file
func InterviewPath(projectDir string) string {
	return filepath.Join(projectDir, ".ai", "interview.md")
}

// GetCurrentPlanPhase determines the current planning phase based on existing files
// Uses GetStatus(fs.FS) for detection
func GetCurrentPlanPhase(projectDir string) PlanPhase {
	fsys := os.DirFS(projectDir)
	result := GetStatus(fsys)
	return result.Phase
}

// GetNextPlanAction returns a human-readable description of the next planning action
func GetNextPlanAction(phase PlanPhase) string {
	switch phase {
	case PhaseInterview:
		return "Generate interview questions"
	case PhaseDesign:
		return "Generate design overview"
	case PhaseDecisions:
		return "Generate technical decisions"
	case PhaseSprint:
		return "Generate sprint plan"
	case PhaseExecution:
		return "Execute sprint tasks"
	default:
		return "Unknown phase"
	}
}

// ExecutePlanPhase executes a single planning phase and returns
func ExecutePlanPhase(projectDir string, opts PlanOptions) (*Result, error) {
	proj := project.New(projectDir)

	// Check for GOAL.md
	if !proj.HasGoal() {
		return nil, fmt.Errorf("GOAL.md not found. Create a GOAL.md file describing what you want to build")
	}

	// Ensure directories exist
	if err := proj.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	// Check for available agents
	if err := agent.EnsureAgentsAvailable(); err != nil {
		return nil, fmt.Errorf("%w\n\n%s", err, agent.FormatInstallInstructions())
	}

	phase := GetCurrentPlanPhase(projectDir)

	switch phase {
	case PhaseInterview:
		return executeInterviewPhase(projectDir, proj, opts)
	case PhaseDesign:
		return executeDesignPhase(projectDir, proj, opts)
	case PhaseDecisions:
		return executeDecisionsPhase(projectDir, proj, opts)
	case PhaseSprint:
		return executeSprintPhase(projectDir, proj, opts)
	case PhaseExecution:
		return &Result{
			Message:  "Planning complete. Run 'agate next' to execute sprint tasks.",
			MoreWork: true,
		}, nil
	}

	return nil, fmt.Errorf("unknown planning phase: %s", phase)
}

func getSelectedAgent(opts PlanOptions) agent.Agent {
	if opts.PreferredAgent != "" {
		a := agent.GetAgentByName(opts.PreferredAgent)
		if a != nil && a.Available() {
			return a
		}
	}
	agents := agent.GetAvailableAgents()
	if len(agents) > 0 {
		return agents[0]
	}
	return nil
}

func executeInterviewPhase(projectDir string, proj *project.Project, opts PlanOptions) (*Result, error) {
	interviewPath := InterviewPath(projectDir)

	// Check if interview exists and is pending answers
	if fileExists(interviewPath) {
		content, err := os.ReadFile(interviewPath)
		if err == nil && !logging.ParseInterviewStatus(string(content)) {
			return &Result{
				Message:  fmt.Sprintf("Interview questions pending. Please answer the questions in:\n  %s\n\nCheck the completion box at the bottom when done, then run 'agate next' again.", interviewPath),
				MoreWork: true,
			}, nil
		}
		// Interview already complete, move to next phase
		return &Result{
			Message:  "Interview complete. Run 'agate next' to generate design.",
			MoreWork: true,
		}, nil
	}

	// Parse the goal
	goal, err := project.ParseGoal(proj.GoalPath())
	if err != nil {
		return nil, fmt.Errorf("failed to parse GOAL.md: %w", err)
	}

	// Get agent and logger
	selectedAgent := getSelectedAgent(opts)
	if selectedAgent == nil {
		return nil, agent.NoAgentsError{}
	}

	logger := logging.NewLogger(projectDir, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Generate interview questions
	interviewPrompt := buildInterviewPrompt(goal)
	execResult := agent.ExecuteWithLogging(ctx, selectedAgent, interviewPrompt, projectDir, agent.ExecuteOptions{
		Logger:        logger,
		Phase:         "interview",
		Task:          "Generate project interview questions",
		TaskIndex:     0,
		Skill:         "_interviewer",
		PromptSummary: "Generating interview questions",
		StreamWriter:  opts.StreamOutput,
		SafeMode:      true, // Planning phase - no file writes needed
	})

	if execResult.Error != nil {
		return nil, fmt.Errorf("failed to generate interview: %w", execResult.Error)
	}

	// Parse and write interview questions
	questions := parseInterviewQuestionsFromResponse(execResult.Output)
	if len(questions) == 0 {
		return nil, fmt.Errorf("no interview questions generated")
	}

	interviewContent := logging.FormatInterview(questions)
	if err := os.WriteFile(interviewPath, []byte(interviewContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write interview: %w", err)
	}

	return &Result{
		Message:  fmt.Sprintf("Interview questions generated. Please answer the questions in:\n  %s\n\nCheck the completion box when done, then run 'agate next' again.", interviewPath),
		MoreWork: true,
	}, nil
}

func executeDesignPhase(projectDir string, proj *project.Project, opts PlanOptions) (*Result, error) {
	// Parse the goal
	goal, err := project.ParseGoal(proj.GoalPath())
	if err != nil {
		return nil, fmt.Errorf("failed to parse GOAL.md: %w", err)
	}

	// Load interview answers
	interviewPath := InterviewPath(projectDir)
	var interviewAnswers map[string]string
	if content, err := os.ReadFile(interviewPath); err == nil {
		interviewAnswers = logging.ParseInterviewAnswers(string(content))
	}
	interviewContext := formatInterviewContext(interviewAnswers)

	// Get agent
	selectedAgent := getSelectedAgent(opts)
	if selectedAgent == nil {
		return nil, agent.NoAgentsError{}
	}

	logger := logging.NewLogger(projectDir, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Generate design overview
	overviewPath := filepath.Join(proj.DesignDir(), "overview.md")
	designPrompt := buildDesignPromptWithContext(goal, interviewContext, overviewPath)
	execResult := agent.ExecuteWithLogging(ctx, selectedAgent, designPrompt, projectDir, agent.ExecuteOptions{
		Logger:        logger,
		Phase:         "design",
		Task:          "Generate design overview",
		TaskIndex:     1,
		Skill:         "_planner",
		PromptSummary: "Generating design overview",
		StreamWriter:  opts.StreamOutput,
	})

	if execResult.Error != nil {
		return nil, fmt.Errorf("failed to generate design: %w", execResult.Error)
	}

	// Verify the agent wrote valid content
	if err := validateMarkdownContent(overviewPath); err != nil {
		return nil, err
	}

	return &Result{
		Message:  "Design overview generated. Run 'agate next' to generate technical decisions.",
		MoreWork: true,
	}, nil
}

func executeDecisionsPhase(projectDir string, proj *project.Project, opts PlanOptions) (*Result, error) {
	// Parse the goal
	goal, err := project.ParseGoal(proj.GoalPath())
	if err != nil {
		return nil, fmt.Errorf("failed to parse GOAL.md: %w", err)
	}

	// Load design
	designPath := filepath.Join(proj.DesignDir(), "overview.md")
	designContent, err := os.ReadFile(designPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read design: %w", err)
	}

	// Get agent
	selectedAgent := getSelectedAgent(opts)
	if selectedAgent == nil {
		return nil, agent.NoAgentsError{}
	}

	logger := logging.NewLogger(projectDir, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Generate decisions
	decisionsPath := filepath.Join(proj.DesignDir(), "decisions.md")
	decisionsPrompt := buildDecisionsPrompt(goal, string(designContent), decisionsPath)
	execResult := agent.ExecuteWithLogging(ctx, selectedAgent, decisionsPrompt, projectDir, agent.ExecuteOptions{
		Logger:        logger,
		Phase:         "decisions",
		Task:          "Generate technical decisions",
		TaskIndex:     2,
		Skill:         "_planner",
		PromptSummary: "Generating technical decisions",
		StreamWriter:  opts.StreamOutput,
	})

	if execResult.Error != nil {
		return nil, fmt.Errorf("failed to generate decisions: %w", execResult.Error)
	}

	// Verify the agent wrote valid content
	if err := validateMarkdownContent(decisionsPath); err != nil {
		return nil, err
	}

	return &Result{
		Message:  "Technical decisions generated. Run 'agate next' to generate sprint plan.",
		MoreWork: true,
	}, nil
}

func executeSprintPhase(projectDir string, proj *project.Project, opts PlanOptions) (*Result, error) {
	// Parse the goal
	goal, err := project.ParseGoal(proj.GoalPath())
	if err != nil {
		return nil, fmt.Errorf("failed to parse GOAL.md: %w", err)
	}

	// Load interview answers for context
	interviewPath := InterviewPath(projectDir)
	var interviewAnswers map[string]string
	if content, err := os.ReadFile(interviewPath); err == nil {
		interviewAnswers = logging.ParseInterviewAnswers(string(content))
	}
	interviewContext := formatInterviewContext(interviewAnswers)

	// Load design
	designPath := filepath.Join(proj.DesignDir(), "overview.md")
	designContent, err := os.ReadFile(designPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read design: %w", err)
	}

	// Get agent
	selectedAgent := getSelectedAgent(opts)
	if selectedAgent == nil {
		return nil, agent.NoAgentsError{}
	}

	logger := logging.NewLogger(projectDir, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Generate skills before sprint prompt so we can use real skill names
	skills := project.GenerateSkills(goal.Language, goal.Type)
	if err := project.WriteSkills(proj.SkillsDir(), skills); err != nil {
		fmt.Printf("%s\n", logging.Yellow(fmt.Sprintf("Warning: failed to write skills: %v", err)))
	}

	var skillNames []string
	for _, s := range skills {
		skillNames = append(skillNames, s.Name)
	}

	// Generate sprint plan
	sprintPath := filepath.Join(proj.SprintsDir(), "01-initial.md")
	sprintPrompt := buildSprintsPromptWithContext(goal, string(designContent), interviewContext, sprintPath, skillNames)
	execResult := agent.ExecuteWithLogging(ctx, selectedAgent, sprintPrompt, projectDir, agent.ExecuteOptions{
		Logger:        logger,
		Phase:         "sprint_plan",
		Task:          "Generate sprint 1 plan",
		TaskIndex:     3,
		Skill:         "_planner",
		PromptSummary: "Generating sprint plan",
		StreamWriter:  opts.StreamOutput,
	})

	if execResult.Error != nil {
		return nil, fmt.Errorf("failed to generate sprint: %w", execResult.Error)
	}

	// Verify the agent wrote valid content
	if err := validateMarkdownContent(sprintPath); err != nil {
		return nil, err
	}

	return &Result{
		Message:  "Sprint plan generated. Run 'agate next' to start implementation.",
		MoreWork: true,
	}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// validateMarkdownContent checks if the file content is actual markdown content
// and not meta-commentary from an agent
func validateMarkdownContent(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("file not found: %s", path)
	}

	text := strings.TrimSpace(string(content))
	if len(text) == 0 {
		return fmt.Errorf("file is empty: %s", path)
	}

	// Check if content looks like meta-commentary instead of actual document
	metaPatterns := []string{
		"I've created ", "I created ", "Created ",
		"Here's ", "Here is ",
		"Perfect! I've ",
		"Done! I've ",
	}

	for _, pattern := range metaPatterns {
		if strings.HasPrefix(text, pattern) {
			return fmt.Errorf("agent wrote meta-commentary instead of document content to %s - retry with 'agate next'", path)
		}
	}

	// Valid document should start with markdown heading
	if !strings.HasPrefix(text, "#") {
		return fmt.Errorf("document doesn't start with markdown heading: %s", path)
	}

	return nil
}

func buildInterviewPrompt(goal *project.Goal) string {
	return fmt.Sprintf(`You are a software architect preparing to design a project. Based on the following goal, generate 3-5 clarifying questions that would help you create a better design.

## Goal

%s

## Instructions

Generate questions that:
1. Clarify ambiguous requirements
2. Identify technology preferences
3. Understand scale and performance needs
4. Clarify integration requirements
5. Understand deployment constraints

Format each question as:
QUESTION: [Brief title]
[The full question text]
OPTIONS: [If applicable, comma-separated options]

Example:
QUESTION: Authentication Method
What authentication method should be used for user login?
OPTIONS: JWT tokens, Session cookies, OAuth 2.0, API keys

Only include OPTIONS if there are specific choices to pick from.
`, goal.Content)
}

func parseInterviewQuestionsFromResponse(response string) []logging.InterviewQuestion {
	var questions []logging.InterviewQuestion

	lines := strings.Split(response, "\n")
	var currentQuestion *logging.InterviewQuestion

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Strip markdown bold wrapping: **QUESTION: ...** → QUESTION: ...
		stripped := strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(line, "**"), "**"), "**")
		stripped = strings.TrimSpace(stripped)

		if strings.HasPrefix(stripped, "QUESTION:") {
			if currentQuestion != nil {
				questions = append(questions, *currentQuestion)
			}
			title := strings.TrimSpace(strings.TrimPrefix(stripped, "QUESTION:"))
			title = strings.TrimSuffix(title, "**")
			title = strings.TrimSpace(title)
			currentQuestion = &logging.InterviewQuestion{
				Title: title,
			}
		} else if strings.HasPrefix(stripped, "OPTIONS:") && currentQuestion != nil {
			optStr := strings.TrimSpace(strings.TrimPrefix(stripped, "OPTIONS:"))
			opts := strings.Split(optStr, ",")
			for _, opt := range opts {
				opt = strings.TrimSpace(opt)
				if opt != "" {
					currentQuestion.Options = append(currentQuestion.Options, opt)
				}
			}
		} else if currentQuestion != nil && line != "" && !strings.HasPrefix(stripped, "QUESTION:") {
			if currentQuestion.Question == "" {
				currentQuestion.Question = line
			} else {
				currentQuestion.Question += " " + line
			}
		}
	}

	if currentQuestion != nil {
		questions = append(questions, *currentQuestion)
	}

	return questions
}

func formatInterviewContext(answers map[string]string) string {
	if len(answers) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Interview Answers\n\n")
	for q, a := range answers {
		sb.WriteString(fmt.Sprintf("**%s**: %s\n\n", q, a))
	}
	return sb.String()
}

func buildDesignPromptWithContext(goal *project.Goal, interviewContext string, outputPath string) string {
	return fmt.Sprintf(`You are a software architect. Based on the following project goal, create a high-level design overview.

## Goal

%s
%s
## Instructions

Create a design document covering:
1. Overview - What this project does and why
2. Architecture - High-level component structure
3. Key Components - Main modules and their responsibilities
4. Data Flow - How data moves through the system
5. Technology Choices - Languages, frameworks, libraries with rationale

Keep it concise but comprehensive. Use markdown formatting.

IMPORTANT: Write the complete document content directly to this file path: %s
Do not create any other files. Do not output any summary or commentary. Just write the document to that exact path.
`, goal.Content, interviewContext, outputPath)
}

// findSkillByPattern returns the first skill name containing the given substring, or fallback if none found.
func findSkillByPattern(skillNames []string, pattern string, fallback string) string {
	for _, name := range skillNames {
		if strings.Contains(name, pattern) {
			return name
		}
	}
	return fallback
}

func buildSprintsPromptWithContext(goal *project.Goal, design string, interviewContext string, outputPath string, skillNames []string) string {
	// Build dynamic skill references from actual generated skills
	coderSkill := findSkillByPattern(skillNames, "coder", "coder")
	reviewerSkill := findSkillByPattern(skillNames, "reviewer", "_reviewer")

	// Build the "Available skills" list: all project skills + _reviewer
	allSkills := make([]string, len(skillNames))
	copy(allSkills, skillNames)
	hasBuiltinReviewer := false
	for _, s := range allSkills {
		if s == "_reviewer" {
			hasBuiltinReviewer = true
			break
		}
	}
	if !hasBuiltinReviewer {
		allSkills = append(allSkills, "_reviewer")
	}
	availableSkills := strings.Join(allSkills, ", ")

	// Build example tasks using real skill names
	examples := fmt.Sprintf(`## Tasks

- [ ] Set up project structure
  - [ ] %s: Initialize project with dependencies
  - [ ] _reviewer: Validate project structure is correct

- [ ] Implement core feature
  - [ ] %s: Write the main logic
  - [ ] %s: Review code quality
  - [ ] _reviewer: Validate feature works correctly`, coderSkill, coderSkill, reviewerSkill)

	return fmt.Sprintf(`You are a project manager. Based on the project goal and design, create the first sprint plan.

## Goal

%s
%s
## Design

%s

## Instructions

Create a sprint document with:
1. Sprint Goal - One sentence describing the objective
2. Tasks - Specific, actionable items with nested sub-tasks
3. Definition of Done - Clear criteria for completion

Focus on getting a minimal working version. Use markdown formatting with NESTED task checkboxes. Each top-level task has sub-tasks indented with 2 spaces, with the skill name followed by a colon:

%s

Important:
- Top-level tasks describe what to accomplish
- Sub-tasks (indented 2 spaces) specify WHICH SKILL does WHAT
- Each sub-task format: "- [ ] skill-name: description"
- End each task with a "_reviewer" sub-task for validation
- Available skills: %s

IMPORTANT: Write the complete sprint document directly to this file path: %s
Do not create any other files. Do not output any summary or commentary. Just write the sprint plan to that exact path.
`, goal.Content, interviewContext, design, examples, availableSkills, outputPath)
}

func buildDecisionsPrompt(goal *project.Goal, design string, outputPath string) string {
	return fmt.Sprintf(`You are a software architect. Based on the project goal and design, document key technical decisions.

## Goal

%s

## Design

%s

## Instructions

Create a technical decisions document (ADR style) covering:
1. Key architectural decisions and their rationale
2. Trade-offs considered
3. Alternatives rejected and why

Keep it focused on the important decisions. Use markdown formatting.

IMPORTANT: Write the complete document content directly to this file path: %s
Do not create any other files. Do not output any summary or commentary. Just write the document to that exact path.
`, goal.Content, design, outputPath)
}
