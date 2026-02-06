package workflow

import (
	"os"
	"testing"
	"testing/fstest"
)

// Test catalog: This file documents ALL possible workflow states
// and verifies GetStatus correctly detects each one.

func TestGetStatus_NoGoal(t *testing.T) {
	// Empty filesystem - no GOAL.md
	fsys := fstest.MapFS{}

	result := GetStatus(fsys)

	if result.HasGoal {
		t.Error("expected HasGoal=false for empty filesystem")
	}
	if result.Phase != PhaseInterview {
		t.Errorf("expected Phase=interview, got %s", result.Phase)
	}
}

func TestGetStatus_GoalOnly(t *testing.T) {
	// GOAL.md exists but no interview yet
	fsys := fstest.MapFS{
		"GOAL.md": &fstest.MapFile{Data: []byte("# My Project\n\nBuild something cool.")},
	}

	result := GetStatus(fsys)

	if !result.HasGoal {
		t.Error("expected HasGoal=true")
	}
	if result.InterviewExists {
		t.Error("expected InterviewExists=false")
	}
	if result.Phase != PhaseInterview {
		t.Errorf("expected Phase=interview (pending), got %s", result.Phase)
	}
}

func TestGetStatus_InterviewAwaiting(t *testing.T) {
	// Interview file exists but completion checkbox not checked
	fsys := fstest.MapFS{
		"GOAL.md": &fstest.MapFile{Data: []byte("# My Project")},
		".ai/interview.md": &fstest.MapFile{Data: []byte(`# Interview

## Questions

### Q1: Scope
What is the scope?

**Answer**: TBD

---
- [ ] All questions answered (check when complete)
`)},
	}

	result := GetStatus(fsys)

	if !result.InterviewExists {
		t.Error("expected InterviewExists=true")
	}
	if result.InterviewComplete {
		t.Error("expected InterviewComplete=false for unchecked completion box")
	}
	if result.Phase != PhaseInterview {
		t.Errorf("expected Phase=interview (awaiting answers), got %s", result.Phase)
	}
}

func TestGetStatus_InterviewComplete(t *testing.T) {
	// Interview file exists with completion checkbox checked
	fsys := fstest.MapFS{
		"GOAL.md": &fstest.MapFile{Data: []byte("# My Project")},
		".ai/interview.md": &fstest.MapFile{Data: []byte(`# Interview

## Questions

### Q1: Scope
What is the scope?

**Answer**: Build a CLI tool

---
- [x] All questions answered (check when complete)
`)},
	}

	result := GetStatus(fsys)

	if !result.InterviewExists {
		t.Error("expected InterviewExists=true")
	}
	if !result.InterviewComplete {
		t.Error("expected InterviewComplete=true for checked completion box")
	}
	if result.Phase != PhaseDesign {
		t.Errorf("expected Phase=design after completed interview, got %s", result.Phase)
	}
}

func TestGetStatus_InterviewCompleteLegacy(t *testing.T) {
	// Legacy format: "Status: COMPLETE" instead of checkbox
	fsys := fstest.MapFS{
		"GOAL.md": &fstest.MapFile{Data: []byte("# My Project")},
		".ai/interview.md": &fstest.MapFile{Data: []byte(`# Interview

Status: COMPLETE

## Questions
...
`)},
	}

	result := GetStatus(fsys)

	if !result.InterviewComplete {
		t.Error("expected InterviewComplete=true for legacy Status: COMPLETE format")
	}
	if result.Phase != PhaseDesign {
		t.Errorf("expected Phase=design, got %s", result.Phase)
	}
}

func TestGetStatus_DesignPhase(t *testing.T) {
	// Interview complete but no design yet
	fsys := fstest.MapFS{
		"GOAL.md":             &fstest.MapFile{Data: []byte("# My Project")},
		".ai/interview.md": &fstest.MapFile{Data: []byte("- [x] All questions answered")},
	}

	result := GetStatus(fsys)

	if result.Phase != PhaseDesign {
		t.Errorf("expected Phase=design, got %s", result.Phase)
	}
	if result.HasDesignOverview {
		t.Error("expected HasDesignOverview=false")
	}
}

