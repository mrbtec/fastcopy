// Package internal provides the core copy engine for fastcopy.
package internal

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Progress tracks copy operation statistics and displays real-time progress.
type Progress struct {
	totalFiles   atomic.Int64
	copiedFiles  atomic.Int64
	skippedFiles atomic.Int64
	errorFiles   atomic.Int64
	totalBytes   atomic.Int64
	copiedBytes  atomic.Int64

	startTime time.Time
	quiet     bool
	done      chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
}

// NewProgress creates a new Progress tracker.
// If quiet is true, no output is printed.
func NewProgress(totalFiles int64, totalBytes int64, quiet bool) *Progress {
	p := &Progress{
		quiet:     quiet,
		startTime: time.Now(),
		done:      make(chan struct{}),
	}
	p.totalFiles.Store(totalFiles)
	p.totalBytes.Store(totalBytes)
	return p
}

// Start begins the progress display loop, printing every 500ms.
func (p *Progress) Start() {
	if p.quiet {
		return
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.print()
			case <-p.done:
				p.print()
				return
			}
		}
	}()
}

// Stop halts the progress display and prints a final summary.
func (p *Progress) Stop() {
	if p.quiet {
		return
	}
	close(p.done)
	p.wg.Wait()
	fmt.Fprintln(os.Stderr) // newline after progress
}

// AddCopiedFile increments the copied file counter and adds bytes.
func (p *Progress) AddCopiedFile(bytes int64) {
	p.copiedFiles.Add(1)
	p.copiedBytes.Add(bytes)
}

// AddDiscoveredFile increments total expected files and bytes.
func (p *Progress) AddDiscoveredFile(bytes int64) {
	p.totalFiles.Add(1)
	p.totalBytes.Add(bytes)
}

// AddSkippedFile increments the skipped file counter.
func (p *Progress) AddSkippedFile() {
	p.skippedFiles.Add(1)
}

// AddErrorFile increments the error file counter.
func (p *Progress) AddErrorFile() {
	p.errorFiles.Add(1)
}

// AddCopiedBytes adds to the byte counter without incrementing file count.
func (p *Progress) AddCopiedBytes(bytes int64) {
	p.copiedBytes.Add(bytes)
}

// Snapshot returns a point-in-time snapshot of all progress counters.
type ProgressSnapshot struct {
	TotalFiles   int64
	CopiedFiles  int64
	SkippedFiles int64
	ErrorFiles   int64
	TotalBytes   int64
	CopiedBytes  int64
	Elapsed      time.Duration
	Speed        float64 // bytes/sec
}

// Snapshot returns the current progress state (thread-safe).
func (p *Progress) Snapshot() ProgressSnapshot {
	copied := p.copiedFiles.Load()
	skipped := p.skippedFiles.Load()
	errors := p.errorFiles.Load()
	bytesC := p.copiedBytes.Load()
	elapsed := time.Since(p.startTime)

	var speed float64
	if elapsed.Seconds() > 0 {
		speed = float64(bytesC) / elapsed.Seconds()
	}

	return ProgressSnapshot{
		TotalFiles:   p.totalFiles.Load(),
		CopiedFiles:  copied,
		SkippedFiles: skipped,
		ErrorFiles:   errors,
		TotalBytes:   p.totalBytes.Load(),
		CopiedBytes:  bytesC,
		Elapsed:      elapsed,
		Speed:        speed,
	}
}

// FormatBytes formats a byte count as a human-readable string.
func FormatBytes(b int64) string {
	return formatBytes(b)
}

// FormatDuration formats a duration as a human-readable string.
func FormatDuration(d time.Duration) string {
	return formatDuration(d)
}

func (p *Progress) print() {
	copied := p.copiedFiles.Load()
	skipped := p.skippedFiles.Load()
	errors := p.errorFiles.Load()
	processed := copied + skipped + errors
	bytesC := p.copiedBytes.Load()
	tBytes := p.totalBytes.Load()
	tFiles := p.totalFiles.Load()
	elapsed := time.Since(p.startTime)

	var speed float64
	if elapsed.Seconds() > 0 {
		speed = float64(bytesC) / elapsed.Seconds()
	}

	var eta string
	if speed > 0 && tBytes > 0 {
		remaining := float64(tBytes-bytesC) / speed
		if remaining < 0 {
			remaining = 0
		}
		eta = formatDuration(time.Duration(remaining * float64(time.Second)))
	} else {
		eta = "calculating..."
	}

	fmt.Fprintf(os.Stderr, "\r\033[K[%d/%d files] %s / %s — %s/s — ETA %s",
		processed, tFiles,
		formatBytes(bytesC), formatBytes(tBytes),
		formatBytes(int64(speed)),
		eta,
	)
}

// PrintSummary outputs a final summary to the given writer.
func (p *Progress) PrintSummary(w io.Writer) {
	elapsed := time.Since(p.startTime)
	copied := p.copiedFiles.Load()
	skipped := p.skippedFiles.Load()
	errors := p.errorFiles.Load()
	bytesC := p.copiedBytes.Load()

	var speed float64
	if elapsed.Seconds() > 0 {
		speed = float64(bytesC) / elapsed.Seconds()
	}

	fmt.Fprintf(w, "\n── fastcopy summary ──────────────────────────\n")
	fmt.Fprintf(w, "  Copied:  %d files (%s)\n", copied, formatBytes(bytesC))
	fmt.Fprintf(w, "  Skipped: %d files (unchanged)\n", skipped)
	if errors > 0 {
		fmt.Fprintf(w, "  Errors:  %d files\n", errors)
	}
	fmt.Fprintf(w, "  Speed:   %s/s\n", formatBytes(int64(speed)))
	fmt.Fprintf(w, "  Time:    %s\n", formatDuration(elapsed))
	fmt.Fprintf(w, "──────────────────────────────────────────────\n")
}

func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.1f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "< 1s"
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
