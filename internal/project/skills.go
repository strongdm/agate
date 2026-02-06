package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillMetadata contains parsed skill frontmatter
type SkillMetadata struct {
	Name                string   `yaml:"name"`
	Agents              []string `yaml:"agents"`
	Phase               string   `yaml:"phase"`
	CanModifyCheckboxes bool     `yaml:"can_modify_checkboxes"`
	Version             int      `yaml:"version"`
}

// Skill represents a generated skill with metadata
type Skill struct {
	Name     string
	Metadata SkillMetadata
	Content  string
}

// ParseSkillMetadata extracts frontmatter from skill content
func ParseSkillMetadata(content string) (SkillMetadata, string) {
	meta := SkillMetadata{
		Agents:  []string{"claude", "codex"}, // Default to both
		Version: 1,
	}

	// Check for YAML frontmatter
	if !strings.HasPrefix(content, "---\n") {
		return meta, content
	}

	// Find end of frontmatter
	endIdx := strings.Index(content[4:], "\n---")
	if endIdx == -1 {
		return meta, content
	}

	frontmatter := content[4 : endIdx+4]
	body := content[endIdx+8:] // Skip past "\n---\n"

	// Simple YAML parsing (avoiding external dependency for now)
	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			meta.Name = value
		case "phase":
			meta.Phase = value
		case "can_modify_checkboxes":
			meta.CanModifyCheckboxes = value == "true"
		case "version":
			fmt.Sscanf(value, "%d", &meta.Version)
		case "agents":
			// Parse [claude, codex] format
			value = strings.Trim(value, "[]")
			agents := strings.Split(value, ",")
			meta.Agents = nil
			for _, a := range agents {
				a = strings.TrimSpace(a)
				if a != "" {
					meta.Agents = append(meta.Agents, a)
				}
			}
		}
	}

	return meta, strings.TrimPrefix(body, "\n")
}

// FormatSkillWithFrontmatter adds frontmatter to skill content
func FormatSkillWithFrontmatter(meta SkillMetadata, content string) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", meta.Name))
	sb.WriteString(fmt.Sprintf("agents: [%s]\n", strings.Join(meta.Agents, ", ")))
	if meta.Phase != "" {
		sb.WriteString(fmt.Sprintf("phase: %s\n", meta.Phase))
	}
	sb.WriteString(fmt.Sprintf("can_modify_checkboxes: %t\n", meta.CanModifyCheckboxes))
	sb.WriteString(fmt.Sprintf("version: %d\n", meta.Version))
	sb.WriteString("---\n\n")
	sb.WriteString(content)

	return sb.String()
}

// CheckboxDisclaimer is added to non-review skills
const CheckboxDisclaimer = `
**IMPORTANT**: This skill is for implementation only. Do NOT modify sprint
checkboxes or mark tasks as complete. Only the reviewer skill can do that.
`

// AddCheckboxDisclaimer adds the disclaimer if not already present
func AddCheckboxDisclaimer(content string) string {
	if strings.Contains(content, "Do NOT modify sprint") {
		return content
	}
	return content + "\n" + CheckboxDisclaimer
}

// GenerateSkills generates skills based on the detected language and project type
func GenerateSkills(language, projectType string) []Skill {
	var skills []Skill

	// Add language-specific skills
	switch language {
	case "go":
		skills = append(skills, goSkills()...)
	case "python":
		skills = append(skills, pythonSkills()...)
	case "rust":
		skills = append(skills, rustSkills()...)
	case "javascript", "typescript":
		skills = append(skills, jsSkills()...)
	default:
		skills = append(skills, genericSkills()...)
	}

	// Add project-type specific skills
	switch projectType {
	case "cli":
		skills = append(skills, cliSkills()...)
	case "api":
		skills = append(skills, apiSkills()...)
	case "webapp":
		skills = append(skills, webappSkills()...)
	}

	return skills
}

// WriteSkills writes skill files to the skills directory
func WriteSkills(skillsDir string, skills []Skill) error {
	for _, skill := range skills {
		path := filepath.Join(skillsDir, skill.Name+".md")

		// Format with frontmatter
		content := FormatSkillWithFrontmatter(skill.Metadata, skill.Content)

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write skill %s: %w", skill.Name, err)
		}
	}
	return nil
}

// LoadSkill loads a skill from a file
func LoadSkill(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	meta, body := ParseSkillMetadata(string(content))
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	if meta.Name == "" {
		meta.Name = name
	}

	return &Skill{
		Name:     name,
		Metadata: meta,
		Content:  body,
	}, nil
}

