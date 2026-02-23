package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// Logger manages log files for agent invocations
type Logger struct {
	baseDir      string
	sprintNumber int
	sequence     int64
}

// NewLogger creates a new logger for the given project directory and sprint
func NewLogger(projectDir string, sprintNumber int) *Logger {
	return &Logger{
		baseDir:      filepath.Join(projectDir, ".ai", "logs", fmt.Sprintf("sprint-%03d", sprintNumber)),
		sprintNumber: sprintNumber,
		sequence:     0,
	}
}

// Invocation represents a single agent invocation to be logged
type Invocation struct {
	Timestamp    time.Time
	Sprint       int
	Phase        string
	Task         string
	TaskIndex    int
	Agent        string
	Skill        string
	Prompt       string
	Response     string
	Duration     time.Duration
	Status       string
	Error        error
	FilesWritten []string
	Notes        string
}

// LogFile represents an open log file
type LogFile struct {
	Path       string
	file       *os.File
	invocation *Invocation
	startTime  time.Time
}

// StartInvocation begins logging a new agent invocation
func (l *Logger) StartInvocation(phase, task string, taskIndex int, agent, skill, promptSummary string) (*LogFile, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(l.baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Generate sequence number
	seq := atomic.AddInt64(&l.sequence, 1)

	// Generate filename: {seq}-{phase}-{task}-{skill}-{agent}.md
	filename := fmt.Sprintf("%03d-%s-%02d-%s-%s.md", seq, phase, taskIndex, skill, agent)
	path := filepath.Join(l.baseDir, filename)

	// Create log file
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	inv := &Invocation{
		Timestamp: time.Now(),
		Sprint:    l.sprintNumber,
		Phase:     phase,
		Task:      task,
		TaskIndex: taskIndex,
		Agent:     agent,
		Skill:     skill,
	}

	lf := &LogFile{
		Path:       path,
		file:       file,
		invocation: inv,
		startTime:  time.Now(),
	}

	// Print console summary
	PrintInvocationStart(agent, skill, promptSummary, path)

	return lf, nil
}

// SetPrompt records the prompt sent to the agent
func (lf *LogFile) SetPrompt(prompt string) {
	lf.invocation.Prompt = prompt
}

// SetResponse records the agent's response
func (lf *LogFile) SetResponse(response string) {
	lf.invocation.Response = response
}

// SetStatus sets the invocation status
func (lf *LogFile) SetStatus(status string) {
	lf.invocation.Status = status
}

// SetError records any error that occurred
func (lf *LogFile) SetError(err error) {
	lf.invocation.Error = err
	if err != nil {
		lf.invocation.Status = "error"
	}
}

// AddFileWritten records a file that was written
func (lf *LogFile) AddFileWritten(path string) {
	lf.invocation.FilesWritten = append(lf.invocation.FilesWritten, path)
}

// SetNotes adds notes to the log
func (lf *LogFile) SetNotes(notes string) {
	lf.invocation.Notes = notes
}

// Close finalizes and writes the log file
func (lf *LogFile) Close() error {
	lf.invocation.Duration = time.Since(lf.startTime)

	// Write the formatted log
	content := FormatInvocation(lf.invocation)
	if _, err := lf.file.WriteString(content); err != nil {
		lf.file.Close()
		return fmt.Errorf("failed to write log: %w", err)
	}

	return lf.file.Close()
}

// GetRelativePath returns the path relative to project root
func (lf *LogFile) GetRelativePath(projectDir string) string {
	rel, err := filepath.Rel(projectDir, lf.Path)
	if err != nil {
		return lf.Path
	}
	return rel
}

// PrintInvocationStart prints the console summary line with timestamp
func PrintInvocationStart(agent, skill, summary, logPath string) {
	// Truncate summary if too long
	if len(summary) > 50 {
		summary = summary[:47] + "..."
	}
	ts := strings.TrimSuffix(strings.ToLower(time.Now().Format("3:04PM")), "m")
	fmt.Printf("%s %s %s: %q %s\n",
		Dim("["+ts+"]"), Cyan("["+agent+"]"), skill, summary, Dim("→ "+logPath))
}

// GetLogsDir returns the logs directory for a sprint
func GetLogsDir(projectDir string, sprintNumber int) string {
	return filepath.Join(projectDir, ".ai", "logs", fmt.Sprintf("sprint-%03d", sprintNumber))
}

// ListLogs returns all log files for a sprint
func ListLogs(projectDir string, sprintNumber int) ([]string, error) {
	dir := GetLogsDir(projectDir, sprintNumber)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var logs []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
			logs = append(logs, filepath.Join(dir, e.Name()))
		}
	}
	return logs, nil
}

// EnsureRetrosDir ensures the retros directory exists
func EnsureRetrosDir(projectDir string) error {
	dir := filepath.Join(projectDir, ".ai", "retros")
	return os.MkdirAll(dir, 0755)
}

// GetRetroPath returns the path to a sprint's retrospective file
func GetRetroPath(projectDir string, sprintNumber int) string {
	return filepath.Join(projectDir, ".ai", "retros", fmt.Sprintf("sprint-%03d.md", sprintNumber))
}