func TestGetStatus_DecisionsPhase(t *testing.T) {
	// Design overview exists but no decisions
	fsys := fstest.MapFS{
		"GOAL.md":             &fstest.MapFile{Data: []byte("# My Project")},
		".ai/interview.md": &fstest.MapFile{Data: []byte("- [x] All questions answered")},
		".ai/design/overview.md":  &fstest.MapFile{Data: []byte("# Design Overview\n\n...")},
	}

	result := GetStatus(fsys)

	if !result.HasDesignOverview {
		t.Error("expected HasDesignOverview=true")
	}
	if result.HasDesignDecisions {
		t.Error("expected HasDesignDecisions=false")
	}
	if result.Phase != PhaseDecisions {
		t.Errorf("expected Phase=decisions, got %s", result.Phase)
	}
	if len(result.DesignFiles) != 1 || result.DesignFiles[0] != "overview.md" {
		t.Errorf("expected DesignFiles=[overview.md], got %v", result.DesignFiles)
	}
}

func TestGetStatus_SprintPhase(t *testing.T) {
	// Design complete but no sprint
	fsys := fstest.MapFS{
		"GOAL.md":              &fstest.MapFile{Data: []byte("# My Project")},
		".ai/interview.md":  &fstest.MapFile{Data: []byte("- [x] All questions answered")},
		".ai/design/overview.md":   &fstest.MapFile{Data: []byte("# Design Overview")},
		".ai/design/decisions.md":  &fstest.MapFile{Data: []byte("# Technical Decisions")},
	}

	result := GetStatus(fsys)

	if !result.HasDesignDecisions {
		t.Error("expected HasDesignDecisions=true")
	}
	if result.Phase != PhaseSprint {
		t.Errorf("expected Phase=sprint, got %s", result.Phase)
	}
}

func TestGetStatus_ExecutionPhase(t *testing.T) {
	// Full planning complete with sprint
	fsys := fstest.MapFS{
		"GOAL.md":              &fstest.MapFile{Data: []byte("# My Project")},
		".ai/interview.md":  &fstest.MapFile{Data: []byte("- [x] All questions answered")},
		".ai/design/overview.md":   &fstest.MapFile{Data: []byte("# Design Overview")},
		".ai/design/decisions.md":  &fstest.MapFile{Data: []byte("# Technical Decisions")},
		".ai/sprints/01-initial.md": &fstest.MapFile{Data: []byte(`# Sprint 1

## Tasks

- [ ] Set up project
  - [ ] go-coder: Create main.go
  - [ ] _reviewer: Validate structure
`)},
	}

	result := GetStatus(fsys)

	if result.Phase != PhaseExecution {
		t.Errorf("expected Phase=execution, got %s", result.Phase)
	}
	if result.CurrentSprintPath != ".ai/sprints/01-initial.md" {
		t.Errorf("expected CurrentSprintPath=sprints/01-initial.md, got %s", result.CurrentSprintPath)
	}
	if result.CurrentSprintNum != 1 {
		t.Errorf("expected CurrentSprintNum=1, got %d", result.CurrentSprintNum)
	}
	if result.Sprint == nil {
		t.Error("expected Sprint to be parsed")
	}
}

