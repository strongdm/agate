package workflow

import (
	"fmt"
	"strings"
	"testing"

	"github.com/strongdm/agate/internal/project"
)

func TestBuildSprintsPromptSkillNames(t *testing.T) {
	goal := &project.Goal{Content: "Build a sample app"}

	tests := []struct {
		name       string
		skills     []string
		wantCoder  string
		wantAvail  []string
		noCoder    string // should NOT appear
	}{
		{
			name:      "go project",
			skills:    []string{"go-coder", "go-reviewer", "test-writer"},
			wantCoder: "go-coder",
			wantAvail: []string{"go-coder", "go-reviewer", "test-writer", "_reviewer"},
			noCoder:   "rust-coder",
		},
		{
			name:      "rust project",
			skills:    []string{"rust-coder", "cli-designer"},
			wantCoder: "rust-coder",
			wantAvail: []string{"rust-coder", "cli-designer", "_reviewer"},
			noCoder:   "go-coder",
		},
		{
			name:      "python project",
			skills:    []string{"python-coder", "python-reviewer"},
			wantCoder: "python-coder",
			wantAvail: []string{"python-coder", "python-reviewer", "_reviewer"},
			noCoder:   "go-coder",
		},
		{
			name:      "generic project",
			skills:    []string{"coder", "reviewer"},
			wantCoder: "coder",
			wantAvail: []string{"coder", "reviewer", "_reviewer"},
			noCoder:   "go-coder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := buildSprintsPromptWithContext(goal, "design doc", "", "/tmp/sprint.md", tt.skills)

			// Should contain the expected coder in examples
			if !strings.Contains(prompt, fmt.Sprintf("  - [ ] %s:", tt.wantCoder)) {
				t.Errorf("prompt should contain subtask with %q, got:\n%s", tt.wantCoder, prompt)
			}

			// Should NOT contain the wrong coder
			if tt.noCoder != "" && strings.Contains(prompt, tt.noCoder) {
				t.Errorf("prompt should NOT contain %q", tt.noCoder)
			}

			// Available skills line should list all expected skills
			for _, s := range tt.wantAvail {
				if !strings.Contains(prompt, s) {
					t.Errorf("prompt should contain skill %q in available list", s)
				}
			}
		})
	}
}
