package workflow

import (
	"io/fs"
	"path/filepath"

	"github.com/strongdm/agate/internal/fsutil"
	"github.com/strongdm/agate/internal/logging"
)

// StatusResult captures the detected workflow state from a filesystem
type StatusResult struct {
	// Goal
	HasGoal bool

	// Phase
	Phase PlanPhase // interview, design, decisions, sprint, execution

	// Interview
	InterviewExists   bool
	InterviewComplete bool // uses logging.ParseInterviewStatus()

	// Design
	HasDesignOverview  bool
	HasDesignDecisions bool
	DesignFiles        []string

	// Skills
	Skills []string

	// Sprint (execution phase)
	CurrentSprintPath string       // relative path, e.g. ".ai/sprints/01-initial.md"
	CurrentSprintNum  int          // sprint number (1, 2, etc.)
	Sprint            *SprintState // parsed sprint with checkbox states
}

// GetStatus detects workflow state from an abstract filesystem.
// This function contains NO os.* calls - it only uses fs.FS operations.
func GetStatus(fsys fs.FS) StatusResult {
	result := StatusResult{}

	// Check for GOAL.md
	result.HasGoal = fsExists(fsys, "GOAL.md")
	if !result.HasGoal {
		result.Phase = PhaseInterview
		return result
	}

	// Check interview status
	interviewPath := filepath.Join(".ai", "interview.md")
	result.InterviewExists = fsExists(fsys, interviewPath)
	if result.InterviewExists {
		content, err := fs.ReadFile(fsys, interviewPath)
		if err == nil {
			result.InterviewComplete = logging.ParseInterviewStatus(string(content))
		}
	}

	// Check design files
	result.HasDesignOverview = fsExists(fsys, filepath.Join(".ai", "design", "overview.md"))
	result.HasDesignDecisions = fsExists(fsys, filepath.Join(".ai", "design", "decisions.md"))
	result.DesignFiles = fsutil.ListMarkdownFilesFS(fsys, filepath.Join(".ai", "design"))

	// Check skills
	result.Skills = fsutil.ListMarkdownFilesFS(fsys, filepath.Join(".ai", "skills"))

	// Find current sprint
	sprintPath, sprintNum := FindCurrentSprintFS(fsys)
	result.CurrentSprintPath = sprintPath
	result.CurrentSprintNum = sprintNum

	// Parse sprint if found
	if sprintPath != "" {
		sprint, err := ParseSprintFS(fsys, sprintPath)
		if err == nil {
			result.Sprint = sprint
		}
	}

	// Determine phase based on what exists
	result.Phase = derivePhase(result)

	return result
}

// derivePhase determines the current workflow phase from detected state
func derivePhase(r StatusResult) PlanPhase {
	// No goal = can't proceed
	if !r.HasGoal {
		return PhaseInterview
	}

	// Interview not done
	if !r.InterviewExists || !r.InterviewComplete {
		return PhaseInterview
	}

	// No design overview
	if !r.HasDesignOverview {
		return PhaseDesign
	}

	// No decisions
	if !r.HasDesignDecisions {
		return PhaseDecisions
	}

	// No sprint
	if r.CurrentSprintPath == "" {
		return PhaseSprint
	}

	return PhaseExecution
}

// fsExists checks if a path exists in the filesystem
func fsExists(fsys fs.FS, path string) bool {
	_, err := fs.Stat(fsys, path)
	return err == nil
}
