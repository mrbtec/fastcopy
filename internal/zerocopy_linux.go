//go:build linux

package internal

import (
	"fmt"
	"io"
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
// Strategy: fallocate → fadvise(SEQUENTIAL) → copy_file_range → splice → fallback
func platformCopyFile(dst, src *os.File, size int64) error {
	// Step 1: Pre-allocate destination space
	_ = preallocate(dst, size)

	// Step 2: Hint sequential read pattern
	_ = adviseSequential(src)

	// Step 3: Zero-copy transfer via kernel
	err := zeroCopy(dst, src, size)
	if err == nil {
		// Step 4: Release pages from cache (don't pollute system cache)
		_ = adviseDontNeed(src)
		_ = adviseDontNeed(dst)
		return nil
	}

	// Step 5: Fallback 1 - Zero-copy via pipe (splice) for cross-device/filesystem limits
	src.Seek(0, io.SeekStart)
	dst.Seek(0, io.SeekStart)
	dst.Truncate(0)
	_ = preallocate(dst, size)

	err = spliceCopy(dst, src, size)
	if err == nil {
		_ = adviseDontNeed(src)
		_ = adviseDontNeed(dst)
		return nil
	}

	// Step 6: Fallback 2 - standard buffered userspace copy
	src.Seek(0, io.SeekStart)
	dst.Seek(0, io.SeekStart)
	dst.Truncate(0)
	return fallbackCopy(dst, src, size)
}

// spliceCopy performs a zero-copy transfer via a kernel pipe.
// This is used as a fallback when copy_file_range fails (e.g., cross-device).
func spliceCopy(dst, src *os.File, size int64) error {
	srcFd := int(src.Fd())
	dstFd := int(dst.Fd())

	// Create a pipe for the splice transfer
	var pipeFds [2]int
	if err := unix.Pipe2(pipeFds[:], unix.O_CLOEXEC); err != nil {
		return fmt.Errorf("pipe2: %w", err)
	}
	defer unix.Close(pipeFds[0]) // read end
	defer unix.Close(pipeFds[1]) // write end

	var written int64
	for written < size {
		// Use chunks to avoid pipe buffer issues
		toWrite := size - written
		if toWrite > 4*1024*1024 { // 4MB chunks
			toWrite = 4 * 1024 * 1024
		}

		// Splice from source file to pipe write end
		n, err := unix.Splice(srcFd, nil, pipeFds[1], nil, int(toWrite), unix.SPLICE_F_MORE)
		if err != nil {
			return fmt.Errorf("splice src->pipe: %w", err)
		}
		if n == 0 {
			break
		}

		// Splice from pipe read end to destination file
		var pipeWritten int64
		for pipeWritten < n {
			m, err := unix.Splice(pipeFds[0], nil, dstFd, nil, int(n-pipeWritten), unix.SPLICE_F_MORE)
			if err != nil {
				return fmt.Errorf("splice pipe->dst: %w", err)
			}
			if m == 0 {
				break
			}
			pipeWritten += m
		}
		written += pipeWritten
	}

	// Ensure data is synced to disk
	return unix.Fsync(dstFd)
}


