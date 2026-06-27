package index

import (
	"path/filepath"
	"sort"
	"strings"
)

// Search executes a Query against the Index and returns matching entries.
func (idx *Index) Search(q Query) []Entry {
	var results []Entry
	
	// Exact hash search - O(1)
	if q.Hash != "" {
		if indices, ok := idx.HashMap[q.Hash]; ok {
			for _, pos := range indices {
				e := idx.Entries[pos]
				if passesFilters(e, q) {
					results = append(results, e)
				}
			}
		}
	} else if q.Name != "" && !hasGlobChars(q.Name) {
		// Exact path search - O(1)
		if pos, ok := idx.PathMap[q.Name]; ok {
			e := idx.Entries[pos]
			if passesFilters(e, q) {
				results = append(results, e)
			}
		}
	} else if q.Name != "" && strings.HasSuffix(q.Name, "*") && !hasGlobChars(strings.TrimSuffix(q.Name, "*")) {
		// Prefix search - O(log N + K)
		prefix := strings.TrimSuffix(q.Name, "*")
		
		// Find first element that is >= prefix
		startIdx := sort.Search(len(idx.Entries), func(i int) bool {
			return idx.Entries[i].Path >= prefix
		})
		
		for i := startIdx; i < len(idx.Entries); i++ {
			e := idx.Entries[i]
			if !strings.HasPrefix(e.Path, prefix) {
				break // Since it's sorted, we can stop when prefix doesn't match anymore
			}
			if passesFilters(e, q) {
				results = append(results, e)
			}
		}
	} else {
		// Linear scan for complex globs or full enumeration - O(N)
		for _, e := range idx.Entries {
			if q.Name != "" {
				match, err := filepath.Match(q.Name, e.Path)
				if err != nil || !match {
					continue
				}
			}
			if passesFilters(e, q) {
				results = append(results, e)
			}
		}
	}

	// If duplicates requested, filter after gathering
	if q.Duplicates {
		dupMap := make(map[string]bool)
		for _, e := range results {
			if len(idx.HashMap[e.Hash]) > 1 {
				dupMap[e.Path] = true
			}
		}
		var dupResults []Entry
		for _, e := range results {
			if dupMap[e.Path] {
				dupResults = append(dupResults, e)
			}
		}
		results = dupResults
	}

	// Apply pagination (Offset/Limit)
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.Offset >= len(results) {
		return []Entry{}
	}
	end := len(results)
	if q.Limit > 0 && q.Offset+q.Limit < end {
		end = q.Offset + q.Limit
	}
	return results[q.Offset:end]
}

// passesFilters checks size constraints (hash is handled in the main loops if needed).
func passesFilters(e Entry, q Query) bool {
	if q.MinSize > 0 && e.Size < q.MinSize {
		return false
	}
	if q.MaxSize > 0 && e.Size > q.MaxSize {
		return false
	}
	// Hash is checked in exact hash match, but if we got here via glob, we should filter it
	if q.Hash != "" && e.Hash != q.Hash {
		return false
	}
	return true
}

// hasGlobChars returns true if the pattern contains glob meta‑characters.
func hasGlobChars(p string) bool {
	return strings.ContainsAny(p, "*?[]")
}

// LookupByPath returns the Entry for a given relative path, if it exists (O(1)).
func (idx *Index) LookupByPath(relPath string) (Entry, bool) {
	if pos, ok := idx.PathMap[relPath]; ok {
		return idx.Entries[pos], true
	}
	return Entry{}, false
}
