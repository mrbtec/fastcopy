//go:build linux

package internal

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// zeroCopy performs a kernel-level file copy using copy_file_range syscall.
// Data never transits to userspace. On reflink-capable filesystems (Btrfs, XFS),
// this can be near-instantaneous via Copy-on-Write.
func zeroCopy(dst, src *os.File, size int64) error {
	srcFd := int(src.Fd())
	dstFd := int(dst.Fd())

	var off int64
	remaining := size

	for remaining > 0 {
		// copy_file_range can copy up to a certain amount per call
		// Use chunks of 1GB to avoid potential kernel limits
		toWrite := remaining
		if toWrite > 1<<30 { // 1 GB
			toWrite = 1 << 30
		}

		n, err := unix.CopyFileRange(srcFd, &off, dstFd, nil, int(toWrite), 0)
		if err != nil {
			return fmt.Errorf("copy_file_range: %w", err)
		}
		if n == 0 {
			break
		}
		remaining -= int64(n)
	}

	return nil
}

// preallocate reserves disk space for the file using fallocate.
// This reduces fragmentation and prevents mid-copy disk-full failures.
func preallocate(f *os.File, size int64) error {
	if size <= 0 {
		return nil
	}
	err := unix.Fallocate(int(f.Fd()), 0, 0, size)
	if err != nil {
		// fallocate not supported on all filesystems (e.g., NFS, tmpfs)
		// Fall through silently — the copy will still work, just without prealloc
		return nil
	}
	return nil
}

// adviseSequential tells the kernel to expect sequential reads on this file,
// enabling more aggressive read-ahead in the page cache.
func adviseSequential(f *os.File) error {
	return unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_SEQUENTIAL)
}

// adviseDontNeed tells the kernel to drop this file's pages from the page cache.
// This prevents bulk copy operations from evicting useful cached data.
func adviseDontNeed(f *os.File) error {
	return unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_DONTNEED)
}

// platformCopyFile performs optimized file copy on Linux using kernel syscalls.
// Strategy: fallocate → fadvise(SEQUENTIAL) → copy_file_range → fadvise(DONTNEED)
func platformCopyFile(dst, src *os.File, size int64) error {
	// Step 1: Pre-allocate destination space
	_ = preallocate(dst, size)

	// Step 2: Hint sequential read pattern
	_ = adviseSequential(src)

	// Step 3: Zero-copy transfer via kernel
	err := zeroCopy(dst, src, size)
	if err != nil {
		// Fallback: copy_file_range may fail on cross-filesystem, NFS, etc.
		// Reset file positions and use standard copy
		src.Seek(0, 0)
		dst.Seek(0, 0)
		dst.Truncate(0)
		return fallbackCopy(dst, src, size)
	}

	// Step 4: Release pages from cache (don't pollute system cache)
	_ = adviseDontNeed(src)
	_ = adviseDontNeed(dst)

	return nil
}


