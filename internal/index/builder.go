package index

import (
	"context"
	"sort"
	"time"

	"github.com/moises/fastcopy/internal"
)

// BuildFromScan creates an Index by scanning the given rootPath.
// If computeHash is true, a SHA‑256 hash is calculated for each regular file.
func BuildFromScan(ctx context.Context, rootPath string, computeHash bool) (*Index, error) {
	idx := &Index{
		Version:   1,
		RootPath:  rootPath,
		CreatedAt: time.Now(),
		HashMap:   make(map[string][]int),
		PathMap:   make(map[string]int),
	}

	outChan := make(chan internal.FileEntry, 1000)
	// Use ScanDirAsync with DryRun to avoid creating destination dirs.
	opts := internal.Options{DryRun: true, SkipErrors: true}

	go func() {
		defer close(outChan)
		// src and dst are the same because we only need to scan.
		internal.ScanDirAsync(ctx, rootPath, rootPath, opts, outChan, nil)
	}()

	for fe := range outChan {
		entry := Entry{
			Path:    fe.RelPath,
			Size:    fe.Info.Size(),
			ModTime: fe.Info.ModTime(),
			Mode:    uint32(fe.Info.Mode()),
			IsDir:   fe.Info.IsDir(),
		}
		if computeHash && !entry.IsDir && !fe.IsSymlink {
			if h, err := internal.ChecksumFile(fe.SrcPath); err == nil {
				entry.Hash = h
			}
		}
		idx.Entries = append(idx.Entries, entry)
	}

	// Sort entries by path to allow binary search
	sort.Slice(idx.Entries, func(i, j int) bool {
		return idx.Entries[i].Path < idx.Entries[j].Path
	})

	// Populate maps after sorting to ensure correct indices
	for i, entry := range idx.Entries {
		if entry.Hash != "" {
			idx.HashMap[entry.Hash] = append(idx.HashMap[entry.Hash], i)
		}
		idx.PathMap[entry.Path] = i
	}

	return idx, nil
}
