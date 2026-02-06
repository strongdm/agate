package workflow

// Exit code constants for agate commands.
const (
	ExitDone         = 0   // All work complete
	ExitMoreWork     = 1   // More work remains, automation can continue
	ExitError        = 2   // Error occurred
	ExitHumanNeeded  = 255 // Human action required
)

// GetExitCode determines the exit code from a StatusResult.
//
// Decision tree:
//   - No GOAL.md → 255 (human: create goal)
//   - Interview awaiting answers → 255 (human: answer questions)
//   - Planning phases incomplete → 1 (automation: run agate next)
//   - Sprint tasks incomplete → 1 (automation: run agate next)
//   - All sprints complete → 0 (done)
func GetExitCode(r StatusResult) int {
	// No goal = human must create one
	if !r.HasGoal {
		return ExitHumanNeeded
	}

	// Interview exists but not complete = human must answer
	if r.InterviewExists && !r.InterviewComplete {
		return ExitHumanNeeded
	}

	// Planning phases incomplete = automation can proceed
	if r.Phase != PhaseExecution {
		return ExitMoreWork
	}

	// Execution phase: check sprint state
	if r.Sprint == nil {
		return ExitMoreWork
	}

	if r.Sprint.IsComplete() {
		return ExitDone
	}

	return ExitMoreWork
}
