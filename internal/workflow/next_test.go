package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strongdm/agate/internal/project"
)

func TestBuildRecoveryPrompt_IncludesErrorDetails(t *testing.T) {
	task := &Task{Text: "Implement feature X"}
	subTask := &SubTask{Text: "Write code for feature X", Skill: "go-coder"}
	err := fmt.Errorf("API Error: 400 bad request")
	logPath := ".ai/logs/sprint-001/003-implement.md"
	skillContent := "# Recovery Agent\n\nYou are a recovery agent."

	prompt := buildRecoveryPrompt("codex", err, task, subTask, logPath, skillContent)

	// Should include error message
	if !strings.Contains(prompt, "API Error: 400 bad request") {
		t.Error("prompt should contain error message")
	}

	// Should include failed agent name
	if !strings.Contains(prompt, "codex") {
		t.Error("prompt should contain failed agent name")
	}

	// Should include task info
	if !strings.Contains(prompt, "Implement feature X") {
		t.Error("prompt should contain task text")
	}
	if !strings.Contains(prompt, "Write code for feature X") {
		t.Error("prompt should contain sub-task text")
	}

	// Should include skill name
	if !strings.Contains(prompt, "go-coder") {
		t.Error("prompt should contain skill name")
	}

	// Should include log path
	if !strings.Contains(prompt, logPath) {
		t.Error("prompt should contain log path")
	}

	// Should include skill content
	if !strings.Contains(prompt, "Recovery Agent") {
		t.Error("prompt should contain skill content")
	}
}

func TestBuildRecoveryPrompt_ExcludesDesignContext(t *testing.T) {
	task := &Task{Text: "Implement feature X"}
	subTask := &SubTask{Text: "Write code", Skill: "go-coder"}
	err := fmt.Errorf("some error")

	prompt := buildRecoveryPrompt("claude", err, task, subTask, "", "")

	// Should NOT include design context headers
	if strings.Contains(prompt, "## Design Context") {
		t.Error("recovery prompt should not contain design context")
	}
	if strings.Contains(prompt, "## Skill Guidelines") {
		t.Error("recovery prompt should not contain skill guidelines header")
	}
}

func TestBuildRecoveryPrompt_EmptyLogPath(t *testing.T) {
	task := &Task{Text: "Task"}
	subTask := &SubTask{Text: "Sub", Skill: "coder"}
	err := fmt.Errorf("fail")

	prompt := buildRecoveryPrompt("claude", err, task, subTask, "", "")

	// Should not contain log file line when path is empty
	if strings.Contains(prompt, "**Log file**") {
		t.Error("prompt should not contain log file line when path is empty")
	}
}

func TestBuildReplanPrompt_IncludesContext(t *testing.T) {
	task := &Task{
		Index:        1,
		Text:         "Implement auth system",
		FailureCount: 3,
	}
	skillContent := "# Sprint Replanner\n\nYou are a sprint replanner."
	designContent := "# Design Overview\n\nAuth system design."
	sprintContent := "# Sprint 1\n\n- [ ] ❌❌❌ Implement auth system\n  - [ ] go-coder: Write auth\n"
	reviewerFeedback := "The auth tests were run before the code was written."
	sprintPath := ".ai/sprints/01-initial.md"

	prompt := buildReplanPrompt(skillContent, designContent, sprintContent, reviewerFeedback, task, sprintPath)

	// Should include skill content
	if !strings.Contains(prompt, "Sprint Replanner") {
		t.Error("prompt should contain skill content")
	}

	// Should include design content
	if !strings.Contains(prompt, "Auth system design") {
		t.Error("prompt should contain design content")
	}

	// Should include sprint content
	if !strings.Contains(prompt, "Implement auth system") {
		t.Error("prompt should contain sprint content")
	}

	// Should include reviewer feedback
	if !strings.Contains(prompt, "auth tests were run before") {
		t.Error("prompt should contain reviewer feedback")
	}

	// Should include task text
	if !strings.Contains(prompt, "Implement auth system") {
		t.Error("prompt should contain task text")
	}

	// Should include sprint path
	if !strings.Contains(prompt, sprintPath) {
		t.Error("prompt should contain sprint path")
	}
}

