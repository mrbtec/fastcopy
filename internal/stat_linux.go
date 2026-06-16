//go:build linux || solaris

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
		atime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
		mtime = time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec)
	}
	return
}
