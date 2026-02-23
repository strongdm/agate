package workflow

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/strongdm/agate/internal/agent"
	"github.com/strongdm/agate/internal/logging"
	"github.com/strongdm/agate/internal/project"
)

const maxReviewRetries = 3

// HumanNeededError indicates a task needs human intervention (e.g. too many review failures).
type HumanNeededError struct {
	Message string
}

func (e *HumanNeededError) Error() string {
	return e.Message
}

// NextOptions contains options for the Next workflow
type NextOptions struct {
	// StreamOutput enables streaming agent output to this writer
	StreamOutput io.Writer
	// PreferredAgent overrides automatic agent selection
	PreferredAgent string
}

// Next executes the next step in the workflow
func Next(projectDir string) (*Result, error) {
	return NextWithOptions(projectDir, NextOptions{})
}

// NextWithOptions executes the next step with options
func NextWithOptions(projectDir string, opts NextOptions) (*Result, error) {
	proj := project.New(projectDir)

	// Check for GOAL.md
	if !proj.HasGoal() {
		return nil, fmt.Errorf("GOAL.md not found. Create a GOAL.md file describing what you want to build")
	}

	// Check for agents
	if err := agent.EnsureAgentsAvailable(); err != nil {
		return nil, fmt.Errorf("%w\n\n%s", err, agent.FormatInstallInstructions())
	}

	// Use GetStatus for unified state detection
	fsys := os.DirFS(projectDir)
	status := GetStatus(fsys)

	// Check if we're still in planning phases
	if status.Phase != PhaseExecution {
		// Execute ONE planning phase
		planOpts := PlanOptions{
			StreamOutput:   opts.StreamOutput,
			PreferredAgent: opts.PreferredAgent,
		}
		return ExecutePlanPhase(projectDir, planOpts)
	}

	// Use sprint info from GetStatus
	if status.CurrentSprintPath == "" {
		return &Result{
			Message:  "No sprint files found. Run 'agate next' to continue planning.",
			MoreWork: true,
		}, nil
	}

	sprintNum := status.CurrentSprintNum

	// Parse the sprint file (need mutable version for checkbox updates)
	sprintPath := filepath.Join(projectDir, status.CurrentSprintPath)
	sprint, err := ParseSprint(sprintPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sprint: %w", err)
	}

	// Check if sprint is complete
	if sprint.IsComplete() {
		return assessGoalAndPlanNext(projectDir, proj, sprintNum, opts)
	}

	// Get the next sub-task to work on
	subTask := sprint.GetNextSubTask()
	if subTask == nil {
		// Auto-check orphaned tasks (no subtasks, or all subtasks done but task unchecked)
		fixed := autoCheckOrphanedTasks(sprint)
		if fixed > 0 {
			// Re-parse and retry once
			sprint, err = ParseSprint(sprintPath)
			if err != nil {
				return nil, fmt.Errorf("failed to re-parse sprint: %w", err)
			}
			if sprint.IsComplete() {
				return assessGoalAndPlanNext(projectDir, proj, sprintNum, opts)
			}
			subTask = sprint.GetNextSubTask()
		}
		if subTask == nil {
			return &Result{
				Message:  "No more tasks in current sprint.",
				MoreWork: false,
			}, nil
		}
	}

	// Get the parent task for context
	currentTask := sprint.GetCurrentTask()
	if currentTask == nil {
		return nil, fmt.Errorf("no current task found")
	}

	// Create logger
	logger := logging.NewLogger(projectDir, sprintNum)

	// Check if task has exceeded review retry limit
	if currentTask.FailureCount >= maxReviewRetries {
		// If already replanned, give up
		if currentTask.ReplanCount > 0 {
			return nil, &HumanNeededError{
				Message: fmt.Sprintf("task %q has failed review %d times (max %d) even after replan, human intervention needed", currentTask.Text, currentTask.FailureCount, maxReviewRetries),
			}
		}
		// Attempt replan
		fmt.Println(logging.Yellow("⚠ Review failed too many times. Attempting sprint replan..."))
		result, err := attemptReplan(projectDir, proj, sprint, currentTask, logger, opts)
		if err != nil {
			return nil, &HumanNeededError{
				Message: fmt.Sprintf("task %q has failed review %d times and replan failed: %v", currentTask.Text, currentTask.FailureCount, err),
			}
		}
		return result, nil
	}

	// Execute the sub-task
	result, err := executeSubTask(projectDir, proj, sprint, currentTask, subTask, logger, opts, false)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// executeSubTask runs a single sub-task. isRecovery prevents recursive recovery attempts.
func executeSubTask(projectDir string, proj *project.Project, sprint *SprintState, task *Task, subTask *SubTask, logger *logging.Logger, opts NextOptions, isRecovery bool) (*Result, error) {
	// Determine which agent to use
	agentName := opts.PreferredAgent
	if agentName == "" {
		agentName = selectAgentForSkill(subTask.Skill)
	}

	selectedAgent := agent.GetAgentByName(agentName)
	if selectedAgent == nil || !selectedAgent.Available() {
		// Fall back to first available
		agents := agent.GetAvailableAgents()
		if len(agents) == 0 {
			return nil, agent.NoAgentsError{}
		}
		selectedAgent = agents[0]
	}

	// Build context from project
	designContent := ""
	if designPath := filepath.Join(proj.DesignDir(), "overview.md"); fileExists(designPath) {
		if content, err := os.ReadFile(designPath); err == nil {
			designContent = string(content)
		}
	}

	// Load skills for context
	skills, _ := project.LoadSkills(proj.SkillsDir())
	skillContent := getSkillContent(skills, subTask.Skill)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Build prompt based on skill type
	prompt := buildSubTaskPrompt(task, subTask, designContent, skillContent, sprint)

	// Get sprint number for display
	sprintNum := ExtractSprintNum(filepath.Base(sprint.FilePath))
	taskSummary := TruncateText(subTask.Text, 50)

	// Show progress bar before invocation so user sees where we are
	progressBar := sprint.RenderProgressBar(sprintNum, task.Index, subTask.Index)
	fmt.Printf("%s\n", progressBar)

	// Execute with logging
	execResult := agent.ExecuteWithLogging(ctx, selectedAgent, prompt, projectDir, agent.ExecuteOptions{
		Logger:        logger,
		Phase:         "implement",
		Task:          subTask.Text,
		TaskIndex:     subTask.Index,
		Skill:         subTask.Skill,
		PromptSummary: taskSummary,
		StreamWriter:  opts.StreamOutput,
	})

	if execResult.Error != nil {
		if isRecovery {
			return nil, fmt.Errorf("failed to execute sub-task (after recovery): %w", execResult.Error)
		}
		fmt.Println(logging.Yellow("⚠ Agent execution failed. Attempting recovery..."))
		recoveryErr := attemptRecovery(projectDir, proj, task, subTask,
			selectedAgent.Name(), execResult, logger, opts)
		if recoveryErr != nil {
			fmt.Printf("  %s\n", logging.Yellow(fmt.Sprintf("Recovery failed: %v", recoveryErr)))
			return nil, fmt.Errorf("failed to execute sub-task: %w", execResult.Error)
		}
		fmt.Println("  " + logging.Yellow("Recovery complete. Retrying original task..."))
		return executeSubTask(projectDir, proj, sprint, task, subTask, logger, opts, true)
	}

	// If this is an implementation task, parse and write files
	if isImplementationSkill(subTask.Skill) {
		filesWritten := parseAndWriteFiles(projectDir, execResult.Output)
		if filesWritten > 0 {
			fmt.Printf("  %s\n", logging.Green(fmt.Sprintf("Wrote %d file(s)", filesWritten)))
		}
	}

	// Check for review failure
	isReviewer := subTask.Skill == "_reviewer" || strings.HasSuffix(subTask.Skill, "-reviewer")
	if isReviewer && !isReviewApproved(execResult.Output) {
		// Review failed - add ❌ to parent task and uncheck subtasks for retry
		fmt.Println(logging.Yellow("⚠ Review failed. Adding failure marker and unchecking tasks for retry..."))

		// Add failure emoji to the parent task
		if err := sprint.AddFailure(task.Index); err != nil {
			fmt.Printf("%s\n", logging.Yellow(fmt.Sprintf("Warning: failed to add failure marker: %v", err)))
		}

		// Uncheck this subtask and all subsequent ones in this task
		for i := subTask.Index; i < len(task.SubTasks); i++ {
			if task.SubTasks[i].Checked {
				if err := sprint.UncheckSubTask(task.Index, i); err != nil {
					fmt.Printf("%s\n", logging.Yellow(fmt.Sprintf("Warning: failed to uncheck sub-task %d: %v", i, err)))
				}
			}
		}
		// Re-parse sprint
		sprint, _ = ParseSprint(sprint.FilePath)
		return &Result{
			Message:  "Review failed. Tasks unchecked for retry. Run 'agate next' to try again.",
			MoreWork: true,
		}, nil
	}

	// Mark the sub-task as complete
	if err := sprint.CheckSubTask(task.Index, subTask.Index); err != nil {
		return nil, fmt.Errorf("failed to mark sub-task complete: %w", err)
	}

	// Re-parse to get updated state
	sprint, _ = ParseSprint(sprint.FilePath)

	// Check if all sub-tasks for this task are complete
	if sprint.AllSubTasksComplete(task.Index) {
		if err := sprint.CheckTask(task.Index); err != nil {
			fmt.Printf("%s\n", logging.Yellow(fmt.Sprintf("Warning: failed to mark task complete: %v", err)))
		}
		fmt.Printf("%s\n", logging.Green(fmt.Sprintf("✓ Task complete: %s", task.Text)))
	}

	// Re-parse for final state
	sprint, _ = ParseSprint(sprint.FilePath)

	// Check progress
	completed, total := sprint.GetOverallProgress()
	if sprint.IsComplete() {
		return &Result{
			Message:  fmt.Sprintf("Sprint complete! All %d tasks done.", total),
			MoreWork: true, // There might be more sprints
		}, nil
	}

	pct := 0
	if total > 0 {
		pct = completed * 100 / total
	}
	return &Result{
		Message:  fmt.Sprintf("Sub-task complete (%d%%). Run 'agate next' to continue.", pct),
		MoreWork: true,
	}, nil
}

func selectAgentForSkill(skill string) string {
	// Prefer codex for implementation, claude for review/planning
	if strings.Contains(skill, "coder") {
		return "codex"
	}
	if strings.HasPrefix(skill, "_") {
		return "claude"
	}
	return "claude"
}

func getSkillContent(skills []project.Skill, skillName string) string {
	for _, s := range skills {
		if s.Name == skillName {
			return s.Content
		}
	}
	return ""
}

func buildSubTaskPrompt(task *Task, subTask *SubTask, designContent, skillContent string, sprint *SprintState) string {
	var sb strings.Builder

	sb.WriteString("You are working on a software project.\n\n")

	if designContent != "" {
		sb.WriteString("## Design Context\n\n")
		sb.WriteString(designContent)
		sb.WriteString("\n\n")
	}

	if skillContent != "" {
		sb.WriteString("## Skill Guidelines\n\n")
		sb.WriteString(skillContent)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Current Task\n\n")
	sb.WriteString(fmt.Sprintf("**Main Task**: %s\n\n", task.Text))
	sb.WriteString(fmt.Sprintf("**Sub-task**: %s\n\n", subTask.Text))

	sb.WriteString("## Instructions\n\n")

	if isImplementationSkill(subTask.Skill) {
		sb.WriteString(`Complete the sub-task above. Output any files that should be created or modified.
For each file, use this format:

### File: path/to/file.ext
` + "```" + `
file contents here
` + "```" + `

If no files need to be created, just describe what you did.
`)
	} else if strings.Contains(subTask.Skill, "reviewer") || subTask.Skill == "_reviewer" {
		sb.WriteString(`Review the implementation for this task. Check that:
1. The task requirements are met
2. Code follows best practices
3. No obvious bugs or issues

If the implementation is good, respond with: APPROVED
If there are issues, describe them.
`)
	} else {
		sb.WriteString("Complete the sub-task described above.\n")
	}

	return sb.String()
}

func isImplementationSkill(skill string) bool {
	return strings.Contains(skill, "coder") || skill == "implement"
}

// isReviewApproved checks if the review output contains APPROVED
func isReviewApproved(output string) bool {
	output = strings.ToUpper(output)
	return strings.Contains(output, "APPROVED")
}

// fileExists is defined in plan.go

// autoCheckOrphanedTasks checks top-level tasks that have no subtasks or
// have all subtasks complete but are themselves unchecked. Returns the number fixed.
func autoCheckOrphanedTasks(sprint *SprintState) int {
	fixed := 0
	for i := range sprint.Tasks {
		task := &sprint.Tasks[i]
		if task.Checked {
			continue
		}
		// Task with no subtasks, or all subtasks already done
		if len(task.SubTasks) == 0 || sprint.AllSubTasksComplete(task.Index) {
			if err := sprint.CheckTask(task.Index); err != nil {
				fmt.Printf("%s\n", logging.Yellow(fmt.Sprintf("Warning: failed to auto-check task %d: %v", i, err)))
				continue
			}
			fmt.Printf("%s\n", logging.Green(fmt.Sprintf("✓ Auto-checked orphaned task: %s", task.Text)))
			fixed++
		}
	}
	return fixed
}

// attemptRecovery invokes a Claude recovery agent to diagnose and fix the environment
// after a task execution failure. Returns nil on success, error on failure.
func attemptRecovery(projectDir string, proj *project.Project, task *Task, subTask *SubTask,
	failedAgentName string, execResult agent.Result, logger *logging.Logger, opts NextOptions) error {

	recoveryAgent := agent.GetAgentByName("claude")
	if recoveryAgent == nil || !recoveryAgent.Available() {
		return fmt.Errorf("claude agent not available for recovery")
	}

	// Load the _recover skill content
	skills, _ := project.LoadSkills(proj.SkillsDir())
	recoverSkillContent := getSkillContent(skills, "_recover")

	prompt := buildRecoveryPrompt(failedAgentName, execResult.Error, task, subTask, execResult.LogPath, recoverSkillContent)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	recoveryResult := agent.ExecuteWithLogging(ctx, recoveryAgent, prompt, projectDir, agent.ExecuteOptions{
		Logger:        logger,
		Phase:         "recover",
		Task:          subTask.Text,
		TaskIndex:     subTask.Index,
		Skill:         "_recover",
		PromptSummary: "Recovery: " + TruncateText(subTask.Text, 40),
		StreamWriter:  opts.StreamOutput,
	})

	if recoveryResult.Error != nil {
		return fmt.Errorf("recovery agent failed: %w", recoveryResult.Error)
	}

	return nil
}

// buildRecoveryPrompt constructs the prompt for the recovery agent.
func buildRecoveryPrompt(failedAgentName string, execErr error, task *Task, subTask *SubTask, logPath string, skillContent string) string {
	var sb strings.Builder

	if skillContent != "" {
		sb.WriteString(skillContent)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Failure Details\n\n")
	sb.WriteString(fmt.Sprintf("**Failed agent**: %s\n", failedAgentName))
	sb.WriteString(fmt.Sprintf("**Error**: %s\n", execErr.Error()))
	if logPath != "" {
		sb.WriteString(fmt.Sprintf("**Log file**: %s\n", logPath))
	}
	sb.WriteString("\n")

	sb.WriteString("## Task Context\n\n")
	sb.WriteString(fmt.Sprintf("**Task**: %s\n", task.Text))
	sb.WriteString(fmt.Sprintf("**Sub-task**: %s\n", subTask.Text))
	sb.WriteString(fmt.Sprintf("**Skill**: %s\n\n", subTask.Skill))

	sb.WriteString("Diagnose the root cause of the error above and fix the environment so the task can be retried. Do NOT attempt to complete the original task.\n")

	return sb.String()
}

// attemptReplan invokes a Claude replanner agent to rewrite the failing task's subtasks
// when review has failed too many times. Returns nil error on success.
func attemptReplan(projectDir string, proj *project.Project, sprint *SprintState, task *Task, logger *logging.Logger, opts NextOptions) (*Result, error) {
	replanAgent := agent.GetAgentByName("claude")
	if replanAgent == nil || !replanAgent.Available() {
		return nil, fmt.Errorf("claude agent not available for replan")
	}

	// Load the _replanner skill content
	skills, _ := project.LoadSkills(proj.SkillsDir())
	replanSkillContent := getSkillContent(skills, "_replanner")

	// Load design overview
	designContent := ""
	if designPath := filepath.Join(proj.DesignDir(), "overview.md"); fileExists(designPath) {
		if content, err := os.ReadFile(designPath); err == nil {
			designContent = string(content)
		}
	}

	// Find the last reviewer log for feedback
	sprintNum := ExtractSprintNum(filepath.Base(sprint.FilePath))
	reviewerFeedback := ""
	lastLog := findLastReviewerLog(projectDir, sprintNum)
	if lastLog != "" {
		reviewerFeedback = extractReviewerFeedback(lastLog)
	}

	// Read current sprint content
	sprintContent, err := os.ReadFile(sprint.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sprint file: %w", err)
	}

	prompt := buildReplanPrompt(replanSkillContent, designContent, string(sprintContent), reviewerFeedback, task, sprint.FilePath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	replanResult := agent.ExecuteWithLogging(ctx, replanAgent, prompt, projectDir, agent.ExecuteOptions{
		Logger:        logger,
		Phase:         "replan",
		Task:          task.Text,
		TaskIndex:     task.Index,
		Skill:         "_replanner",
		PromptSummary: "Replan: " + TruncateText(task.Text, 40),
		StreamWriter:  opts.StreamOutput,
	})

	if replanResult.Error != nil {
		return nil, fmt.Errorf("replan agent failed: %w", replanResult.Error)
	}

	// Re-parse sprint from disk — the replanner agent edited the file directly,
	// so the in-memory sprint.Content is stale and would clobber the replan.
	sprint, err = ParseSprint(sprint.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to re-parse sprint after replan: %w", err)
	}

	// Find the task again by matching text (index may have shifted if subtasks changed)
	var replanTask *Task
	for i := range sprint.Tasks {
		if NormalizeTaskText(sprint.Tasks[i].Text) == NormalizeTaskText(task.Text) {
			replanTask = &sprint.Tasks[i]
			break
		}
	}
	if replanTask == nil {
		return nil, fmt.Errorf("could not find task %q after replan", task.Text)
	}

	// Clear failure markers and add replan marker
	if err := sprint.ClearFailures(replanTask.Index); err != nil {
		fmt.Printf("%s\n", logging.Yellow(fmt.Sprintf("Warning: failed to clear failure markers: %v", err)))
	}
	if err := sprint.AddReplanMarker(replanTask.Index); err != nil {
		fmt.Printf("%s\n", logging.Yellow(fmt.Sprintf("Warning: failed to add replan marker: %v", err)))
	}

	fmt.Println("  Replan complete. Sprint file updated. Run 'agate next' to retry.")
	return &Result{
		Message:  "Sprint replanned. Run 'agate next' to retry the task.",
		MoreWork: true,
	}, nil
}

// findLastReviewerLog finds the last reviewer log file for the given sprint
func findLastReviewerLog(projectDir string, sprintNum int) string {
	logs, err := logging.ListLogs(projectDir, sprintNum)
	if err != nil || len(logs) == 0 {
		return ""
	}

	// Logs are sorted alphabetically (by sequence number), find last reviewer log
	var lastReviewerLog string
	for _, log := range logs {
		if strings.Contains(filepath.Base(log), "_reviewer") {
			lastReviewerLog = log
		}
	}
	return lastReviewerLog
}

// extractReviewerFeedback reads a log file and extracts the response content
func extractReviewerFeedback(logPath string) string {
	content, err := os.ReadFile(logPath)
	if err != nil {
		return ""
	}

	// Find ## Response section and extract content between ``` blocks
	lines := strings.Split(string(content), "\n")
	inResponse := false
	inCodeBlock := false
	var feedback []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## Response") {
			inResponse = true
			continue
		}
		if inResponse && strings.HasPrefix(line, "## ") {
			break // Next section
		}
		if inResponse {
			if strings.HasPrefix(line, "```") {
				if inCodeBlock {
					break // End of response code block
				}
				inCodeBlock = true
				continue
			}
			if inCodeBlock {
				feedback = append(feedback, line)
			}
		}
	}

	return strings.Join(feedback, "\n")
}

// buildReplanPrompt constructs the prompt for the replanner agent
func buildReplanPrompt(skillContent, designContent, sprintContent, reviewerFeedback string, task *Task, sprintPath string) string {
	var sb strings.Builder

	if skillContent != "" {
		sb.WriteString(skillContent)
		sb.WriteString("\n\n")
	}

	if designContent != "" {
		sb.WriteString("## Design Context\n\n")
		sb.WriteString(designContent)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Current Sprint File\n\n")
	sb.WriteString(fmt.Sprintf("Path: %s\n\n", sprintPath))
	sb.WriteString("```markdown\n")
	sb.WriteString(sprintContent)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Failing Task\n\n")
	sb.WriteString(fmt.Sprintf("**Task %d**: %s\n\n", task.Index+1, task.Text))
	sb.WriteString(fmt.Sprintf("This task has failed review %d times.\n\n", task.FailureCount))

	if reviewerFeedback != "" {
		sb.WriteString("## Reviewer Feedback (from last review)\n\n")
		sb.WriteString(reviewerFeedback)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString(fmt.Sprintf("Edit the sprint file at %s to fix the subtasks for task %d (%q). ", sprintPath, task.Index+1, task.Text))
	sb.WriteString("Rewrite the subtasks so the task can succeed. Uncheck all subtasks in the rewritten task.\n")

	return sb.String()
}

// completedSprint holds the number and content of a completed sprint file.
type completedSprint struct {
	Num     int
	Content string
}

// findSprintByNum scans sprintsDir for any .md file whose name starts with the
// zero-padded sprint number prefix (e.g. "02-"). Returns the full path or "".
func findSprintByNum(sprintsDir string, num int) string {
	prefix := fmt.Sprintf("%02d-", num)
	entries, err := os.ReadDir(sprintsDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && strings.HasPrefix(e.Name(), prefix) {
			return filepath.Join(sprintsDir, e.Name())
		}
	}
	return ""
}

// loadCompletedSprintSummaries reads all sprint .md files with number <= upToNum,
// sorted by number.
func loadCompletedSprintSummaries(sprintsDir string, upToNum int) []completedSprint {
	entries, err := os.ReadDir(sprintsDir)
	if err != nil {
		return nil
	}

	var results []completedSprint
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		num := ExtractSprintNum(e.Name())
		if num <= 0 || num > upToNum {
			continue
		}
		content, err := os.ReadFile(filepath.Join(sprintsDir, e.Name()))
		if err != nil {
			continue
		}
		results = append(results, completedSprint{Num: num, Content: string(content)})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Num < results[j].Num })
	return results
}

// buildNextSprintPrompt constructs the prompt for the assess-and-plan-next agent call.
func buildNextSprintPrompt(goalContent, designContent string, completedSprints []completedSprint, skillNames []string, outputPath string) string {
	var sb strings.Builder

	sb.WriteString("You are a project manager assessing whether a project goal is fully met, or planning the next sprint.\n\n")

	sb.WriteString("## Goal\n\n")
	sb.WriteString(goalContent)
	sb.WriteString("\n\n")

	if designContent != "" {
		sb.WriteString("## Design\n\n")
		sb.WriteString(designContent)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Completed Sprints\n\n")
	for _, cs := range completedSprints {
		sb.WriteString(fmt.Sprintf("### Sprint %d\n\n", cs.Num))
		sb.WriteString(cs.Content)
		sb.WriteString("\n\n")
	}

	// Build skill-aware task format examples
	coderSkill := findSkillByPattern(skillNames, "coder", "coder")
	reviewerSkill := findSkillByPattern(skillNames, "reviewer", "_reviewer")

	// Filter skill names for display: include project skills + _reviewer
	var availableSkills []string
	for _, s := range skillNames {
		if !strings.HasPrefix(s, "_") || s == "_reviewer" {
			availableSkills = append(availableSkills, s)
		}
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("Look at the goal and the completed sprints above. Decide:\n\n")
	sb.WriteString("1. If the goal is **fully met** by the completed sprints, respond with exactly:\n")
	sb.WriteString("   GOAL_COMPLETE\n\n")
	sb.WriteString("2. If more work is needed, write the next sprint document to this file path:\n")
	sb.WriteString(fmt.Sprintf("   %s\n\n", outputPath))
	sb.WriteString("The sprint document should use nested task checkboxes with skill assignments:\n\n")
	sb.WriteString(fmt.Sprintf(`- [ ] Task description
  - [ ] %s: Implementation details
  - [ ] %s: Review implementation
  - [ ] _reviewer: Validate correctness
`, coderSkill, reviewerSkill))
	sb.WriteString("\n")
	if len(availableSkills) > 0 {
		sb.WriteString(fmt.Sprintf("Available skills: %s\n\n", strings.Join(availableSkills, ", ")))
	}
	sb.WriteString("IMPORTANT: Either respond with GOAL_COMPLETE or write the sprint document to the file path above. Do not output any other commentary.\n")

	return sb.String()
}

// assessGoalAndPlanNext checks if the goal is met after a sprint completes.
// If a next sprint already exists, returns MoreWork. Otherwise calls an agent
// that either confirms GOAL_COMPLETE or writes the next sprint file.
func assessGoalAndPlanNext(projectDir string, proj *project.Project, completedSprintNum int, opts NextOptions) (*Result, error) {
	nextNum := completedSprintNum + 1

	// If next sprint already exists, just continue
	if findSprintByNum(proj.SprintsDir(), nextNum) != "" {
		return &Result{
			Message:  fmt.Sprintf("Sprint %d complete! Run 'agate next' to start sprint %d.", completedSprintNum, nextNum),
			MoreWork: true,
		}, nil
	}

	// Load GOAL.md
	goalContent, err := os.ReadFile(proj.GoalPath())
	if err != nil {
		return nil, fmt.Errorf("failed to read GOAL.md: %w", err)
	}

	// Load design (optional)
	designContent := ""
	if designPath := filepath.Join(proj.DesignDir(), "overview.md"); fileExists(designPath) {
		if content, err := os.ReadFile(designPath); err == nil {
			designContent = string(content)
		}
	}

	// Load completed sprint summaries
	completed := loadCompletedSprintSummaries(proj.SprintsDir(), completedSprintNum)

	// Load skill names (filter out builtins except _reviewer)
	skills, _ := project.LoadSkills(proj.SkillsDir())
	var skillNames []string
	for _, s := range skills {
		if !strings.HasPrefix(s.Name, "_") || s.Name == "_reviewer" {
			skillNames = append(skillNames, s.Name)
		}
	}

	// Build output path
	outputPath := filepath.Join(proj.SprintsDir(), fmt.Sprintf("%02d-next.md", nextNum))

	// Build prompt
	prompt := buildNextSprintPrompt(string(goalContent), designContent, completed, skillNames, outputPath)

	// Select agent (prefer claude via _planner)
	agentName := opts.PreferredAgent
	if agentName == "" {
		agentName = selectAgentForSkill("_planner")
	}
	selectedAgent := agent.GetAgentByName(agentName)
	if selectedAgent == nil || !selectedAgent.Available() {
		agents := agent.GetAvailableAgents()
		if len(agents) == 0 {
			return nil, agent.NoAgentsError{}
		}
		selectedAgent = agents[0]
	}

	logger := logging.NewLogger(projectDir, completedSprintNum)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	execResult := agent.ExecuteWithLogging(ctx, selectedAgent, prompt, projectDir, agent.ExecuteOptions{
		Logger:        logger,
		Phase:         "assess",
		Task:          "Assess goal and plan next sprint",
		TaskIndex:     0,
		Skill:         "_planner",
		PromptSummary: "Assessing goal completion",
		StreamWriter:  opts.StreamOutput,
	})

	if execResult.Error != nil {
		return nil, fmt.Errorf("failed to assess goal: %w", execResult.Error)
	}

	// Check if the agent declared GOAL_COMPLETE
	if strings.Contains(execResult.Output, "GOAL_COMPLETE") {
		return &Result{
			Message:  fmt.Sprintf("Sprint %d complete. All sprints done — goal is fully met.", completedSprintNum),
			MoreWork: false,
		}, nil
	}

	// Validate the new sprint file was written
	if err := validateMarkdownContent(outputPath); err != nil {
		return nil, fmt.Errorf("agent did not write a valid next sprint: %w", err)
	}

	return &Result{
		Message:  fmt.Sprintf("Sprint %d complete! Next sprint planned. Run 'agate next' to continue.", completedSprintNum),
		MoreWork: true,
	}, nil
}

func parseAndWriteFiles(projectDir string, content string) int {
	// Simple parser for "### File: path" format
	lines := strings.Split(content, "\n")
	var currentFile string
	var currentContent []string
	inCodeBlock := false
	filesWritten := 0

	writeCurrentFile := func() {
		if currentFile != "" && len(currentContent) > 0 {
			path := filepath.Join(projectDir, currentFile)
			dir := filepath.Dir(path)
			os.MkdirAll(dir, 0755)
			content := strings.Join(currentContent, "\n")
			if err := os.WriteFile(path, []byte(content), 0644); err == nil {
				filesWritten++
				fmt.Printf("  %s\n", logging.Green(fmt.Sprintf("Wrote: %s", currentFile)))
			}
		}
		currentFile = ""
		currentContent = nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "### File:") {
			writeCurrentFile()
			currentFile = strings.TrimSpace(strings.TrimPrefix(line, "### File:"))
			inCodeBlock = false
			continue
		}

		if currentFile != "" {
			if strings.HasPrefix(line, "```") {
				if inCodeBlock {
					inCodeBlock = false
				} else {
					inCodeBlock = true
				}
				continue
			}

			if inCodeBlock {
				currentContent = append(currentContent, line)
			}
		}
	}

	writeCurrentFile()
	return filesWritten
}
