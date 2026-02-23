package agent

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/strongdm/agate/internal/logging"
)

// CountingWriter wraps a writer and tracks bytes written, with a live time+size ticker
type CountingWriter struct {
	writer      io.Writer
	bytesRead   int64
	lastDisplay string
	startTime   time.Time
	mu          sync.Mutex
	showKB      bool
	done        chan struct{}
	stopped     bool
}

// NewCountingWriter creates a new counting writer and starts the ticker
func NewCountingWriter(w io.Writer, showKB bool) *CountingWriter {
	c := &CountingWriter{
		writer:    w,
		showKB:    showKB,
		startTime: time.Now(),
		done:      make(chan struct{}),
	}
	if showKB {
		go c.tickLoop()
	}
	return c
}

// formatSize formats bytes as 3-significant-digit string: 1.23k, 12.3k, 123k, 1.23M
func formatSize(bytes int64) string {
	kb := float64(bytes) / 1024.0
	if kb < 1.0 {
		return ""
	}
	if kb < 10 {
		return fmt.Sprintf("%.2fk", kb)
	}
	if kb < 100 {
		return fmt.Sprintf("%.1fk", kb)
	}
	if kb < 1000 {
		return fmt.Sprintf("%.0fk", kb)
	}
	mb := kb / 1024.0
	if mb < 10 {
		return fmt.Sprintf("%.2fM", mb)
	}
	if mb < 100 {
		return fmt.Sprintf("%.1fM", mb)
	}
	return fmt.Sprintf("%.0fM", mb)
}

// formatDuration formats a duration compactly: 3s, 1m 3s, 1h 2m
func formatDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

// tickLoop updates the display every second
func (c *CountingWriter) tickLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.Lock()
			if !c.stopped {
				c.refresh()
			}
			c.mu.Unlock()
		}
	}
}

// refresh updates the terminal display if it changed
func (c *CountingWriter) refresh() {
	display := c.currentDisplay()
	if display != c.lastDisplay {
		c.lastDisplay = display
		fmt.Printf("\r%-20s", logging.Dim(display))
	}
}

// currentDisplay builds the "3s, 4.56k" string
func (c *CountingWriter) currentDisplay() string {
	elapsed := time.Since(c.startTime)
	timeStr := formatDuration(elapsed)
	sizeStr := formatSize(c.bytesRead)
	if sizeStr != "" {
		return fmt.Sprintf("%s, %s", timeStr, sizeStr)
	}
	return timeStr
}

// Write implements io.Writer and tracks bytes
func (c *CountingWriter) Write(p []byte) (n int, err error) {
	n, err = c.writer.Write(p)
	c.mu.Lock()
	c.bytesRead += int64(n)
	if c.showKB && !c.stopped {
		c.refresh()
	}
	c.mu.Unlock()
	return n, err
}

// BytesWritten returns the total bytes written
func (c *CountingWriter) BytesWritten() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bytesRead
}

// PrintFinal stops the ticker and prints the final line
func (c *CountingWriter) PrintFinal() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.showKB && !c.stopped {
		c.stopped = true
		close(c.done)
		display := c.currentDisplay()
		fmt.Printf("\r%-20s\n", logging.Dim(display))
	}
}
