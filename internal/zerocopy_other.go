//go:build !linux

package internal

import (
	"os"
)

// platformCopyFile is the non-Linux fallback using buffered io.Copy.
func platformCopyFile(dst, src *os.File, size int64) error {
	return fallbackCopy(dst, src, size)
}


