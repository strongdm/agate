package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StatusWithResult generates the status output and returns the StatusResult.
// This allows callers to use GetExitCode on the result.
func StatusWithResult(projectDir string) (string, StatusResult, error) {
	fsys := os.DirFS(projectDir)
	result := GetStatus(fsys)
	output, err := formatStatus(projectDir, result)
	return output, result, err
}

// Status generates the status output from markdown files.
// Uses GetStatus(fs.FS) for detection, then formats the output.
func Status(projectDir string) (string, error) {
	output, _, err := StatusWithResult(projectDir)
	return output, err
}

func formatStatus(projectDir string, result StatusResult) (string, error) {
	projectName := filepath.Base(projectDir)

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("%s\n", projectName))
	sb.WriteString(strings.Repeat("=", 40))
	sb.WriteString("\n\n")

	// Goal status
	if result.HasGoal {
		sb.WriteString("GOAL     -> GOAL.md\n")
	} else {
		sb.WriteString("GOAL     (missing) Create GOAL.md to get started\n")
		sb.WriteString("\n")
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")
		sb.WriteString("Next: Create GOAL.md describing what you want to build\n")
		return sb.String(), nil
	}

	// Show planning phase status
	if result.Phase != PhaseExecution {
		sb.WriteString(fmt.Sprintf("PHASE    %s\n", result.Phase))
	}

	// Interview status
	if result.InterviewExists {
		if result.InterviewComplete {
			sb.WriteString("INTERVIEW + complete\n")
		} else {
			sb.WriteString("INTERVIEW + awaiting answers\n")
			sb.WriteString("         -> .ai/interview.md\n")
		}
	} else if result.Phase == PhaseInterview {
		sb.WriteString("INTERVIEW (pending)\n")
	}

	// Design status
	if result.HasDesignOverview {
		sb.WriteString("DESIGN   + complete\n")
		for _, f := range result.DesignFiles {
			sb.WriteString(fmt.Sprintf("         -> .ai/design/%s\n", f))
		}
	} else if result.Phase == PhaseDesign || result.Phase == PhaseDecisions || result.Phase == PhaseSprint || result.Phase == PhaseExecution {
		sb.WriteString("DESIGN   (pending)\n")
	}

	// Skills status
	if len(result.Skills) > 0 {
		sb.WriteString(fmt.Sprintf("SKILLS   + %d generated\n", len(result.Skills)))
		for _, s := range result.Skills {
			sb.WriteString(fmt.Sprintf("         -> .ai/skills/%s\n", s))
		}
	}

	// Sprint status
	if result.Sprint != nil {
		// Show visual progress bar
		progressBar := result.Sprint.RenderProgressBar(result.CurrentSprintNum, -1, -1)
		sb.WriteString(fmt.Sprintf("SPRINT   %s\n", progressBar))

		// Show next sub-task if not complete
		if !result.Sprint.IsComplete() {
			nextSub := result.Sprint.GetNextSubTask()
			if nextSub != nil {
				sb.WriteString(fmt.Sprintf("         Next: [%s] %s\n", nextSub.Skill, TruncateText(nextSub.Text, 40)))
			}
		}
	} else if result.Phase == PhaseSprint || result.Phase == PhaseExecution {
		sb.WriteString("SPRINT   (pending)\n")
	}

	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("-", 40))
	sb.WriteString("\n")

	// Next action based on detected state
	nextAction := getNextActionFromResult(result)
	sb.WriteString(fmt.Sprintf("Next: %s\n", nextAction))

	return sb.String(), nil
}

// getNextActionFromResult derives the next action from StatusResult
func getNextActionFromResult(result StatusResult) string {
	if !result.HasGoal {
		return "Create GOAL.md describing what you want to build"
	}

	// Planning phases
	if result.Phase != PhaseExecution {
		// Special case: interview exists but not complete
		if result.Phase == PhaseInterview && result.InterviewExists && !result.InterviewComplete {
			return "Answer questions in .ai/interview.md, then check completion box"
		}
		return fmt.Sprintf("agate next (%s)", GetNextPlanAction(result.Phase))
	}

	// Execution phase
	if result.Sprint == nil {
		return "agate next (generate sprints)"
	}

	if result.Sprint.IsComplete() {
		return "All tasks complete! Check for more sprints."
	}

	nextSub := result.Sprint.GetNextSubTask()
	if nextSub != nil {
		return fmt.Sprintf("agate next ([%s] %s)", nextSub.Skill, TruncateText(nextSub.Text, 30))
	}

	return "agate next"
}
