package internal

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// CopyWithChecksum copies the contents of src to dst and computes a SHA256
// checksum of the data simultaneously using io.MultiWriter.
// Returns the hex-encoded checksum string.
func CopyWithChecksum(dst, src *os.File, size int64) (string, error) {
	h := sha256.New()
	w := io.MultiWriter(dst, h)

	buf := getBuf()
	defer putBuf(buf)

	_, err := io.CopyBuffer(w, src, buf)
	if err != nil {
		return "", fmt.Errorf("copy with checksum: %w", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// ChecksumFile computes the SHA256 checksum of an existing file.
func ChecksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()

	buf := getBuf()
	defer putBuf(buf)

	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
