// fastcopy is an ultra-fast file copier for Linux that outperforms cp and rsync
// by combining parallel file processing, zero-copy kernel syscalls, and
// intelligent I/O optimizations.
//
// Usage:
//
//	fastcopy [options] <source> <destination>
//
// Options:
//
//	-w N          Number of workers (default: NumCPU * 2)
//	--checksum    Compute SHA256 checksum during copy
//	--dry-run     Show what would be copied without copying
//	--force       Disable incremental mode (always recopy)
//	--no-archive  Don't preserve permissions/ownership/timestamps
//	--quiet       Suppress progress output
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/moises/fastcopy/internal"
)

var (
	version = "0.1.0"
)

func main() {
	// Define flags
	numWorkers := flag.Int("w", runtime.NumCPU()*2, "number of parallel workers")
	checksum := flag.Bool("checksum", false, "compute SHA256 checksum during copy")
	dryRun := flag.Bool("dry-run", false, "show what would be copied without copying")
	force := flag.Bool("force", false, "disable incremental mode, always recopy all files")
	noArchive := flag.Bool("no-archive", false, "don't preserve permissions/ownership/timestamps")
	quiet := flag.Bool("quiet", false, "suppress progress output")
	skipErrors := flag.Bool("skip-errors", false, "skip files/folders with permission or read errors")
	errorLog := flag.String("error-log", "", "path to save detailed error log")
	showVersion := flag.Bool("version", false, "show version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "fastcopy v%s — Ultra-fast parallel file copier\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <source> <destination>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s /data/source /backup/dest          # Copy with defaults\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -w 32 /data/source /backup/dest   # Use 32 workers\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --checksum /src /dst               # Copy with SHA256 verification\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --dry-run /src /dst                # Preview what would be copied\n", os.Args[0])
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("fastcopy v%s (Go %s, %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	if len(flag.Args()) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	srcDir := flag.Args()[0]
	dstDir := flag.Args()[1]

	// Validate source exists
	srcInfo, err := os.Stat(srcDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: source %s does not exist: %v\n", srcDir, err)
		os.Exit(1)
	}
	if !srcInfo.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: source %s is not a directory\n", srcDir)
		os.Exit(1)
	}

	// Create destination if it doesn't exist
	if !*dryRun {
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot create destination %s: %v\n", dstDir, err)
			os.Exit(1)
		}
	}

	if !*quiet {
		fmt.Fprintf(os.Stderr, "fastcopy v%s — %d workers on %d CPUs\n",
			version, *numWorkers, runtime.NumCPU())
	}

	// Configure and run
	opts := internal.Options{
		Archive:    !*noArchive,
		Checksum:   *checksum,
		Force:      *force,
		SkipErrors: *skipErrors,
		ErrorLog:   *errorLog,
		DryRun:     *dryRun,
	}

	engine := internal.NewCopyEngine(*numWorkers, opts, *quiet, *dryRun)

	if err := engine.Run(srcDir, dstDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
