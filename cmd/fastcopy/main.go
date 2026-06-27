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
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/moises/fastcopy/internal"
	idx "github.com/moises/fastcopy/internal/index"
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
	removeSource := flag.Bool("remove-source", false, "delete source files and empty directories after successful copy (move)")
	showVersion := flag.Bool("version", false, "show version and exit")
	// Indexing flags (new design)
	createIdx := flag.Bool("index-build", false, "build file index for the given directory")
	search := flag.String("index-search", "", "search term/pattern in index")
	idxPath := flag.String("index-path", "fastcopy.idx", "path to the index file")
	indexHash := flag.Bool("index-hash", false, "compute SHA-256 hashes during index build")
	indexDupes := flag.Bool("index-dupes", false, "list duplicate files from index")

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

	// Index creation, search, or duplicate listing takes precedence over copy operation.
	if *createIdx || *search != "" || *indexDupes {
		if *createIdx {
			// Expect a source directory argument after flags.
			if len(flag.Args()) < 1 {
				fmt.Fprintln(os.Stderr, "Source directory required for index creation")
				os.Exit(1)
			}
			src := flag.Args()[0]
			idxObj, err := idx.BuildFromScan(context.Background(), src, *indexHash)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error building index: %v\n", err)
				os.Exit(1)
			}
			if err := idxObj.Save(*idxPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving index: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Index built successfully: %d entries\n", len(idxObj.Entries))
			os.Exit(0)
		}

		// Load existing index for search or duplicate listing.
		idxObj, err := idx.Load(*idxPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading index: %v\n", err)
			os.Exit(1)
		}

		if *search != "" {
			q := idx.Query{Name: *search}
			results := idxObj.Search(q)
			for _, r := range results {
				fmt.Printf("  %s (%d bytes)\n", r.Path, r.Size)
			}
			os.Exit(0)
		}

		if *indexDupes {
			groups := idxObj.FindDuplicates()
			if len(groups) == 0 {
				fmt.Println("No duplicates found.")
			}
			for _, grp := range groups {
				fmt.Println("Duplicate group:")
				for _, e := range grp {
					fmt.Printf("  %s (%d bytes, hash=%s)\n", e.Path, e.Size, e.Hash)
				}
			}
			os.Exit(0)
		}
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
		Archive:      !*noArchive,
		Checksum:     *checksum,
		Force:        *force,
		SkipErrors:   *skipErrors,
		ErrorLog:     *errorLog,
		DryRun:       *dryRun,
		RemoveSource: *removeSource,
	}

	engine := internal.NewCopyEngine(*numWorkers, opts, *quiet, *dryRun)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if !*quiet {
			fmt.Fprintf(os.Stderr, "\nReceived interrupt, stopping copy...\n")
		}
		cancel()
	}()

	if err := engine.Run(ctx, srcDir, dstDir); err != nil {
		if err.Error() == "context canceled" {
			os.Exit(130) // standard exit code for SIGINT
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
