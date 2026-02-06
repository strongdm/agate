package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// SplitView provides a terminal UI with scrolling output and fixed status bar
type SplitView struct {
	mu          sync.Mutex
	output      io.Writer
	statusLines int
	width       int
	height      int
	isTTY       bool
	statusBar   []string
}

// NewSplitView creates a new split view terminal UI
// statusLines is the number of lines reserved for the status bar at the bottom
func NewSplitView(output io.Writer, statusLines int) *SplitView {
	sv := &SplitView{
		output:      output,
		statusLines: statusLines,
		statusBar:   make([]string, statusLines),
	}

	// Check if output is a TTY
	if f, ok := output.(*os.File); ok {
		sv.isTTY = term.IsTerminal(int(f.Fd()))
		if sv.isTTY {
			sv.width, sv.height, _ = term.GetSize(int(f.Fd()))
		}
	}

	// Default dimensions if not a TTY
	if sv.width == 0 {
		sv.width = 80
	}
	if sv.height == 0 {
		sv.height = 24
	}

	return sv
}

// IsTTY returns whether the output is a terminal
func (sv *SplitView) IsTTY() bool {
	return sv.isTTY
}

// SetStatus updates a status line (0-indexed from top of status bar)
func (sv *SplitView) SetStatus(line int, text string) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	if line < 0 || line >= sv.statusLines {
		return
	}

	// Truncate to terminal width
	if len(text) > sv.width {
		text = text[:sv.width-3] + "..."
	}

	sv.statusBar[line] = text

	if sv.isTTY {
		sv.redrawStatusBar()
	}
}

// Write implements io.Writer - writes to the scrolling area
func (sv *SplitView) Write(p []byte) (n int, err error) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	if !sv.isTTY {
		// Not a TTY, just write directly
		return sv.output.Write(p)
	}

	// Save cursor, write content, restore cursor and redraw status
	text := string(p)
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			// Move cursor up to scroll region and write newline
			fmt.Fprint(sv.output, "\n")
		}
		fmt.Fprint(sv.output, line)
	}

	sv.redrawStatusBar()

	return len(p), nil
}

// redrawStatusBar redraws the status bar at the bottom
// Must be called with mutex held
func (sv *SplitView) redrawStatusBar() {
	// Save cursor position
	fmt.Fprint(sv.output, "\033[s")

	// Move to status bar area (bottom of terminal)
	statusTop := sv.height - sv.statusLines
	for i, line := range sv.statusBar {
		// Move to status line
		fmt.Fprintf(sv.output, "\033[%d;1H", statusTop+i+1)
		// Clear line
		fmt.Fprint(sv.output, "\033[2K")
		// Write status (with inverted colors for visibility)
		fmt.Fprintf(sv.output, "\033[7m%s\033[0m", padRight(line, sv.width))
	}

	// Restore cursor position
	fmt.Fprint(sv.output, "\033[u")
}

// Setup initializes the terminal for split view mode
func (sv *SplitView) Setup() {
	if !sv.isTTY {
		return
	}

	// Set scroll region to exclude status bar
	scrollBottom := sv.height - sv.statusLines
	fmt.Fprintf(sv.output, "\033[1;%dr", scrollBottom)

	// Move cursor to top of scroll region
	fmt.Fprint(sv.output, "\033[1;1H")

	// Draw initial status bar
	sv.redrawStatusBar()
}

// Teardown restores the terminal to normal mode
func (sv *SplitView) Teardown() {
	if !sv.isTTY {
		return
	}

	// Reset scroll region to full screen
	fmt.Fprintf(sv.output, "\033[1;%dr", sv.height)

	// Move cursor to bottom
	fmt.Fprintf(sv.output, "\033[%d;1H", sv.height)

	// Clear status bar area
	for i := 0; i < sv.statusLines; i++ {
		fmt.Fprint(sv.output, "\033[2K\n")
	}
}

// padRight pads a string to the specified width
func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// StreamWriter creates a simple writer that flushes after each write
// Use this when you don't need the full split view
type StreamWriter struct {
	output io.Writer
}

// NewStreamWriter creates a new stream writer
func NewStreamWriter(output io.Writer) *StreamWriter {
	return &StreamWriter{output: output}
}

// Write implements io.Writer
func (sw *StreamWriter) Write(p []byte) (n int, err error) {
	n, err = sw.output.Write(p)
	// Try to flush if possible
	if f, ok := sw.output.(interface{ Sync() error }); ok {
		f.Sync()
	}
	return n, err
}
