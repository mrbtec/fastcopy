package internal

import (
	"fmt"
	"io"
	"os"
	"sync"
)

const (
	// bufSize is the size of each reusable copy buffer (4 MB).
	bufSize = 4 * 1024 * 1024

	// LargeFileThreshold is the size above which a file is considered "large"
	// and routed to a separate, limited worker queue.
	LargeFileThreshold = 64 * 1024 * 1024 // 64 MB
)

// bufPool is a sync.Pool for reusable 4MB copy buffers to minimize GC pressure.
var bufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, bufSize)
		return buf
	},
}

func getBuf() []byte {
	return bufPool.Get().([]byte)
}

func putBuf(buf []byte) {
	bufPool.Put(buf) //nolint:staticcheck
}

// Options configures a file copy operation.
type Options struct {
	Archive      bool   // Preserve permissions, ownership, timestamps
	Checksum     bool   // Compute SHA256 during copy
	Force        bool   // Disable incremental (always recopy)
	SkipErrors   bool   // Skip files/dirs that cause errors
	ErrorLog     string // File path to save the error log
	DryRun       bool   // Do not modify the filesystem
	RemoveSource bool   // Delete source files and empty directories after successful copy
}

// CopyFile copies a single file from srcPath to dstPath, applying the
// specified options. It uses platform-specific optimizations when available.
// Returns the SHA256 checksum (empty string if checksum is disabled).
func CopyFile(srcPath, dstPath string, srcInfo os.FileInfo, opts Options) (checksum string, err error) {
	// Handle symlinks
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		if err := CopySymlink(srcPath, dstPath); err != nil {
			return "", err
		}
		return "", nil
	}

	// Open source
	src, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("open source %s: %w", srcPath, err)
	}
	defer src.Close()

	// Create destination with source permissions (will be overwritten by metadata if archive)
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return "", fmt.Errorf("create destination %s: %w", dstPath, err)
	}
	defer func() {
		cerr := dst.Close()
		if err == nil {
			err = cerr
		}
	}()

	size := srcInfo.Size()

	// Copy data
	if opts.Checksum {
		checksum, err = CopyWithChecksum(dst, src, size)
		if err != nil {
			return "", fmt.Errorf("copy %s: %w", srcPath, err)
		}
	} else {
		// Use platform-optimized copy (zero-copy on Linux)
		if err := platformCopyFile(dst, src, size); err != nil {
			return "", fmt.Errorf("copy %s: %w", srcPath, err)
		}
	}

	// Preserve metadata (permissions, ownership, timestamps)
	if opts.Archive {
		if err := PreserveMetadata(srcPath, dstPath, srcInfo); err != nil {
			return checksum, fmt.Errorf("metadata %s: %w", dstPath, err)
		}
	}

	return checksum, nil
}

// fallbackCopy performs a standard buffered copy using a pooled 4MB buffer.
// Used when zero-copy syscalls are unavailable or fail.
func fallbackCopy(dst, src *os.File, size int64) error {
	if size < 32*1024 {
		// Small file: use standard io.Copy which allocates a small 32KB buffer
		_, err := io.Copy(dst, src)
		return err
	}

	if size >= 1024*1024*1024 { // 1 GB
		// Very large file: slice and copy chunks concurrently
		return concurrentCopy(dst, src, size, 4)
	}

	buf := getBuf()
	defer putBuf(buf)

	_, err := io.CopyBuffer(dst, src, buf)
	return err
}
