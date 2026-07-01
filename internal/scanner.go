package internal

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FileEntry represents a file or directory found during scanning.
type FileEntry struct {
	// SrcPath is the absolute path to the source file.
	SrcPath string
	// DstPath is the absolute path to the destination file.
	DstPath string
	// RelPath is the path relative to the source root.
	RelPath string
	// Info is the os.FileInfo of the source file.
	Info os.FileInfo
	// IsSymlink is true if the file is a symbolic link.
	IsSymlink bool
}

// ScanDirAsync recursively scans srcDir and sends FileEntry items
// to the 'out' channel. It also creates necessary destination directories.
// It returns a list of directories for metadata preservation, a list of errors, and any fatal error.
func ScanDirAsync(ctx context.Context, srcDir, dstDir string, opts Options, out chan<- FileEntry, p *Progress) ([]FileEntry, []error, error) {
	srcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, nil, fmt.Errorf("abs source: %w", err)
	}
	dstDir, err = filepath.Abs(dstDir)
	if err != nil {
		return nil, nil, fmt.Errorf("abs dest: %w", err)
	}

	var dirs []FileEntry
	var scanErrors []error
	createdDirs := make(map[string]bool)

	err = filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		if err != nil {
			if opts.SkipErrors {
				scanErrors = append(scanErrors, fmt.Errorf("walk %s: %w", path, err))
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			return fmt.Errorf("walk %s: %w", path, err)
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("rel %s: %w", path, err)
		}

		dstPath := filepath.Join(dstDir, relPath)

		info, err := d.Info()
		if err != nil {
			if opts.SkipErrors {
				scanErrors = append(scanErrors, fmt.Errorf("info %s: %w", path, err))
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			return fmt.Errorf("info %s: %w", path, err)
		}

		if d.IsDir() {
			// Create destination directory only if not already created
			if !createdDirs[dstPath] {
				if !opts.DryRun {
					if err := os.MkdirAll(dstPath, info.Mode().Perm()); err != nil {
						if opts.SkipErrors {
							scanErrors = append(scanErrors, fmt.Errorf("mkdir %s: %w", dstPath, err))
							return fs.SkipDir
						}
						return fmt.Errorf("mkdir %s: %w", dstPath, err)
					}
				}
				// Mark this dir and all its parents up to dstDir as created
				dirIter := dstPath
				for dirIter != dstDir && dirIter != "." && dirIter != "/" {
					createdDirs[dirIter] = true
					dirIter = filepath.Dir(dirIter)
				}
			}
			
			dirs = append(dirs, FileEntry{
				SrcPath: path,
				DstPath: dstPath,
				RelPath: relPath,
				Info:    info,
			})
			return nil
		}

		// Check for symlink
		isSymlink := d.Type()&os.ModeSymlink != 0
		if isSymlink {
			// Get the lstat info for symlinks
			linfo, lerr := os.Lstat(path)
			if lerr == nil {
				info = linfo
			} else if opts.SkipErrors {
				scanErrors = append(scanErrors, fmt.Errorf("lstat symlink %s: %w", path, lerr))
				return nil
			} else {
				return fmt.Errorf("lstat symlink %s: %w", path, lerr)
			}
		}

		entry := FileEntry{
			SrcPath:   path,
			DstPath:   dstPath,
			RelPath:   relPath,
			Info:      info,
			IsSymlink: isSymlink,
		}

		if p != nil {
			if !isSymlink {
				p.AddDiscoveredFile(info.Size())
			} else {
				p.AddDiscoveredFile(0)
			}
		}

		out <- entry

		return nil
	})

	if err != nil {
		return nil, scanErrors, err
	}

	return dirs, scanErrors, nil
}



