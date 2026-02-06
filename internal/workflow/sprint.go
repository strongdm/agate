package workflow

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Task represents a top-level task with sub-tasks
type Task struct {
	Index        int
	Text         string
	Checked      bool
	LineNum      int
	FailureCount int // Number of ❌ emojis before the task text
	ReplanCount  int // Number of 🔄 emojis before the task text
	SubTasks     []SubTask
}

// SubTask represents a sub-task with skill assignment
type SubTask struct {
	Index       int
	Skill       string
	Text        string
	Checked     bool
	LineNum     int
	ParentIndex int
}

// SprintState represents the parsed state of a sprint file
type SprintState struct {
	FilePath string
	Tasks    []Task
	Content  string
}

// ParseSprint parses a sprint file with nested checkboxes
func ParseSprint(sprintPath string) (*SprintState, error) {
	content, err := os.ReadFile(sprintPath)
	if err != nil {
		return nil, err
	}
	state, err := ParseSprintContent(string(content))
	if err != nil {
		return nil, err
	}
	state.FilePath = sprintPath
	return state, nil
}

// ParseSprintFS parses a sprint file from an fs.FS
func ParseSprintFS(fsys fs.FS, path string) (*SprintState, error) {
	content, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}
	state, err := ParseSprintContent(string(content))
	if err != nil {
		return nil, err
	}
	state.FilePath = path
	return state, nil
}

// ParseSprintContent parses sprint markdown content (pure, no filesystem access)
func ParseSprintContent(content string) (*SprintState, error) {
	state := &SprintState{
		Content: content,
	}

	// Parse nested checkboxes
	// Top-level: ^- \[([ xX])\] ((?:❌|🔄)*)\s*(.*)$ - captures checkbox, marker emojis, text
	// Sub-task:  ^  - \[([ xX])\] ([^:]+): (.*)$
	topLevelRe := regexp.MustCompile(`^- \[([ xX])\] ((?:❌|🔄)*)\s*(.*)$`)
	subTaskRe := regexp.MustCompile(`^  - \[([ xX])\] ([^:]+): (.*)$`)

	lines := strings.Split(content, "\n")
	var currentTask *Task
	taskIndex := 0

	for lineNum, line := range lines {
		// Check for top-level task
		if matches := topLevelRe.FindStringSubmatch(line); matches != nil {
			// Save previous task if exists
			if currentTask != nil {
				state.Tasks = append(state.Tasks, *currentTask)
			}

			checked := strings.ToLower(matches[1]) == "x"
			// Count ❌ and 🔄 emojis from the marker string
			markerStr := matches[2]
			failureCount := strings.Count(markerStr, "❌")
			replanCount := strings.Count(markerStr, "🔄")
			currentTask = &Task{
				Index:        taskIndex,
				Text:         strings.TrimSpace(matches[3]),
				Checked:      checked,
				LineNum:      lineNum + 1,
				FailureCount: failureCount,
				ReplanCount:  replanCount,
				SubTasks:     []SubTask{},
			}
			taskIndex++
			continue
		}

		// Check for sub-task (must have current task)
		if currentTask != nil {
			if matches := subTaskRe.FindStringSubmatch(line); matches != nil {
				checked := strings.ToLower(matches[1]) == "x"
				subTask := SubTask{
					Index:       len(currentTask.SubTasks),
					Skill:       strings.TrimSpace(matches[2]),
					Text:        strings.TrimSpace(matches[3]),
					Checked:     checked,
					LineNum:     lineNum + 1,
					ParentIndex: currentTask.Index,
				}
				currentTask.SubTasks = append(currentTask.SubTasks, subTask)
			}
		}
	}

	// Don't forget the last task
	if currentTask != nil {
		state.Tasks = append(state.Tasks, *currentTask)
	}

	return state, nil
}

// FindCurrentSprintFS finds the current (first incomplete) sprint from an fs.FS
// Returns the relative path and sprint number, or empty string and 0 if none found
func FindCurrentSprintFS(fsys fs.FS) (string, int) {
	entries, err := fs.ReadDir(fsys, filepath.Join(".ai", "sprints"))
	if err != nil {
		return "", 0
	}

	// Collect sprint files
	var sprintFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			sprintFiles = append(sprintFiles, e.Name())
		}
	}

	// Sort to get them in order
	sort.Strings(sprintFiles)

	// Find first incomplete sprint
	for _, name := range sprintFiles {
		path := ".ai/sprints/" + name
		sprint, err := ParseSprintFS(fsys, path)
		if err != nil {
			continue
		}

		// Extract sprint number from filename (e.g., "01-initial.md" -> 1)
		num := ExtractSprintNum(name)

		if !sprint.IsComplete() {
			return path, num
		}
	}

	// All complete or no sprints - return last one if exists
	if len(sprintFiles) > 0 {
		name := sprintFiles[len(sprintFiles)-1]
		return ".ai/sprints/" + name, ExtractSprintNum(name)
	}

	return "", 0
}

