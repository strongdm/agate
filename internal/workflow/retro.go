package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/strongdm/agate/internal/agent"
	"github.com/strongdm/agate/internal/logging"
	"github.com/strongdm/agate/internal/project"
)

// RetroOptions contains options for the retrospective
type RetroOptions struct {
	// UserInput is optional user feedback to include in the retrospective
	UserInput string
}

// RunRetrospective runs a retrospective for the completed sprint
func RunRetrospective(projectDir string, sprintNumber int) (*Result, error) {
	return RunRetrospectiveWithOptions(projectDir, sprintNumber, RetroOptions{})
}

// RunRetrospectiveWithOptions runs a retrospective with options
func RunRetrospectiveWithOptions(projectDir string, sprintNumber int, opts RetroOptions) (*Result, error) {
	fmt.Printf("Running retrospective for sprint %d...\n", sprintNumber)

	// Check if retro already done for this sprint (by file existence)
	retroPath := logging.GetRetroPath(projectDir, sprintNumber)
	if fileExists(retroPath) {
		return &Result{
			Message:  fmt.Sprintf("Retrospective already completed for sprint %d", sprintNumber),
			MoreWork: false,
		}, nil
	}

	// Read all logs from the sprint
	logs, err := logging.ListLogs(projectDir, sprintNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to list logs: %w", err)
	}

	if len(logs) == 0 {
		return &Result{
			Message:  fmt.Sprintf("No logs found for sprint %d, skipping retrospective", sprintNumber),
			MoreWork: false,
		}, nil
	}

	// Collect log summaries
	var logSummaries []string
	for _, logPath := range logs {
		content, err := os.ReadFile(logPath)
		if err != nil {
			continue
		}
		// Just use first 500 chars of each log for summary
		summary := string(content)
		if len(summary) > 500 {
			summary = summary[:500] + "..."
		}
		logSummaries = append(logSummaries, fmt.Sprintf("### %s\n%s", filepath.Base(logPath), summary))
	}

	// Load current skills
	proj := project.New(projectDir)
	skills, _ := project.LoadSkills(proj.SkillsDir())
	var skillNames []string
	for _, s := range skills {
		skillNames = append(skillNames, s.Name)
	}

	// Get an agent to analyze
	agents := agent.GetAvailableAgents()
	if len(agents) == 0 {
		return nil, agent.NoAgentsError{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	userFeedback := ""
	if opts.UserInput != "" {
		userFeedback = fmt.Sprintf(`
## User Feedback

The user provided this feedback about the sprint:

%s

Please incorporate this feedback into your analysis.
`, opts.UserInput)
	}

	prompt := fmt.Sprintf(`You are a software development coach. Analyze the sprint logs and identify improvement opportunities.

## Sprint %d Logs Summary

%s

## Current Skills

%s
%s
## Instructions

1. Identify patterns in the logs:
   - Repeated errors or failures
   - Missing guidance that caused issues
   - Areas where skills could be improved

2. Generate a retrospective summary covering:
   - What went well
   - What could be improved
   - Specific skill improvements needed

3. For each skill improvement, output in this format:
SKILL_UPDATE: skill-name
[The improvement text to add to the skill]
END_SKILL_UPDATE

Be specific and actionable. Focus on improvements that would prevent similar issues in future sprints.
`, sprintNumber, strings.Join(logSummaries, "\n\n"), strings.Join(skillNames, ", "), userFeedback)

	result, err := agents[0].Execute(ctx, prompt, projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to run retrospective analysis: %w", err)
	}

	// Parse skill updates from the response
	skillUpdates := parseSkillUpdates(result)

	// Apply skill updates
	for skillName, update := range skillUpdates {
		if err := applySkillUpdate(proj.SkillsDir(), skillName, update); err != nil {
			fmt.Printf("%s\n", logging.Yellow(fmt.Sprintf("Warning: failed to update skill %s: %v", skillName, err)))
		} else {
			fmt.Printf("%s\n", logging.Green(fmt.Sprintf("Updated skill: %s", skillName)))
		}
	}

	// Format and save the retrospective
	retroContent := logging.FormatRetro(sprintNumber, result, skillUpdates)
	if err := logging.EnsureRetrosDir(projectDir); err != nil {
		fmt.Printf("%s\n", logging.Yellow(fmt.Sprintf("Warning: failed to create retros directory: %v", err)))
	}
	if err := os.WriteFile(retroPath, []byte(retroContent), 0644); err != nil {
		fmt.Printf("%s\n", logging.Yellow(fmt.Sprintf("Warning: failed to write retrospective: %v", err)))
	} else {
		fmt.Printf("%s\n", logging.Green(fmt.Sprintf("Retrospective saved: %s", retroPath)))
	}

	return &Result{
		Message:  fmt.Sprintf("Retrospective complete for sprint %d. Updated %d skills.", sprintNumber, len(skillUpdates)),
		MoreWork: false,
	}, nil
}

// parseSkillUpdates extracts skill updates from the retrospective response
func parseSkillUpdates(response string) map[string]string {
	updates := make(map[string]string)

	lines := strings.Split(response, "\n")
	var currentSkill string
	var currentUpdate []string
	inUpdate := false

	for _, line := range lines {
		if strings.HasPrefix(line, "SKILL_UPDATE:") {
			if currentSkill != "" && len(currentUpdate) > 0 {
				updates[currentSkill] = strings.Join(currentUpdate, "\n")
			}
			currentSkill = strings.TrimSpace(strings.TrimPrefix(line, "SKILL_UPDATE:"))
			currentUpdate = nil
			inUpdate = true
		} else if line == "END_SKILL_UPDATE" {
			if currentSkill != "" && len(currentUpdate) > 0 {
				updates[currentSkill] = strings.Join(currentUpdate, "\n")
			}
			currentSkill = ""
			currentUpdate = nil
			inUpdate = false
		} else if inUpdate {
			currentUpdate = append(currentUpdate, line)
		}
	}

	// Handle case where END_SKILL_UPDATE is missing
	if currentSkill != "" && len(currentUpdate) > 0 {
		updates[currentSkill] = strings.Join(currentUpdate, "\n")
	}

	return updates
}

// applySkillUpdate appends an update to a skill file
func applySkillUpdate(skillsDir, skillName, update string) error {
	skillPath := filepath.Join(skillsDir, skillName+".md")

	// Check if skill exists
	if _, err := os.Stat(skillPath); err != nil {
		return fmt.Errorf("skill not found: %w", err)
	}

	// Load skill to update version
	skill, err := project.LoadSkill(skillPath)
	if err != nil {
		return fmt.Errorf("failed to parse skill: %w", err)
	}

	// Increment version
	skill.Metadata.Version++

	// Append the update to the content
	newContent := skill.Content + "\n\n## Retrospective Improvements (v" + fmt.Sprintf("%d", skill.Metadata.Version) + ")\n\n" + strings.TrimSpace(update) + "\n"

	// Format with updated frontmatter
	fullContent := project.FormatSkillWithFrontmatter(skill.Metadata, newContent)

	// Write back
	if err := os.WriteFile(skillPath, []byte(fullContent), 0644); err != nil {
		return fmt.Errorf("failed to write skill: %w", err)
	}

	return nil
}

// RunEvolution runs skill evolution at the start of a new sprint
// It finds the most recent retrospective by scanning the retros directory
func RunEvolution(projectDir string) error {
	// Find the most recent retrospective by looking at retro files
	retrosDir := filepath.Join(projectDir, ".ai", "retros")
	if _, err := os.Stat(retrosDir); err != nil {
		return nil // No retros dir, nothing to evolve
	}

	// Look for the highest-numbered retro file
	entries, err := os.ReadDir(retrosDir)
	if err != nil {
		return nil
	}

	var latestRetro string
	var latestNum int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Files are named sprint-NNN.md
		var num int
		if _, err := fmt.Sscanf(name, "sprint-%d.md", &num); err == nil {
			if num > latestNum {
				latestNum = num
				latestRetro = filepath.Join(retrosDir, name)
			}
		}
	}

	if latestRetro == "" {
		return nil // No retrospective files found
	}

	fmt.Printf("Learnings from sprint %d retrospective are available in skills.\n", latestNum)
	fmt.Printf("Retrospective file: %s\n", latestRetro)

	return nil
}
