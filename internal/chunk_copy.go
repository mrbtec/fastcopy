package internal

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// concurrentCopy slices a file into chunks and copies them concurrently.
// It is optimized for very large files (> 1GB).
func concurrentCopy(dst, src *os.File, size int64, maxWorkers int) error {
	const chunkSize = 100 * 1024 * 1024 // 100 MB chunks

	var wg sync.WaitGroup
	errCh := make(chan error, (size/chunkSize)+1)

	// Semaphore to limit concurrent chunk workers
	sem := make(chan struct{}, maxWorkers)

	for offset := int64(0); offset < size; offset += chunkSize {
		off := offset
		length := int64(chunkSize)
		if off+length > size {
			length = size - off
		}

		wg.Add(1)
		go func(o, l int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := copyChunk(dst, src, o, l); err != nil {
				errCh <- err
			}
		}(off, length)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func copyChunk(dst, src *os.File, offset, length int64) error {
	section := io.NewSectionReader(src, offset, length)
	buf := getBuf()
	defer putBuf(buf)

	var writeOff = offset
	for {
		n, err := section.Read(buf)
		if n > 0 {
			// write exactly what was read
			wn, werr := dst.WriteAt(buf[:n], writeOff)
			if werr != nil {
				return fmt.Errorf("writeAt offset %d: %w", writeOff, werr)
			}
			if wn != n {
				return fmt.Errorf("short write at offset %d", writeOff)
			}
			writeOff += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read offset %d: %w", offset, err)
		}
	}
	return nil
}