func TestGetStatus_SprintProgress(t *testing.T) {
	// Sprint with some tasks complete
	fsys := fstest.MapFS{
		"GOAL.md":              &fstest.MapFile{Data: []byte("# My Project")},
		".ai/interview.md":  &fstest.MapFile{Data: []byte("- [x] All questions answered")},
		".ai/design/overview.md":   &fstest.MapFile{Data: []byte("# Design")},
		".ai/design/decisions.md":  &fstest.MapFile{Data: []byte("# Decisions")},
		".ai/sprints/01-initial.md": &fstest.MapFile{Data: []byte(`# Sprint 1

## Tasks

- [x] Set up project
  - [x] go-coder: Create main.go
  - [x] _reviewer: Validate structure

- [ ] Implement feature
  - [x] go-coder: Write code
  - [ ] _reviewer: Review code
`)},
	}

	result := GetStatus(fsys)

	if result.Sprint == nil {
		t.Fatal("expected Sprint to be parsed")
	}
	if result.Sprint.IsComplete() {
		t.Error("expected sprint to NOT be complete")
	}

	// Check task progress
	completed, total := result.Sprint.GetProgress()
	if completed != 1 || total != 2 {
		t.Errorf("expected 1/2 tasks complete, got %d/%d", completed, total)
	}
}

func TestGetStatus_SprintComplete(t *testing.T) {
	// All sprint tasks complete
	fsys := fstest.MapFS{
		"GOAL.md":              &fstest.MapFile{Data: []byte("# My Project")},
		".ai/interview.md":  &fstest.MapFile{Data: []byte("- [x] All questions answered")},
		".ai/design/overview.md":   &fstest.MapFile{Data: []byte("# Design")},
		".ai/design/decisions.md":  &fstest.MapFile{Data: []byte("# Decisions")},
		".ai/sprints/01-initial.md": &fstest.MapFile{Data: []byte(`# Sprint 1

## Tasks

- [x] Set up project
  - [x] go-coder: Create main.go
  - [x] _reviewer: Validate structure

- [x] Implement feature
  - [x] go-coder: Write code
  - [x] _reviewer: Review code
`)},
	}

	result := GetStatus(fsys)

	if result.Sprint == nil {
		t.Fatal("expected Sprint to be parsed")
	}
	if !result.Sprint.IsComplete() {
		t.Error("expected sprint to be complete")
	}
}

func TestGetStatus_MultiSprintFirstIncomplete(t *testing.T) {
	// Sprint 1 complete, Sprint 2 in progress
	fsys := fstest.MapFS{
		"GOAL.md":              &fstest.MapFile{Data: []byte("# My Project")},
		".ai/interview.md":  &fstest.MapFile{Data: []byte("- [x] All questions answered")},
		".ai/design/overview.md":   &fstest.MapFile{Data: []byte("# Design")},
		".ai/design/decisions.md":  &fstest.MapFile{Data: []byte("# Decisions")},
		".ai/sprints/01-initial.md": &fstest.MapFile{Data: []byte(`# Sprint 1

## Tasks

- [x] Task 1
  - [x] go-coder: Do thing
  - [x] _reviewer: Review
`)},
		".ai/sprints/02-features.md": &fstest.MapFile{Data: []byte(`# Sprint 2

## Tasks

- [ ] Task 2
  - [ ] go-coder: Do more
  - [ ] _reviewer: Review more
`)},
	}

	result := GetStatus(fsys)

	// Should pick sprint 2 as it's the first incomplete
	if result.CurrentSprintPath != ".ai/sprints/02-features.md" {
		t.Errorf("expected CurrentSprintPath=sprints/02-features.md, got %s", result.CurrentSprintPath)
	}
	if result.CurrentSprintNum != 2 {
		t.Errorf("expected CurrentSprintNum=2, got %d", result.CurrentSprintNum)
	}
}

func TestGetStatus_Skills(t *testing.T) {
	// Check skills detection
	fsys := fstest.MapFS{
		"GOAL.md":               &fstest.MapFile{Data: []byte("# My Project")},
		".ai/skills/_reviewer.md":   &fstest.MapFile{Data: []byte("# Reviewer skill")},
		".ai/skills/_planner.md":    &fstest.MapFile{Data: []byte("# Planner skill")},
		".ai/skills/go-coder.md":    &fstest.MapFile{Data: []byte("# Go coder skill")},
	}

	result := GetStatus(fsys)

	if len(result.Skills) != 3 {
		t.Errorf("expected 3 skills, got %d: %v", len(result.Skills), result.Skills)
	}
}