// ExtractSprintNum extracts the sprint number from a filename like "01-initial.md"
func ExtractSprintNum(filename string) int {
	// Try to extract leading digits
	re := regexp.MustCompile(`^(\d+)`)
	if matches := re.FindStringSubmatch(filename); matches != nil {
		var num int
		fmt.Sscanf(matches[1], "%d", &num)
		return num
	}
	return 0
}

// FormatSprintFilename formats a sprint filename given a number and name.
// Example: FormatSprintFilename(1, "initial") returns "01-initial.md"
func FormatSprintFilename(num int, name string) string {
	return fmt.Sprintf("%02d-%s.md", num, name)
}

// GetNextSubTask returns the next unchecked sub-task to work on
// Returns nil if all tasks are complete
func (s *SprintState) GetNextSubTask() *SubTask {
	for i := range s.Tasks {
		task := &s.Tasks[i]
		// Skip completed top-level tasks
		if task.Checked {
			continue
		}

		// Find first unchecked sub-task
		for j := range task.SubTasks {
			subTask := &task.SubTasks[j]
			if !subTask.Checked {
				return subTask
			}
		}
	}
	return nil
}

// GetCurrentTask returns the current top-level task being worked on
func (s *SprintState) GetCurrentTask() *Task {
	for i := range s.Tasks {
		if !s.Tasks[i].Checked {
			return &s.Tasks[i]
		}
	}
	return nil
}

// IsComplete returns true if all tasks and sub-tasks are checked
func (s *SprintState) IsComplete() bool {
	for _, task := range s.Tasks {
		if !task.Checked {
			return false
		}
	}
	return len(s.Tasks) > 0
}

// AllSubTasksComplete returns true if all sub-tasks of a task are complete
func (s *SprintState) AllSubTasksComplete(taskIndex int) bool {
	if taskIndex < 0 || taskIndex >= len(s.Tasks) {
		return false
	}
	task := &s.Tasks[taskIndex]
	for _, sub := range task.SubTasks {
		if !sub.Checked {
			return false
		}
	}
	return len(task.SubTasks) > 0
}

// CheckSubTask marks a sub-task as complete in the file
func (s *SprintState) CheckSubTask(taskIndex, subTaskIndex int) error {
	if taskIndex < 0 || taskIndex >= len(s.Tasks) {
		return fmt.Errorf("invalid task index: %d", taskIndex)
	}
	task := &s.Tasks[taskIndex]
	if subTaskIndex < 0 || subTaskIndex >= len(task.SubTasks) {
		return fmt.Errorf("invalid sub-task index: %d", subTaskIndex)
	}

	subTask := &task.SubTasks[subTaskIndex]
	return s.checkLineAt(subTask.LineNum)
}

// CheckTask marks a top-level task as complete in the file
func (s *SprintState) CheckTask(taskIndex int) error {
	if taskIndex < 0 || taskIndex >= len(s.Tasks) {
		return fmt.Errorf("invalid task index: %d", taskIndex)
	}
	task := &s.Tasks[taskIndex]
	return s.checkLineAt(task.LineNum)
}

// checkLineAt changes [ ] to [x] at a specific line number
func (s *SprintState) checkLineAt(lineNum int) error {
	lines := strings.Split(s.Content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return fmt.Errorf("invalid line number: %d", lineNum)
	}

	line := lines[lineNum-1]
	// Replace [ ] with [x]
	newLine := strings.Replace(line, "[ ]", "[x]", 1)
	if newLine == line {
		return nil // Already checked or no checkbox
	}

	lines[lineNum-1] = newLine
	s.Content = strings.Join(lines, "\n")

	// Write back to file
	return os.WriteFile(s.FilePath, []byte(s.Content), 0644)
}

// UncheckSubTask marks a sub-task as incomplete in the file (for review failures)
func (s *SprintState) UncheckSubTask(taskIndex, subTaskIndex int) error {
	if taskIndex < 0 || taskIndex >= len(s.Tasks) {
		return fmt.Errorf("invalid task index: %d", taskIndex)
	}
	task := &s.Tasks[taskIndex]
	if subTaskIndex < 0 || subTaskIndex >= len(task.SubTasks) {
		return fmt.Errorf("invalid sub-task index: %d", subTaskIndex)
	}

	subTask := &task.SubTasks[subTaskIndex]
	return s.uncheckLineAt(subTask.LineNum)
}