// LoadSkills loads all skills from a directory
// User override mechanism: If both _foo.md and foo.md exist, the user's
// foo.md content is appended to the built-in _foo.md content.
func LoadSkills(skillsDir string) ([]Skill, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// First pass: load all skills into a map
	skillMap := make(map[string]*Skill)

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}

		skill, err := LoadSkill(filepath.Join(skillsDir, e.Name()))
		if err != nil {
			continue // Skip invalid skills
		}

		skillMap[skill.Name] = skill
	}

	// Second pass: merge user overrides into built-in skills
	// If both _foo.md and foo.md exist, append user content to builtin
	var toDelete []string
	for name, skill := range skillMap {
		if strings.HasPrefix(name, "_") {
			continue // Skip builtins in this pass
		}

		builtinName := "_" + name
		builtin, hasBuiltin := skillMap[builtinName]
		if hasBuiltin {
			// Append user content to built-in content
			builtin.Content = builtin.Content + "\n\n## User Customizations\n\n" + skill.Content
			// Mark the standalone user override for removal (it's now merged)
			toDelete = append(toDelete, name)
		}
	}
	for _, name := range toDelete {
		delete(skillMap, name)
	}

	// Convert map to slice
	var skills []Skill
	for _, skill := range skillMap {
		skills = append(skills, *skill)
	}

	return skills, nil
}

// GetSkillByName finds a skill by name
func GetSkillByName(skills []Skill, name string) *Skill {
	for i := range skills {
		if skills[i].Name == name {
			return &skills[i]
		}
	}
	return nil
}

// CanAgentUseSkill checks if an agent can use a skill
func CanAgentUseSkill(skill *Skill, agent string) bool {
	if skill == nil {
		return true
	}
	if len(skill.Metadata.Agents) == 0 {
		return true // Default to allowing all agents
	}
	for _, a := range skill.Metadata.Agents {
		if a == agent {
			return true
		}
	}
	return false
}

