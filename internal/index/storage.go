package index

import (
	"encoding/gob"
	"fmt"
	"os"
)

// Save writes the Index to disk using gob encoding.
// The file is created with mode 0644; any existing content is replaced.
func (idx *Index) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create index file: %w", err)
	}
	defer f.Close()

	if err := gob.NewEncoder(f).Encode(idx); err != nil {
		return fmt.Errorf("encode index: %w", err)
	}
	return nil
}

// Load reads a previously saved Index from disk.
func Load(path string) (*Index, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open index file: %w", err)
	}
	defer f.Close()

	var idx Index
	if err := gob.NewDecoder(f).Decode(&idx); err != nil {
		return nil, fmt.Errorf("decode index: %w", err)
	}

	// Basic version validation
	if idx.Version < 1 {
		return nil, fmt.Errorf("unsupported index version: %d", idx.Version)
	}

	return &idx, nil
}
