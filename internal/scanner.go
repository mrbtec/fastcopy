package internal

import (
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

// ScanResult holds the result of a directory scan.
type ScanResult struct {
	Files      []FileEntry
	TotalFiles int64
	TotalBytes int64
	dirs       []FileEntry // directories for later metadata preservation
}

// ScanDir recursively scans srcDir and builds a list of FileEntry items
// that map source paths to destination paths under dstDir.
// It also creates all necessary directories in the destination.
func ScanDir(srcDir, dstDir string) (*ScanResult, error) {
	srcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, fmt.Errorf("abs source: %w", err)
	}
	dstDir, err = filepath.Abs(dstDir)
	if err != nil {
		return nil, fmt.Errorf("abs dest: %w", err)
	}

	result := &ScanResult{}

	// Track directories we need to set metadata on later
	var dirs []FileEntry
	createdDirs := make(map[string]bool)

	err = filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", path, err)
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("rel %s: %w", path, err)
		}

		dstPath := filepath.Join(dstDir, relPath)

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("info %s: %w", path, err)
		}

		if d.IsDir() {
			// Create destination directory only if not already created
			if !createdDirs[dstPath] {
				if err := os.MkdirAll(dstPath, info.Mode().Perm()); err != nil {
					return fmt.Errorf("mkdir %s: %w", dstPath, err)
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
			}
		}

		entry := FileEntry{
			SrcPath:   path,
			DstPath:   dstPath,
			RelPath:   relPath,
			Info:      info,
			IsSymlink: isSymlink,
		}

		result.Files = append(result.Files, entry)
		result.TotalFiles++
		if !isSymlink {
			result.TotalBytes += info.Size()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Store dirs for later metadata preservation
	result.dirs = dirs

	return result, nil
}

// Dirs returns the directories found during scanning (for metadata preservation).
func (r *ScanResult) Dirs() []FileEntry {
	return r.dirs
}



