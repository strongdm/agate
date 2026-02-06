package project

import (
	"os"
	"regexp"
	"strings"
)

// Goal represents parsed GOAL.md content
type Goal struct {
	Content  string
	Language string
	Type     string
}

// ParseGoal reads and parses GOAL.md
func ParseGoal(path string) (*Goal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	goal := &Goal{
		Content:  content,
		Language: detectLanguage(content),
		Type:     detectProjectType(content),
	}

	return goal, nil
}

// detectLanguage attempts to detect the programming language from goal content
func detectLanguage(content string) string {
	lower := strings.ToLower(content)

	// Check for explicit language mentions
	patterns := map[string]*regexp.Regexp{
		"go":         regexp.MustCompile(`\b(go|golang)\b`),
		"python":     regexp.MustCompile(`\b(python|py)\b`),
		"rust":       regexp.MustCompile(`\b(rust)\b`),
		"javascript": regexp.MustCompile(`\b(javascript|js|node|nodejs)\b`),
		"typescript": regexp.MustCompile(`\b(typescript|ts)\b`),
		"java":       regexp.MustCompile(`\b(java)\b`),
		"ruby":       regexp.MustCompile(`\b(ruby)\b`),
		"c++":        regexp.MustCompile(`\b(c\+\+|cpp)\b`),
		"c":          regexp.MustCompile(`\b(c language|in c)\b`),
	}

	for lang, pattern := range patterns {
		if pattern.MatchString(lower) {
			return lang
		}
	}

	return "unknown"
}

// detectProjectType attempts to detect the project type from goal content
func detectProjectType(content string) string {
	lower := strings.ToLower(content)

	// Check for project type keywords
	if strings.Contains(lower, "cli") || strings.Contains(lower, "command line") || strings.Contains(lower, "command-line") {
		return "cli"
	}
	if strings.Contains(lower, "web app") || strings.Contains(lower, "webapp") || strings.Contains(lower, "web application") {
		return "webapp"
	}
	if strings.Contains(lower, "api") || strings.Contains(lower, "rest") || strings.Contains(lower, "graphql") {
		return "api"
	}
	if strings.Contains(lower, "library") || strings.Contains(lower, "package") || strings.Contains(lower, "module") {
		return "library"
	}
	if strings.Contains(lower, "mobile") || strings.Contains(lower, "ios") || strings.Contains(lower, "android") {
		return "mobile"
	}

	return "general"
}
