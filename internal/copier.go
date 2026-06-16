package internal

import (
	"fmt"
	"os"
	"runtime"
	"sync"
)

// CopyEngine orchestrates the parallel copy of files from source to destination.
type CopyEngine struct {
	opts       Options
	numWorkers int
	quiet      bool
	dryRun     bool
	progress   *Progress
	errors     []error
	errorsMu   sync.Mutex
	checksums  map[string]string
	checksumMu sync.Mutex
}

// NewCopyEngine creates a new CopyEngine with the given configuration.
func NewCopyEngine(numWorkers int, opts Options, quiet, dryRun bool) *CopyEngine {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU() * 2
	}
	return &CopyEngine{
		opts:       opts,
		numWorkers: numWorkers,
		quiet:      quiet,
		dryRun:     dryRun,
		checksums:  make(map[string]string),
	}
}

// Run executes the copy operation from srcDir to dstDir.
func (e *CopyEngine) Run(srcDir, dstDir string) error {
	// Phase 1: Scan source directory
	if !e.quiet {
		fmt.Fprintf(os.Stderr, "Scanning %s ...\n", srcDir)
	}

	scanResult, err := ScanDir(srcDir, dstDir, e.opts)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	if len(scanResult.ScanErrors) > 0 {
		e.errorsMu.Lock()
		e.errors = append(e.errors, scanResult.ScanErrors...)
		e.errorsMu.Unlock()
	}

	if !e.quiet {
		fmt.Fprintf(os.Stderr, "Found %d files (%s)\n",
			scanResult.TotalFiles,
			formatBytes(scanResult.TotalBytes))
	}

	if e.dryRun {
		e.printDryRun(scanResult)
		return nil
	}

	// Phase 2: Setup progress tracker
	e.progress = NewProgress(scanResult.TotalFiles, scanResult.TotalBytes, e.quiet)
	e.progress.Start()

	// Phase 3: Dispatch files to workers
	// Separate small and large file queues
	smallFiles := make(chan FileEntry, e.numWorkers*2)
	largeFiles := make(chan FileEntry, 4)

	var wg sync.WaitGroup

	// Start small-file workers
	smallWorkers := e.numWorkers
	for i := 0; i < smallWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.worker(smallFiles)
		}()
	}

	// Start large-file workers (limited to prevent I/O saturation)
	largeWorkers := min(4, e.numWorkers/2)
	if largeWorkers < 1 {
		largeWorkers = 1
	}
	for i := 0; i < largeWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.worker(largeFiles)
		}()
	}

	// Dispatch files to appropriate queues
	for _, f := range scanResult.Files {
		if f.Info.Size() >= LargeFileThreshold && !f.IsSymlink {
			largeFiles <- f
		} else {
			smallFiles <- f
		}
	}
	close(smallFiles)
	close(largeFiles)

	// Wait for all workers to finish
	wg.Wait()

	// Phase 4: Preserve directory metadata (must be done after all files are copied)
	if e.opts.Archive {
		// Process in reverse order so child dirs are set before parents
		dirs := scanResult.Dirs()
		for i := len(dirs) - 1; i >= 0; i-- {
			d := dirs[i]
			if err := PreserveDirMetadata(d.SrcPath, d.DstPath, d.Info); err != nil {
				e.addError(fmt.Errorf("dir metadata %s: %w", d.RelPath, err))
			}
		}
	}

	// Phase 5: Print summary
	e.progress.Stop()
	e.progress.PrintSummary(os.Stderr)

	// Print checksums if enabled
	if e.opts.Checksum {
		e.printChecksums()
	}

	// Report errors
	if len(e.errors) > 0 {
		if e.opts.ErrorLog != "" {
			e.writeErrorLog()
		}
		fmt.Fprintf(os.Stderr, "\n⚠ %d errors occurred:\n", len(e.errors))
		for _, err := range e.errors {
			fmt.Fprintf(os.Stderr, "  • %s\n", err)
		}
		return fmt.Errorf("%d files failed to copy", len(e.errors))
	}

	return nil
}

// worker processes files from the given channel.
func (e *CopyEngine) worker(files <-chan FileEntry) {
	for f := range files {
		// Check if incremental skip applies
		if !e.opts.Force && !NeedsCopy(f.SrcPath, f.DstPath, f.Info, e.opts.Force) {
			e.progress.AddSkippedFile()
			continue
		}

		checksum, err := CopyFile(f.SrcPath, f.DstPath, f.Info, e.opts)
		if err != nil {
			e.addError(fmt.Errorf("%s: %w", f.RelPath, err))
			e.progress.AddErrorFile()
			continue
		}

		if e.opts.Checksum && checksum != "" {
			e.checksumMu.Lock()
			e.checksums[f.RelPath] = checksum
			e.checksumMu.Unlock()
		}

		e.progress.AddCopiedFile(f.Info.Size())
	}
}

func (e *CopyEngine) addError(err error) {
	e.errorsMu.Lock()
	defer e.errorsMu.Unlock()
	e.errors = append(e.errors, err)
}

// Progress returns the progress tracker (available after Run starts).
func (e *CopyEngine) Progress() *Progress {
	return e.progress
}

// Errors returns the list of errors encountered during copy.
func (e *CopyEngine) Errors() []error {
	e.errorsMu.Lock()
	defer e.errorsMu.Unlock()
	result := make([]error, len(e.errors))
	copy(result, e.errors)
	return result
}

func (e *CopyEngine) printDryRun(result *ScanResult) {
	fmt.Println("DRY RUN — files that would be copied:")
	for _, f := range result.Files {
		action := "COPY"
		if !NeedsCopy(f.SrcPath, f.DstPath, f.Info, e.opts.Force) {
			action = "SKIP"
		}
		fmt.Printf("  [%s] %s (%s)\n", action, f.RelPath, formatBytes(f.Info.Size()))
	}
}

func (e *CopyEngine) printChecksums() {
	if len(e.checksums) == 0 {
		return
	}
	fmt.Println("\nSHA256 Checksums:")
	for path, sum := range e.checksums {
		fmt.Printf("  %s  %s\n", sum, path)
	}
}

func (e *CopyEngine) writeErrorLog() {
	f, err := os.OpenFile(e.opts.ErrorLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open error log: %v\n", err)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "--- Fastcopy Errors ---\n")
	for _, eErr := range e.errors {
		fmt.Fprintf(f, "%v\n", eErr)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