func TestBuildReplanPrompt_NoDesignOrFeedback(t *testing.T) {
	task := &Task{Index: 0, Text: "Task", FailureCount: 3}

	prompt := buildReplanPrompt("", "", "sprint content", "", task, "sprint.md")

	// Should not include design context header
	if strings.Contains(prompt, "## Design Context") {
		t.Error("prompt should not contain design context when empty")
	}

	// Should not include reviewer feedback header
	if strings.Contains(prompt, "## Reviewer Feedback") {
		t.Error("prompt should not contain reviewer feedback when empty")
	}

	// Should still include sprint content
	if !strings.Contains(prompt, "sprint content") {
		t.Error("prompt should contain sprint content")
	}
}

func TestFindLastReviewerLog(t *testing.T) {
	// Create temp directory structure simulating log files
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, ".ai", "logs", "sprint-001")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create some log files
	logFiles := []string{
		"001-implement-00-go-coder-codex.md",
		"002-implement-01-_reviewer-claude.md",
		"003-implement-00-go-coder-codex.md",
		"004-implement-01-_reviewer-claude.md",
	}
	for _, name := range logFiles {
		if err := os.WriteFile(filepath.Join(logsDir, name), []byte("log content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result := findLastReviewerLog(tmpDir, 1)

	if result == "" {
		t.Fatal("expected to find a reviewer log")
	}

	// Should be the last reviewer log
	if !strings.Contains(result, "004-implement-01-_reviewer-claude.md") {
		t.Errorf("expected last reviewer log, got %s", result)
	}
}

func TestFindLastReviewerLog_NoLogs(t *testing.T) {
	tmpDir := t.TempDir()
	result := findLastReviewerLog(tmpDir, 1)
	if result != "" {
		t.Errorf("expected empty string for no logs, got %s", result)
	}
}

func TestExtractReviewerFeedback(t *testing.T) {
	// Create a log file with the standard format
	tmpFile := filepath.Join(t.TempDir(), "log.md")
	content := `# Agent Invocation Log

## Metadata

| Field | Value |
|-------|-------|
| Phase | implement |

## Prompt

` + "```" + `
Review the implementation
` + "```" + `

## Response

` + "```" + `
ISSUES_FOUND: The tests are trying to import a module that doesn't exist yet.
The play-test step should come after the code is fully written.
` + "```" + `

## Notes

Some notes here.
`

	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	feedback := extractReviewerFeedback(tmpFile)

	if !strings.Contains(feedback, "ISSUES_FOUND") {
		t.Error("feedback should contain ISSUES_FOUND")
	}
	if !strings.Contains(feedback, "play-test step should come after") {
		t.Error("feedback should contain the reviewer's message")
	}
}

func TestExtractReviewerFeedback_EmptyFile(t *testing.T) {
	feedback := extractReviewerFeedback("/nonexistent/file.md")
	if feedback != "" {
		t.Errorf("expected empty feedback for nonexistent file, got %q", feedback)
	}
}

func TestBuildNextSprintPrompt_IncludesAllContext(t *testing.T) {
	goal := "Build a CLI tool for task management"
	design := "# Design\n\nArchitecture overview here."
	sprints := []completedSprint{
		{Num: 1, Content: "# Sprint 1\n\n- [x] Set up project"},
	}
	skills := []string{"go-coder", "go-reviewer", "_reviewer"}
	outputPath := ".ai/sprints/02-next.md"

	prompt := buildNextSprintPrompt(goal, design, sprints, skills, outputPath)

	// Should include goal
	if !strings.Contains(prompt, "Build a CLI tool") {
		t.Error("prompt should contain goal content")
	}
	// Should include design
	if !strings.Contains(prompt, "## Design") {
		t.Error("prompt should contain design section")
	}
	if !strings.Contains(prompt, "Architecture overview") {
		t.Error("prompt should contain design content")
	}
	// Should include completed sprint
	if !strings.Contains(prompt, "### Sprint 1") {
		t.Error("prompt should contain sprint 1 heading")
	}
	if !strings.Contains(prompt, "Set up project") {
		t.Error("prompt should contain sprint content")
	}
	// Should include skills
	if !strings.Contains(prompt, "go-coder") {
		t.Error("prompt should contain coder skill")
	}
	// Should include output path
	if !strings.Contains(prompt, outputPath) {
		t.Error("prompt should contain output path")
	}
	// Should include GOAL_COMPLETE instruction
	if !strings.Contains(prompt, "GOAL_COMPLETE") {
		t.Error("prompt should contain GOAL_COMPLETE instruction")
	}
}

func TestBuildNextSprintPrompt_NoDesign(t *testing.T) {
	prompt := buildNextSprintPrompt("goal", "", []completedSprint{{Num: 1, Content: "sprint 1"}}, nil, "out.md")

	if strings.Contains(prompt, "## Design") {
		t.Error("prompt should not contain design section when design is empty")
	}
}

func TestBuildNextSprintPrompt_MultipleCompletedSprints(t *testing.T) {
	sprints := []completedSprint{
		{Num: 1, Content: "Sprint 1 content"},
		{Num: 2, Content: "Sprint 2 content"},
		{Num: 3, Content: "Sprint 3 content"},
	}

	prompt := buildNextSprintPrompt("goal", "", sprints, nil, "out.md")

	if !strings.Contains(prompt, "### Sprint 1") {
		t.Error("prompt should contain sprint 1 heading")
	}
	if !strings.Contains(prompt, "### Sprint 2") {
		t.Error("prompt should contain sprint 2 heading")
	}
	if !strings.Contains(prompt, "### Sprint 3") {
		t.Error("prompt should contain sprint 3 heading")
	}
}

func TestFindSprintByNum(t *testing.T) {
	tmpDir := t.TempDir()

	// Create sprint files with different naming patterns
	files := []string{"01-initial.md", "02-features.md", "03-polish.md"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Find sprint 1
	result := findSprintByNum(tmpDir, 1)
	if result == "" {
		t.Fatal("expected to find sprint 1")
	}
	if !strings.HasSuffix(result, "01-initial.md") {
		t.Errorf("expected 01-initial.md, got %s", result)
	}

	// Find sprint 2
	result = findSprintByNum(tmpDir, 2)
	if result == "" {
		t.Fatal("expected to find sprint 2")
	}
	if !strings.HasSuffix(result, "02-features.md") {
		t.Errorf("expected 02-features.md, got %s", result)
	}

	// Missing sprint
	result = findSprintByNum(tmpDir, 99)
	if result != "" {
		t.Errorf("expected empty for missing sprint, got %s", result)
	}
}

func TestFindSprintByNum_EmptyDir(t *testing.T) {
	result := findSprintByNum("/nonexistent/dir", 1)
	if result != "" {
		t.Errorf("expected empty for nonexistent dir, got %s", result)
	}
}

func TestLoadCompletedSprintSummaries(t *testing.T) {
	tmpDir := t.TempDir()

	// Create sprint files
	os.WriteFile(filepath.Join(tmpDir, "01-initial.md"), []byte("# Sprint 1\nFirst"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "02-features.md"), []byte("# Sprint 2\nSecond"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "03-polish.md"), []byte("# Sprint 3\nThird"), 0644)

	// Load up to sprint 2
	results := loadCompletedSprintSummaries(tmpDir, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 sprints, got %d", len(results))
	}

	// Should be sorted by number
	if results[0].Num != 1 {
		t.Errorf("first sprint should be num 1, got %d", results[0].Num)
	}
	if results[1].Num != 2 {
		t.Errorf("second sprint should be num 2, got %d", results[1].Num)
	}

	// Content should be loaded
	if !strings.Contains(results[0].Content, "Sprint 1") {
		t.Error("first sprint content should contain 'Sprint 1'")
	}
	if !strings.Contains(results[1].Content, "Sprint 2") {
		t.Error("second sprint content should contain 'Sprint 2'")
	}

	// Load all 3
	results = loadCompletedSprintSummaries(tmpDir, 3)
	if len(results) != 3 {
		t.Fatalf("expected 3 sprints, got %d", len(results))
	}
}

// TestFindCurrentSprintFS_PicksUpNewSprint verifies that after sprint 01 is complete
// and a new 02-next.md file is written, FindCurrentSprintFS finds it as the current sprint.
func TestFindCurrentSprintFS_PicksUpNewSprint(t *testing.T) {
	tmpDir := t.TempDir()
	sprintsDir := filepath.Join(tmpDir, ".ai", "sprints")
	os.MkdirAll(sprintsDir, 0755)

	// Sprint 01 is complete (all tasks checked)
	sprint01 := "# Sprint 1\n\n- [x] Set up project\n  - [x] go-coder: Init module\n  - [x] _reviewer: Validate\n"
	os.WriteFile(filepath.Join(sprintsDir, "01-initial.md"), []byte(sprint01), 0644)

	// Before new sprint exists, FindCurrentSprintFS should return the last (complete) sprint
	fsys := os.DirFS(tmpDir)
	path, num := FindCurrentSprintFS(fsys)
	if num != 1 {
		t.Errorf("expected sprint 1 as current (last complete), got %d", num)
	}
	if path != ".ai/sprints/01-initial.md" {
		t.Errorf("expected .ai/sprints/01-initial.md, got %s", path)
	}

	// Now write a new 02-next.md (incomplete)
	sprint02 := "# Sprint 2\n\n- [ ] Implement feature X\n  - [ ] go-coder: Write code\n  - [ ] _reviewer: Review\n"
	os.WriteFile(filepath.Join(sprintsDir, "02-next.md"), []byte(sprint02), 0644)

	// FindCurrentSprintFS should now find the incomplete sprint 02
	path, num = FindCurrentSprintFS(fsys)
	if num != 2 {
		t.Errorf("expected sprint 2 as current (incomplete), got %d", num)
	}
	if path != ".ai/sprints/02-next.md" {
		t.Errorf("expected .ai/sprints/02-next.md, got %s", path)
	}
}

// TestFindSprintByNum_DifferentNamingPatterns verifies findSprintByNum works
// with various naming conventions (not just NN-sprint.md).
func TestFindSprintByNum_DifferentNamingPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	files := map[string]int{
		"01-initial.md":  1,
		"02-next.md":     2,
		"03-features.md": 3,
	}
	for name := range files {
		os.WriteFile(filepath.Join(tmpDir, name), []byte("content"), 0644)
	}

	for name, num := range files {
		result := findSprintByNum(tmpDir, num)
		if result == "" {
			t.Errorf("expected to find sprint %d (%s)", num, name)
			continue
		}
		if !strings.HasSuffix(result, name) {
			t.Errorf("sprint %d: expected %s, got %s", num, name, result)
		}
	}
}

// TestAssessGoalAndPlanNext_NextSprintAlreadyExists verifies the fast path
// when the next sprint file already exists on disk.
func TestAssessGoalAndPlanNext_NextSprintAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	sprintsDir := filepath.Join(tmpDir, ".ai", "sprints")
	os.MkdirAll(sprintsDir, 0755)

	// Create sprint 01 (complete)
	os.WriteFile(filepath.Join(sprintsDir, "01-initial.md"),
		[]byte("# Sprint 1\n\n- [x] Done"), 0644)
	// Create sprint 02 (already exists)
	os.WriteFile(filepath.Join(sprintsDir, "02-next.md"),
		[]byte("# Sprint 2\n\n- [ ] Next task"), 0644)

	// assessGoalAndPlanNext should return immediately without calling an agent
	proj := project.New(tmpDir)
	result, err := assessGoalAndPlanNext(tmpDir, proj, 1, NextOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.MoreWork {
		t.Error("expected MoreWork=true when next sprint exists")
	}
	if !strings.Contains(result.Message, "sprint 2") {
		t.Errorf("expected message to mention sprint 2, got: %s", result.Message)
	}
}