// Test ParseSprintContent separately
func TestParseSprintContent(t *testing.T) {
	content := `# Sprint 1

## Tasks

- [ ] First task
  - [ ] go-coder: Create file
  - [ ] _reviewer: Validate

- [x] Second task
  - [x] go-coder: Done
  - [x] _reviewer: Reviewed
`

	sprint, err := ParseSprintContent(content)
	if err != nil {
		t.Fatalf("ParseSprintContent failed: %v", err)
	}

	if len(sprint.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(sprint.Tasks))
	}

	if sprint.Tasks[0].Checked {
		t.Error("first task should not be checked")
	}
	if !sprint.Tasks[1].Checked {
		t.Error("second task should be checked")
	}

	if len(sprint.Tasks[0].SubTasks) != 2 {
		t.Errorf("expected 2 subtasks in first task, got %d", len(sprint.Tasks[0].SubTasks))
	}
}

func TestParseSprintContent_NoSubtaskTask(t *testing.T) {
	// Task with no subtasks — GetNextSubTask should return nil, IsComplete false
	content := `# Sprint 1

## Tasks

- [ ] Task with no subtasks
`

	sprint, err := ParseSprintContent(content)
	if err != nil {
		t.Fatalf("ParseSprintContent failed: %v", err)
	}

	if len(sprint.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(sprint.Tasks))
	}

	if len(sprint.Tasks[0].SubTasks) != 0 {
		t.Errorf("expected 0 subtasks, got %d", len(sprint.Tasks[0].SubTasks))
	}

	// GetNextSubTask returns nil (no subtasks to work on)
	if sub := sprint.GetNextSubTask(); sub != nil {
		t.Error("expected GetNextSubTask to return nil for task with no subtasks")
	}

	// Sprint is not complete (task is unchecked)
	if sprint.IsComplete() {
		t.Error("expected sprint to NOT be complete")
	}

	// AllSubTasksComplete returns false for no subtasks (requires len > 0)
	if sprint.AllSubTasksComplete(0) {
		t.Error("expected AllSubTasksComplete=false for task with no subtasks")
	}
}

func TestParseSprintContent_OrphanedTask(t *testing.T) {
	// Task where all subtasks are checked but top-level task is not
	content := `# Sprint 1

## Tasks

- [ ] Orphaned task
  - [x] go-coder: Do work
  - [x] _reviewer: Review work
`

	sprint, err := ParseSprintContent(content)
	if err != nil {
		t.Fatalf("ParseSprintContent failed: %v", err)
	}

	// All subtasks are complete
	if !sprint.AllSubTasksComplete(0) {
		t.Error("expected AllSubTasksComplete=true")
	}

	// But task is not checked
	if sprint.Tasks[0].Checked {
		t.Error("expected task to NOT be checked")
	}

	// GetNextSubTask returns nil (all subtasks done)
	if sub := sprint.GetNextSubTask(); sub != nil {
		t.Error("expected GetNextSubTask to return nil when all subtasks done but task unchecked")
	}

	// Sprint is not complete
	if sprint.IsComplete() {
		t.Error("expected sprint to NOT be complete")
	}
}

func TestParseSprintContent_FailureMarkers(t *testing.T) {
	content := `# Sprint 1

## Tasks

- [ ] ❌❌ Problematic task
  - [ ] go-coder: Fix bugs
  - [ ] _reviewer: Review fix
`

	sprint, err := ParseSprintContent(content)
	if err != nil {
		t.Fatalf("ParseSprintContent failed: %v", err)
	}

	if sprint.Tasks[0].FailureCount != 2 {
		t.Errorf("expected FailureCount=2, got %d", sprint.Tasks[0].FailureCount)
	}
}