// uncheckLineAt changes [x] to [ ] at a specific line number
func (s *SprintState) uncheckLineAt(lineNum int) error {
	lines := strings.Split(s.Content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return fmt.Errorf("invalid line number: %d", lineNum)
	}

	line := lines[lineNum-1]
	// Replace [x] or [X] with [ ]
	newLine := strings.Replace(line, "[x]", "[ ]", 1)
	if newLine == line {
		newLine = strings.Replace(line, "[X]", "[ ]", 1)
	}
	if newLine == line {
		return nil // Already unchecked or no checkbox
	}

	lines[lineNum-1] = newLine
	s.Content = strings.Join(lines, "\n")

	// Write back to file
	return os.WriteFile(s.FilePath, []byte(s.Content), 0644)
}

// GetProgress returns completed/total counts
func (s *SprintState) GetProgress() (completed, total int) {
	for _, task := range s.Tasks {
		total++
		if task.Checked {
			completed++
		}
	}
	return completed, total
}

// GetSubTaskProgress returns completed/total sub-task counts for current task
func (s *SprintState) GetSubTaskProgress() (completed, total int) {
	task := s.GetCurrentTask()
	if task == nil {
		return 0, 0
	}
	for _, sub := range task.SubTasks {
		total++
		if sub.Checked {
			completed++
		}
	}
	return completed, total
}

// TruncateText truncates text to maxLen characters with "..."
func TruncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return "..."
	}
	return text[:maxLen-3] + "..."
}

// AddFailure increments the failure count (❌) on a top-level task
func (s *SprintState) AddFailure(taskIndex int) error {
	if taskIndex < 0 || taskIndex >= len(s.Tasks) {
		return fmt.Errorf("invalid task index: %d", taskIndex)
	}
	task := &s.Tasks[taskIndex]

	lines := strings.Split(s.Content, "\n")
	if task.LineNum < 1 || task.LineNum > len(lines) {
		return fmt.Errorf("invalid line number: %d", task.LineNum)
	}

	line := lines[task.LineNum-1]

	// Pattern: - [x] ❌🔄❌ Task text OR - [ ] Task text
	// We need to insert one ❌ after existing markers, before the task text
	re := regexp.MustCompile(`^(- \[[ xX]\]) ((?:❌|🔄)*)(.*)$`)
	if matches := re.FindStringSubmatch(line); matches != nil {
		// matches[1] = "- [x]" or "- [ ]"
		// matches[2] = existing markers (may be empty)
		// matches[3] = rest of line (task text)
		newLine := matches[1] + " " + matches[2] + "❌" + matches[3]
		lines[task.LineNum-1] = newLine
	} else {
		return fmt.Errorf("could not parse task line: %s", line)
	}

	s.Content = strings.Join(lines, "\n")
	task.FailureCount++

	// Write back to file
	return os.WriteFile(s.FilePath, []byte(s.Content), 0644)
}

// AddReplanMarker adds a 🔄 marker to a top-level task
func (s *SprintState) AddReplanMarker(taskIndex int) error {
	if taskIndex < 0 || taskIndex >= len(s.Tasks) {
		return fmt.Errorf("invalid task index: %d", taskIndex)
	}
	task := &s.Tasks[taskIndex]

	lines := strings.Split(s.Content, "\n")
	if task.LineNum < 1 || task.LineNum > len(lines) {
		return fmt.Errorf("invalid line number: %d", task.LineNum)
	}

	line := lines[task.LineNum-1]

	re := regexp.MustCompile(`^(- \[[ xX]\]) ((?:❌|🔄)*)(.*)$`)
	if matches := re.FindStringSubmatch(line); matches != nil {
		newLine := matches[1] + " " + matches[2] + "🔄" + matches[3]
		lines[task.LineNum-1] = newLine
	} else {
		return fmt.Errorf("could not parse task line: %s", line)
	}

	s.Content = strings.Join(lines, "\n")
	task.ReplanCount++

	return os.WriteFile(s.FilePath, []byte(s.Content), 0644)
}

// ClearFailures removes all ❌ markers from a top-level task (keeps 🔄)
func (s *SprintState) ClearFailures(taskIndex int) error {
	if taskIndex < 0 || taskIndex >= len(s.Tasks) {
		return fmt.Errorf("invalid task index: %d", taskIndex)
	}
	task := &s.Tasks[taskIndex]

	lines := strings.Split(s.Content, "\n")
	if task.LineNum < 1 || task.LineNum > len(lines) {
		return fmt.Errorf("invalid line number: %d", task.LineNum)
	}

	line := lines[task.LineNum-1]

	re := regexp.MustCompile(`^(- \[[ xX]\]) ((?:❌|🔄)*)\s*(.*)$`)
	if matches := re.FindStringSubmatch(line); matches != nil {
		// Remove ❌ but keep 🔄
		markers := strings.ReplaceAll(matches[2], "❌", "")
		if markers == "" {
			newLine := matches[1] + " " + matches[3]
			lines[task.LineNum-1] = newLine
		} else {
			newLine := matches[1] + " " + markers + matches[3]
			lines[task.LineNum-1] = newLine
		}
	} else {
		return fmt.Errorf("could not parse task line: %s", line)
	}

	s.Content = strings.Join(lines, "\n")
	task.FailureCount = 0

	return os.WriteFile(s.FilePath, []byte(s.Content), 0644)
}

