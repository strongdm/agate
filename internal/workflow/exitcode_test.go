package workflow

import "testing"

func TestGetExitCode_NoGoal(t *testing.T) {
	r := StatusResult{HasGoal: false}
	if code := GetExitCode(r); code != ExitHumanNeeded {
		t.Errorf("expected %d (human needed), got %d", ExitHumanNeeded, code)
	}
}

func TestGetExitCode_InterviewAwaiting(t *testing.T) {
	r := StatusResult{
		HasGoal:           true,
		InterviewExists:   true,
		InterviewComplete: false,
		Phase:             PhaseInterview,
	}
	if code := GetExitCode(r); code != ExitHumanNeeded {
		t.Errorf("expected %d (human needed), got %d", ExitHumanNeeded, code)
	}
}

func TestGetExitCode_InterviewNotStarted(t *testing.T) {
	// Goal exists but no interview file yet - automation can generate it
	r := StatusResult{
		HasGoal:         true,
		InterviewExists: false,
		Phase:           PhaseInterview,
	}
	if code := GetExitCode(r); code != ExitMoreWork {
		t.Errorf("expected %d (more work), got %d", ExitMoreWork, code)
	}
}

func TestGetExitCode_DesignPhase(t *testing.T) {
	r := StatusResult{
		HasGoal:           true,
		InterviewExists:   true,
		InterviewComplete: true,
		Phase:             PhaseDesign,
	}
	if code := GetExitCode(r); code != ExitMoreWork {
		t.Errorf("expected %d (more work), got %d", ExitMoreWork, code)
	}
}

func TestGetExitCode_DecisionsPhase(t *testing.T) {
	r := StatusResult{
		HasGoal:           true,
		InterviewExists:   true,
		InterviewComplete: true,
		Phase:             PhaseDecisions,
	}
	if code := GetExitCode(r); code != ExitMoreWork {
		t.Errorf("expected %d (more work), got %d", ExitMoreWork, code)
	}
}

func TestGetExitCode_SprintPhase(t *testing.T) {
	r := StatusResult{
		HasGoal:           true,
		InterviewExists:   true,
		InterviewComplete: true,
		Phase:             PhaseSprint,
	}
	if code := GetExitCode(r); code != ExitMoreWork {
		t.Errorf("expected %d (more work), got %d", ExitMoreWork, code)
	}
}

func TestGetExitCode_ExecutionNoSprint(t *testing.T) {
	r := StatusResult{
		HasGoal: true,
		Phase:   PhaseExecution,
		Sprint:  nil,
	}
	if code := GetExitCode(r); code != ExitMoreWork {
		t.Errorf("expected %d (more work), got %d", ExitMoreWork, code)
	}
}

func TestGetExitCode_ExecutionSprintIncomplete(t *testing.T) {
	sprint := &SprintState{
		Tasks: []Task{
			{Checked: true},
			{Checked: false},
		},
	}
	r := StatusResult{
		HasGoal: true,
		Phase:   PhaseExecution,
		Sprint:  sprint,
	}
	if code := GetExitCode(r); code != ExitMoreWork {
		t.Errorf("expected %d (more work), got %d", ExitMoreWork, code)
	}
}

func TestGetExitCode_ExecutionSprintComplete(t *testing.T) {
	sprint := &SprintState{
		Tasks: []Task{
			{Checked: true},
			{Checked: true},
		},
	}
	r := StatusResult{
		HasGoal: true,
		Phase:   PhaseExecution,
		Sprint:  sprint,
	}
	if code := GetExitCode(r); code != ExitDone {
		t.Errorf("expected %d (done), got %d", ExitDone, code)
	}
}
