package internal

import (
	"fmt"
	"os"
	"syscall"
)

// PreserveMetadata copies the metadata (permissions, ownership, timestamps)
// from the source path to the destination path.
func PreserveMetadata(srcPath, dstPath string, srcInfo os.FileInfo) error {
	// Preserve permissions
	if err := os.Chmod(dstPath, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("chmod %s: %w", dstPath, err)
	}

	// Preserve ownership (requires root for changing to other users)
	if stat, ok := srcInfo.Sys().(*syscall.Stat_t); ok {
		if err := os.Lchown(dstPath, int(stat.Uid), int(stat.Gid)); err != nil {
			// Non-fatal: non-root users can't change ownership
			// We log but don't fail
			_ = err
		}
	}

	// Preserve timestamps (atime + mtime)
	atime, mtime := statTimes(srcInfo)

	if err := os.Chtimes(dstPath, atime, mtime); err != nil {
		return fmt.Errorf("chtimes %s: %w", dstPath, err)
	}

	return nil
}

// PreserveDirMetadata is like PreserveMetadata but for directories.
// It should be called after all contents have been copied.
func PreserveDirMetadata(srcPath, dstPath string, srcInfo os.FileInfo) error {
	return PreserveMetadata(srcPath, dstPath, srcInfo)
}

// CopySymlink reads the symlink target from src and creates a new
// symlink at dst pointing to the same target.
func CopySymlink(srcPath, dstPath string) error {
	target, err := os.Readlink(srcPath)
	if err != nil {
		return fmt.Errorf("readlink %s: %w", srcPath, err)
	}

	// Remove existing destination if it exists
	if _, err := os.Lstat(dstPath); err == nil {
		if err := os.Remove(dstPath); err != nil {
			return fmt.Errorf("remove existing %s: %w", dstPath, err)
		}
	}

	if err := os.Symlink(target, dstPath); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", dstPath, target, err)
	}

	return nil
}
