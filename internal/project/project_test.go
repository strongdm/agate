package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProject_HasGoal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agate-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	p := New(tmpDir)

	// No goal initially
	if p.HasGoal() {
		t.Error("expected HasGoal to be false")
	}

	// Create GOAL.md
	if err := os.WriteFile(filepath.Join(tmpDir, "GOAL.md"), []byte("test goal"), 0644); err != nil {
		t.Fatal(err)
	}

	// Now has goal
	if !p.HasGoal() {
		t.Error("expected HasGoal to be true")
	}
}

func TestProject_EnsureDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agate-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	p := New(tmpDir)
	if err := p.EnsureDirectories(); err != nil {
		t.Fatalf("failed to ensure directories: %v", err)
	}

	// Check directories exist
	dirs := []string{
		p.DesignDir(),
		p.SprintsDir(),
		p.SkillsDir(),
		p.DataDir(),
		p.DraftsDir(),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("directory not created: %s", dir)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"Build a Go CLI tool", "go"},
		{"Create a Python web scraper", "python"},
		{"Write a Rust game engine", "rust"},
		{"Build a JavaScript app", "javascript"},
		{"Create a TypeScript library", "typescript"},
		{"Something without a language", "unknown"},
	}

	for _, tt := range tests {
		got := detectLanguage(tt.content)
		if got != tt.expected {
			t.Errorf("detectLanguage(%q): expected %s, got %s", tt.content, tt.expected, got)
		}
	}
}

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"Build a CLI tool", "cli"},
		{"Create a command line interface", "cli"},
		{"Build a web application", "webapp"},
		{"Create a REST API", "api"},
		{"Build a library", "library"},
		{"Something general", "general"},
	}

	for _, tt := range tests {
		got := detectProjectType(tt.content)
		if got != tt.expected {
			t.Errorf("detectProjectType(%q): expected %s, got %s", tt.content, tt.expected, got)
		}
	}
}

func TestLoadSkills_UserOverride(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agate-skills-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a builtin skill (_reviewer.md)
	builtinContent := `---
name: _reviewer
agents: [claude]
phase: review
can_modify_checkboxes: true
version: 1
---

# Builtin Reviewer

This is the built-in reviewer content.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "_reviewer.md"), []byte(builtinContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a user override (reviewer.md)
	userContent := `---
name: reviewer
agents: [claude]
phase: review
can_modify_checkboxes: true
version: 1
---

# User Custom Rules

These are user-specific rules.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "reviewer.md"), []byte(userContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load skills
	skills, err := LoadSkills(tmpDir)
	if err != nil {
		t.Fatalf("failed to load skills: %v", err)
	}

	// Find _reviewer skill
	var reviewerSkill *Skill
	for i := range skills {
		if skills[i].Name == "_reviewer" {
			reviewerSkill = &skills[i]
			break
		}
	}

	if reviewerSkill == nil {
		t.Fatal("_reviewer skill not found")
	}

	// Verify the user content was merged
	if !contains(reviewerSkill.Content, "User Custom Rules") {
		t.Error("user override content not merged into builtin skill")
	}

	if !contains(reviewerSkill.Content, "Builtin Reviewer") {
		t.Error("builtin content should still be present")
	}

	// Verify standalone reviewer.md was removed (merged into _reviewer)
	for _, s := range skills {
		if s.Name == "reviewer" {
			t.Error("standalone reviewer.md should have been merged and removed")
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