func TestParseSprintContent_ReplanMarker(t *testing.T) {
	content := `# Sprint 1

## Tasks

- [ ] 🔄 Replanned task
  - [ ] go-coder: Fix bugs
  - [ ] _reviewer: Review fix
`

	sprint, err := ParseSprintContent(content)
	if err != nil {
		t.Fatalf("ParseSprintContent failed: %v", err)
	}

	if sprint.Tasks[0].ReplanCount != 1 {
		t.Errorf("expected ReplanCount=1, got %d", sprint.Tasks[0].ReplanCount)
	}
	if sprint.Tasks[0].FailureCount != 0 {
		t.Errorf("expected FailureCount=0, got %d", sprint.Tasks[0].FailureCount)
	}
	if sprint.Tasks[0].Text != "Replanned task" {
		t.Errorf("expected Text='Replanned task', got %q", sprint.Tasks[0].Text)
	}
}

func TestParseSprintContent_MixedMarkers(t *testing.T) {
	content := `# Sprint 1

## Tasks

- [ ] ❌🔄❌ Mixed markers task
  - [ ] go-coder: Fix bugs
  - [ ] _reviewer: Review fix
`

	sprint, err := ParseSprintContent(content)
	if err != nil {
		t.Fatalf("ParseSprintContent failed: %v", err)
	}

	if sprint.Tasks[0].FailureCount != 2 {
		t.Errorf("expected FailureCount=2, got %d", sprint.Tasks[0].FailureCount)
	}
	if sprint.Tasks[0].ReplanCount != 1 {
		t.Errorf("expected ReplanCount=1, got %d", sprint.Tasks[0].ReplanCount)
	}
}

func TestClearFailures(t *testing.T) {
	content := `# Sprint 1

## Tasks

- [ ] ❌🔄❌ Task with mixed markers
  - [ ] go-coder: Fix bugs
`

	sprint, err := ParseSprintContent(content)
	if err != nil {
		t.Fatalf("ParseSprintContent failed: %v", err)
	}

	// Write to temp file for ClearFailures to work
	tmpFile := t.TempDir() + "/sprint.md"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	sprint.FilePath = tmpFile

	if err := sprint.ClearFailures(0); err != nil {
		t.Fatalf("ClearFailures failed: %v", err)
	}

	// Re-parse to verify
	updated, err := ParseSprint(tmpFile)
	if err != nil {
		t.Fatalf("ParseSprint failed: %v", err)
	}

	if updated.Tasks[0].FailureCount != 0 {
		t.Errorf("expected FailureCount=0 after clearing, got %d", updated.Tasks[0].FailureCount)
	}
	if updated.Tasks[0].ReplanCount != 1 {
		t.Errorf("expected ReplanCount=1 preserved, got %d", updated.Tasks[0].ReplanCount)
	}
}

func TestAddReplanMarker(t *testing.T) {
	content := `# Sprint 1

## Tasks

- [ ] ❌❌ Task with failures
  - [ ] go-coder: Fix bugs
`

	sprint, err := ParseSprintContent(content)
	if err != nil {
		t.Fatalf("ParseSprintContent failed: %v", err)
	}

	// Write to temp file for AddReplanMarker to work
	tmpFile := t.TempDir() + "/sprint.md"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	sprint.FilePath = tmpFile

	if err := sprint.AddReplanMarker(0); err != nil {
		t.Fatalf("AddReplanMarker failed: %v", err)
	}

	// Re-parse to verify
	updated, err := ParseSprint(tmpFile)
	if err != nil {
		t.Fatalf("ParseSprint failed: %v", err)
	}

	if updated.Tasks[0].ReplanCount != 1 {
		t.Errorf("expected ReplanCount=1, got %d", updated.Tasks[0].ReplanCount)
	}
	if updated.Tasks[0].FailureCount != 2 {
		t.Errorf("expected FailureCount=2 preserved, got %d", updated.Tasks[0].FailureCount)
	}
}
