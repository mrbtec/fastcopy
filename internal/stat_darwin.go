//go:build darwin || freebsd || netbsd || openbsd

package internal

import (
	"os"
	"syscall"
	"time"
)

func statTimes(fi os.FileInfo) (atime, mtime time.Time) {
	mtime = fi.ModTime()
	atime = mtime // fallback
	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		atime = time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
		mtime = time.Unix(stat.Mtimespec.Sec, stat.Mtimespec.Nsec)
	}
	return
}
