package project

import (
	"os"
	"path/filepath"

	"github.com/strongdm/agate/internal/fsutil"
)

// Project represents an agate project
type Project struct {
	Dir string
}

// New creates a new project instance
func New(dir string) *Project {
	return &Project{Dir: dir}
}

// GoalPath returns the path to GOAL.md
func (p *Project) GoalPath() string {
	return filepath.Join(p.Dir, "GOAL.md")
}

// HasGoal checks if GOAL.md exists
func (p *Project) HasGoal() bool {
	_, err := os.Stat(p.GoalPath())
	return err == nil
}

// DesignDir returns the path to the design directory
func (p *Project) DesignDir() string {
	return filepath.Join(p.Dir, ".ai", "design")
}

// SprintsDir returns the path to the sprints directory
func (p *Project) SprintsDir() string {
	return filepath.Join(p.Dir, ".ai", "sprints")
}

// SkillsDir returns the path to the skills directory
func (p *Project) SkillsDir() string {
	return filepath.Join(p.Dir, ".ai", "skills")
}

// DataDir returns the path to the .ai directory
func (p *Project) DataDir() string {
	return filepath.Join(p.Dir, ".ai")
}

// DraftsDir returns the path to the design drafts directory
func (p *Project) DraftsDir() string {
	return filepath.Join(p.Dir, ".ai", "design", ".drafts")
}

// EnsureDirectories creates the required project directories
func (p *Project) EnsureDirectories() error {
	dirs := []string{
		p.DesignDir(),
		p.SprintsDir(),
		p.SkillsDir(),
		p.DataDir(),
		p.DraftsDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// HasDesign checks if design documents exist
func (p *Project) HasDesign() bool {
	overview := filepath.Join(p.DesignDir(), "overview.md")
	_, err := os.Stat(overview)
	return err == nil
}

// HasSprints checks if sprint documents exist
func (p *Project) HasSprints() bool {
	return len(fsutil.ListMarkdownFiles(p.SprintsDir())) > 0
}

// HasSkills checks if skill documents exist
func (p *Project) HasSkills() bool {
	return len(fsutil.ListMarkdownFiles(p.SkillsDir())) > 0
}

// ListSprints returns the list of sprint files
func (p *Project) ListSprints() ([]string, error) {
	return fsutil.ListMarkdownFiles(p.SprintsDir()), nil
}

// ListSkills returns the list of skill files
func (p *Project) ListSkills() ([]string, error) {
	return fsutil.ListMarkdownFiles(p.SkillsDir()), nil
}

// ListDesignFiles returns the list of design files
func (p *Project) ListDesignFiles() ([]string, error) {
	return fsutil.ListMarkdownFiles(p.DesignDir()), nil
}