func goSkills() []Skill {
	return []Skill{
		{
			Name: "go-coder",
			Metadata: SkillMetadata{
				Name:                "go-coder",
				Agents:              []string{"claude", "codex"},
				Phase:               "implement",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Go Coder

You are an expert Go developer. Write idiomatic Go code following these principles:

## Style
- Follow effective Go guidelines
- Use gofmt formatting
- Prefer simplicity over cleverness
- Use meaningful variable names

## Error Handling
- Always check errors
- Wrap errors with context using fmt.Errorf
- Return errors, don't panic

## Patterns
- Use interfaces for abstraction
- Prefer composition over inheritance
- Keep functions small and focused
- Use table-driven tests

## Dependencies
- Prefer standard library when possible
- Minimize external dependencies
- Use go modules for dependency management
` + CheckboxDisclaimer,
		},
		{
			Name: "go-reviewer",
			Metadata: SkillMetadata{
				Name:                "go-reviewer",
				Agents:              []string{"claude", "codex"},
				Phase:               "review",
				CanModifyCheckboxes: true,
				Version:             1,
			},
			Content: `# Go Reviewer

Review Go code for quality and correctness. You MAY mark sprint checkboxes as complete.

## Check For
- Unchecked errors
- Resource leaks (defer for cleanup)
- Race conditions
- Unnecessary complexity
- Missing tests
- Documentation gaps

## Code Quality
- Is the code idiomatic Go?
- Are variable names clear?
- Is error handling consistent?
- Are there any security issues?

## Performance
- Unnecessary allocations
- Inefficient loops
- Missing context cancellation

## Checkbox Updates
When reviewing, you may mark tasks as complete by changing [ ] to [x] in the sprint file
if the task has been fully implemented and passes review.
`,
		},
		{
			Name: "test-writer",
			Metadata: SkillMetadata{
				Name:                "test-writer",
				Agents:              []string{"claude", "codex"},
				Phase:               "implement",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Test Writer

Write comprehensive tests for Go code.

## Principles
- Use table-driven tests
- Test edge cases
- Use meaningful test names
- Keep tests focused and independent

## Structure
- Use t.Run for subtests
- Use t.Helper for helper functions
- Use testify for assertions when helpful
- Mock external dependencies

## Coverage
- Test happy path
- Test error conditions
- Test boundary conditions
- Test concurrent access if applicable
` + CheckboxDisclaimer,
		},
	}
}

func pythonSkills() []Skill {
	return []Skill{
		{
			Name: "python-coder",
			Metadata: SkillMetadata{
				Name:                "python-coder",
				Agents:              []string{"claude", "codex"},
				Phase:               "implement",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Python Coder

Write clean, Pythonic code following these principles:

## Style
- Follow PEP 8
- Use type hints
- Write docstrings for public APIs
- Use meaningful names

## Patterns
- Use context managers for resources
- Prefer list comprehensions when readable
- Use dataclasses for simple data structures
- Handle exceptions appropriately

## Dependencies
- Use virtual environments
- Pin dependencies in requirements.txt
- Prefer standard library when possible
` + CheckboxDisclaimer,
		},
		{
			Name: "python-reviewer",
			Metadata: SkillMetadata{
				Name:                "python-reviewer",
				Agents:              []string{"claude", "codex"},
				Phase:               "review",
				CanModifyCheckboxes: true,
				Version:             1,
			},
			Content: `# Python Reviewer

Review Python code for quality and correctness. You MAY mark sprint checkboxes as complete.

## Check For
- Type hint completeness
- Exception handling
- Resource management
- Security issues
- Missing tests

## Code Quality
- Is the code Pythonic?
- Are there any code smells?
- Is the structure clear?
`,
		},
	}
}

func rustSkills() []Skill {
	return []Skill{
		{
			Name: "rust-coder",
			Metadata: SkillMetadata{
				Name:                "rust-coder",
				Agents:              []string{"claude", "codex"},
				Phase:               "implement",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Rust Coder

Write safe, idiomatic Rust code.

## Principles
- Embrace the ownership model
- Use Result for error handling
- Prefer iterators over loops
- Use clippy suggestions

## Patterns
- Use enums for state machines
- Implement traits for abstraction
- Use modules for organization
- Write documentation with examples
` + CheckboxDisclaimer,
		},
	}
}

func jsSkills() []Skill {
	return []Skill{
		{
			Name: "js-coder",
			Metadata: SkillMetadata{
				Name:                "js-coder",
				Agents:              []string{"claude", "codex"},
				Phase:               "implement",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# JavaScript/TypeScript Coder

Write modern, clean JavaScript/TypeScript code.

## Style
- Use TypeScript for type safety
- Prefer const/let over var
- Use async/await for asynchronous code
- Follow ESLint recommendations

## Patterns
- Use functional programming where appropriate
- Handle promises properly
- Use modules for organization
- Write unit tests with Jest
` + CheckboxDisclaimer,
		},
	}
}

func genericSkills() []Skill {
	return []Skill{
		{
			Name: "coder",
			Metadata: SkillMetadata{
				Name:                "coder",
				Agents:              []string{"claude", "codex"},
				Phase:               "implement",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Coder

Write clean, maintainable code.

## Principles
- Keep functions small and focused
- Use meaningful names
- Write self-documenting code
- Handle errors appropriately
- Follow language conventions

## Quality
- Write tests for your code
- Document public APIs
- Avoid premature optimization
- Keep dependencies minimal
` + CheckboxDisclaimer,
		},
		{
			Name: "reviewer",
			Metadata: SkillMetadata{
				Name:                "reviewer",
				Agents:              []string{"claude", "codex"},
				Phase:               "review",
				CanModifyCheckboxes: true,
				Version:             1,
			},
			Content: `# Reviewer

Review code for quality and correctness. You MAY mark sprint checkboxes as complete.

## Check For
- Logic errors
- Edge cases
- Security issues
- Performance problems
- Missing tests
- Documentation gaps

## Feedback
- Be specific and constructive
- Suggest improvements
- Acknowledge good patterns
`,
		},
	}
}

func cliSkills() []Skill {
	return []Skill{
		{
			Name: "cli-designer",
			Metadata: SkillMetadata{
				Name:                "cli-designer",
				Agents:              []string{"claude", "codex"},
				Phase:               "implement",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# CLI Designer

Design intuitive command-line interfaces.

## Principles
- Follow common CLI conventions
- Use clear, consistent command names
- Provide helpful --help output
- Use meaningful exit codes

## User Experience
- Show progress for long operations
- Handle interrupts gracefully
- Provide clear error messages
- Support both verbose and quiet modes
` + CheckboxDisclaimer,
		},
	}
}

func apiSkills() []Skill {
	return []Skill{
		{
			Name: "api-designer",
			Metadata: SkillMetadata{
				Name:                "api-designer",
				Agents:              []string{"claude", "codex"},
				Phase:               "implement",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# API Designer

Design clean, RESTful APIs.

## Principles
- Use appropriate HTTP methods
- Return meaningful status codes
- Use consistent naming conventions
- Version your API

## Security
- Validate all input
- Use authentication/authorization
- Rate limit endpoints
- Log security-relevant events
` + CheckboxDisclaimer,
		},
	}
}

func webappSkills() []Skill {
	return []Skill{
		{
			Name: "frontend-developer",
			Metadata: SkillMetadata{
				Name:                "frontend-developer",
				Agents:              []string{"claude", "codex"},
				Phase:               "implement",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Frontend Developer

Build responsive, accessible web interfaces.

## Principles
- Write semantic HTML
- Use CSS efficiently
- Ensure accessibility (WCAG)
- Optimize performance

## User Experience
- Handle loading states
- Show meaningful errors
- Support keyboard navigation
- Test across browsers
` + CheckboxDisclaimer,
		},
	}
}

// BuiltinSkills returns the agate internal operation skills
// These are prefixed with _ and regenerated on every startup
func BuiltinSkills() []Skill {
	return []Skill{
		{
			Name: "_agents",
			Metadata: SkillMetadata{
				Name:                "_agents",
				Agents:              []string{"claude", "codex"},
				Phase:               "reference",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Available Agents

This document describes the AI agents available for use in agate workflows.

## Agent Selection Guidelines

When planning tasks, choose agents based on task complexity and type:
- **Planning, reviewing, complex reasoning**: Use claude
- **Difficult coding tasks**: Use codex
- **Simple/trivial tasks**: Use haiku (fast, cheap)
- **Testing workflow only**: Use dummy (never for real work)

## Agents

### claude
- **Model**: Claude (Opus/Sonnet)
- **Best for**: General purpose thinking, planning, design decisions, code review, complex reasoning
- **Strengths**: Strong at understanding context, following nuanced instructions, explaining rationale
- **Use when**: Task requires judgment, planning, review, or understanding complex requirements

### codex
- **Model**: OpenAI Codex/GPT
- **Best for**: Difficult coding efforts, implementation tasks, code generation
- **Strengths**: Strong at generating working code, handling complex implementations
- **Use when**: Task is primarily writing substantial code

### haiku
- **Model**: Claude 3.5 Haiku
- **Best for**: Extremely trivial tasks, quick lookups, simple transformations
- **Strengths**: Fast, cheap, good for high-volume simple tasks
- **Use when**: Task is trivial and speed/cost matters more than sophistication
- **Avoid when**: Task requires nuance, complex reasoning, or high-quality output

### dummy
- **Model**: No-op (returns mock responses)
- **Purpose**: Testing the agate workflow only
- **NEVER use for**: Real work - it does not execute anything
- **Use when**: Testing workflow mechanics, CI/CD validation
`,
		},
		{
			Name: "_interviewer",
			Metadata: SkillMetadata{
				Name:                "_interviewer",
				Agents:              []string{"claude"},
				Phase:               "planning",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Interviewer

You conduct project interviews to clarify requirements and design decisions.

## Purpose
- Gather essential project context
- Identify ambiguities in GOAL.md
- Clarify technology choices
- Understand constraints and preferences

## Interview Style
- Ask focused, specific questions
- Provide multiple choice options when appropriate
- Limit questions to the essential (5-10 questions max)
- Group related questions logically

## Output Format
Generate questions in this format:
- Title: Brief topic summary
- Question: Clear question text
- Options: If multiple choice, list options

Do not make assumptions - ask when unclear.
`,
		},
		{
			Name: "_planner",
			Metadata: SkillMetadata{
				Name:                "_planner",
				Agents:              []string{"claude", "codex"},
				Phase:               "planning",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Planner

You create sprint plans and task breakdowns for software projects.

## Purpose
- Break down large goals into manageable sprints
- Create clear, actionable task lists
- Estimate complexity and dependencies
- Sequence work appropriately

## Planning Principles
- Each sprint should be completable in reasonable time
- Tasks should be specific and testable
- Consider dependencies between tasks
- Include testing and review steps

## Task Format
- Use checkbox format: - [ ] Task description
- Be specific about what "done" means
- Include file paths when relevant
- Note which skill should handle each task
`,
		},
		{
			Name: "_reviewer",
			Metadata: SkillMetadata{
				Name:                "_reviewer",
				Agents:              []string{"claude"},
				Phase:               "review",
				CanModifyCheckboxes: true,
				Version:             1,
			},
			Content: `# Reviewer

You review sprint implementation for completeness and quality.

## Purpose
- Verify sprint goals are met
- Check code quality and correctness
- Identify issues before moving forward
- Mark tasks as complete when done

## Review Process
1. Check each task in the sprint plan
2. Verify the implementation matches requirements
3. Look for obvious bugs or issues
4. Mark completed tasks with [x]

## Checkbox Updates
You MAY mark tasks as complete by changing [ ] to [x] in sprint files
when the task has been fully implemented and passes review.

## Output
- SPRINT_COMPLETE if all goals met
- ISSUES_FOUND with description if problems exist
`,
		},
		{
			Name: "_recover",
			Metadata: SkillMetadata{
				Name:                "_recover",
				Agents:              []string{"claude"},
				Phase:               "recover",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Recovery Agent

You are a recovery agent. A previous agent execution failed with an error.
Your job is to diagnose and fix the root cause so the task can be retried.

## Common Issues
- Corrupted or binary files that confuse the AI agent (e.g. PNG, binary data committed as text)
- Missing dependencies or build artifacts
- Permission issues on files or directories
- Invalid or malformed configuration files
- Syntax errors in generated code from a previous failed attempt

## Instructions
1. Read the error message carefully to understand what went wrong
2. Investigate the project files to find the root cause
3. Fix the environment issue (delete bad files, fix configs, install deps, etc.)
4. Do NOT attempt to complete the original task — just fix the environment
5. Keep changes minimal — only fix what caused the error

## Important
- You are fixing the environment, not implementing features
- If you cannot determine the root cause, say so clearly
- Do not modify sprint files or checkboxes
`,
		},
		{
			Name: "_replanner",
			Metadata: SkillMetadata{
				Name:                "_replanner",
				Agents:              []string{"claude"},
				Phase:               "replan",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Sprint Replanner

You are a sprint replanner. A reviewer repeatedly rejected a task because the sprint plan is structurally wrong (e.g., tasks in the wrong order, missing prerequisites, impossible steps). Your job is to rewrite the failing task's subtasks so work can proceed.

## Instructions

1. Read the sprint file and understand the full context
2. Look at the reviewer's feedback to understand what's structurally wrong
3. Rewrite ONLY the failing task's subtasks — do NOT touch other tasks
4. Keep the same top-level task text, just fix the subtask breakdown
5. Uncheck any checked subtasks in the rewritten task
6. Preserve existing skill assignments (go-coder, _reviewer, etc.) or reassign as needed
7. Edit the sprint file directly

## Important

- Do NOT modify other tasks in the sprint
- Do NOT modify sprint checkboxes on other tasks
- Keep subtask format: "  - [ ] skill-name: description"
- The goal is to make the task achievable, not to change what it does
`,
		},
		{
			Name: "_retro",
			Metadata: SkillMetadata{
				Name:                "_retro",
				Agents:              []string{"claude"},
				Phase:               "retrospective",
				CanModifyCheckboxes: false,
				Version:             1,
			},
			Content: `# Retrospective Facilitator

You conduct sprint retrospectives to improve future work.

## Purpose
- Analyze what went well
- Identify what could improve
- Extract lessons learned
- Suggest skill updates

## Retrospective Format
1. Review sprint goals vs outcomes
2. Analyze agent logs for patterns
3. Identify recurring issues
4. Suggest concrete improvements

## Skill Evolution
When patterns emerge, suggest updates to skills:
- New guidelines based on what worked
- Warnings based on common mistakes
- Better prompts for similar tasks

## Output
Generate a structured retrospective with:
- Summary of sprint
- What went well
- What to improve
- Skill update suggestions
`,
		},
	}
}

// EnsureBuiltinSkills writes all built-in skills to the skills directory
// This is called on every agate command to ensure fresh built-ins
func EnsureBuiltinSkills(skillsDir string) error {
	// Ensure directory exists
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Errorf("failed to create skills directory: %w", err)
	}

	builtins := BuiltinSkills()
	for _, skill := range builtins {
		path := filepath.Join(skillsDir, skill.Name+".md")
		content := FormatSkillWithFrontmatter(skill.Metadata, skill.Content)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write builtin skill %s: %w", skill.Name, err)
		}
	}

	return nil
}

// IsBuiltinSkill checks if a skill name is a built-in (has _ prefix)
func IsBuiltinSkill(name string) bool {
	return strings.HasPrefix(name, "_")
}
