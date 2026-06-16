//go:build windows || plan9 || js || wasm

package internal

import (
	"os"
	"time"
)

func statTimes(fi os.FileInfo) (atime, mtime time.Time) {
	mtime = fi.ModTime()
	atime = mtime // fallback
	return
}