// NormalizeTaskText strips checkbox, ❌/🔄 emojis, and normalizes whitespace for task matching
func NormalizeTaskText(text string) string {
	// Remove any leading/trailing whitespace
	text = strings.TrimSpace(text)
	// Remove ❌ and 🔄 emojis
	text = strings.ReplaceAll(text, "❌", "")
	text = strings.ReplaceAll(text, "🔄", "")
	// Normalize multiple spaces to single space
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// MergeFailureCounts preserves failure counts from an old sprint to a new sprint
// It matches tasks by normalized text and copies failure counts to the new sprint
func MergeFailureCounts(oldSprintPath, newSprintPath string) error {
	// Parse old sprint
	oldSprint, err := ParseSprint(oldSprintPath)
	if err != nil {
		return fmt.Errorf("failed to parse old sprint: %w", err)
	}

	// Build map of normalized task text -> failure count
	failureCounts := make(map[string]int)
	for _, task := range oldSprint.Tasks {
		if task.FailureCount > 0 {
			normalized := NormalizeTaskText(task.Text)
			failureCounts[normalized] = task.FailureCount
		}
	}

	if len(failureCounts) == 0 {
		return nil // No failures to merge
	}

	// Parse new sprint
	newSprint, err := ParseSprint(newSprintPath)
	if err != nil {
		return fmt.Errorf("failed to parse new sprint: %w", err)
	}

	// Apply failure counts to matching tasks
	for i := range newSprint.Tasks {
		task := &newSprint.Tasks[i]
		normalized := NormalizeTaskText(task.Text)
		if count, ok := failureCounts[normalized]; ok && count > 0 {
			// Add the failure emojis
			for j := 0; j < count; j++ {
				if err := newSprint.AddFailure(i); err != nil {
					fmt.Printf("Warning: failed to add failure marker to task %d: %v\n", i, err)
					break
				}
			}
		}
	}

	return nil
}

// GetOverallProgress returns completed/total counts across all subtask-level items
// Tasks with no subtasks count as a single item each
func (s *SprintState) GetOverallProgress() (completed, total int) {
	for _, task := range s.Tasks {
		if len(task.SubTasks) == 0 {
			total++
			if task.Checked {
				completed++
			}
		} else {
			for _, sub := range task.SubTasks {
				total++
				if sub.Checked {
					completed++
				}
			}
		}
	}
	return completed, total
}

// RenderProgressBar renders visual progress as [xo. ... .....] format
// runningTaskIdx and runningSubIdx indicate which subtask is currently running (-1 for none)
// Returns a string like "[xo. ... .....] 45% Sprint 1 - Task name"
func (s *SprintState) RenderProgressBar(sprintNum int, runningTaskIdx, runningSubIdx int) string {
	var segments []string
	var singleChars string

	flushSingles := func() {
		if singleChars != "" {
			segments = append(segments, singleChars)
			singleChars = ""
		}
	}

	for _, task := range s.Tasks {
		if len(task.SubTasks) == 0 {
			// Accumulate no-subtask tasks into a single grouped segment
			if task.Checked {
				singleChars += "x"
			} else {
				singleChars += "."
			}
			continue
		}

		flushSingles()

		var chars []byte
		for _, sub := range task.SubTasks {
			if sub.Checked {
				chars = append(chars, 'x')
			} else if task.Index == runningTaskIdx && sub.Index == runningSubIdx {
				chars = append(chars, 'o')
			} else {
				chars = append(chars, '.')
			}
		}
		segments = append(segments, string(chars))
	}
	flushSingles()

	bar := "[" + strings.Join(segments, " ") + "]"

	// Compute percentage from all subtask-level items
	completed, total := s.GetOverallProgress()
	pct := 0
	if total > 0 {
		pct = completed * 100 / total
	}

	// Add context: percentage, sprint number, and current task name
	currentTask := s.GetCurrentTask()
	if currentTask != nil {
		return fmt.Sprintf("%s %d%% Sprint %d - %s", bar, pct, sprintNum, TruncateText(currentTask.Text, 40))
	}

	return fmt.Sprintf("%s %d%% Sprint %d - Complete", bar, pct, sprintNum)
}
