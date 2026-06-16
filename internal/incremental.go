package internal

import (
	"os"
)

// NeedsCopy checks whether a source file needs to be copied to the destination.
// It returns false (skip) if the destination file already exists with the same
// size and modification time. If force is true, always returns true.
func NeedsCopy(srcPath, dstPath string, srcInfo os.FileInfo, force bool) bool {
	if force {
		return true
	}

	dstInfo, err := os.Lstat(dstPath)
	if err != nil {
		// Destination doesn't exist → needs copy
		return true
	}

	// If types differ (e.g., file vs symlink), needs copy
	if srcInfo.Mode().Type() != dstInfo.Mode().Type() {
		return true
	}

	// For symlinks, check if the destination is a symlink pointing to the same target
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		srcTarget, err := os.Readlink(srcPath)
		if err != nil {
			return true // read error -> force copy
		}
		
		dstTarget, err := os.Readlink(dstPath)
		if err != nil {
			return true // dest is not a symlink or read error -> force copy
		}
		
		if srcTarget == dstTarget {
			return false // Targets are identical, no need to copy
		}
		return true
	}

	// Compare size and modification time
	if srcInfo.Size() != dstInfo.Size() {
		return true
	}

	if !srcInfo.ModTime().Equal(dstInfo.ModTime()) {
		return true
	}

	return false
}
