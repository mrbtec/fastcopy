# Implementation Plan for Indexing in fastcopy

## Overview
Based on the analysis of `OPCOES-INDEXACAO.md` and the **actual architecture of the project**, this plan has been revised to fix technical issues and align with the existing code.

---

## ⚠️ Identified Problems and Fixes in the Implementation

### 1. Code Did Not Compile – 5 Syntax Errors
The original implementation mixed two architectures (gob + SQLite) causing fatal compilation errors:

| File | Error | Action |
|------|-------|--------|
| `core.go` | Duplicate `package index` declaration; `Query` type conflicted with `index.go` | **Removed** – types already defined in `index.go` |
| `storage.go` | Two concatenated implementations: gob (lines 1‑55) + SQLite (lines 56‑97) | **Rewritten** – kept only the gob version |
| `suffix_trie.go` | Duplicate `package index` declaration | **Removed** – placeholder empty |

### 2. Missing SQLite Dependency in `go.mod`
Four files (`file_index.go`, `content_index.go`, `hash_index.go`, `meta_index.go`) imported `database/sql` and `github.com/mattn/go-sqlite3`, which **are not present in `go.mod`**. Action: all removed.

### 3. Placeholder Files Without Functionality
`prefix_trie.go` and `suffix_trie.go` were empty structs with no methods. Action: removed.

### 4. `main.go` Referenced Non‑existent `idx.NewStorage`
The CLI called `idx.NewStorage()` (SQLite API) which no longer exists. Action: rewritten to use `idx.Save()` / `idx.Load()` directly.

---

## Final Architecture (Compiles ✅)
```
fastcopy/internal/index/
├── index.go      # Types Entry, Index, Query + FindDuplicates
├── builder.go    # BuildFromScan (re‑uses existing scanner + checksum)
├── search.go     # Search (glob, size, hash) + LookupByPath
└── storage.go    # Save/Load via encoding/gob
```
**Zero external dependencies.** Persistence is handled with the standard library `encoding/gob`.

---

## Core Types (`index.go`)
```go
// Entry represents a file or directory indexed.
type Entry struct {
    Path    string    // relative path to the indexed root
    Size    int64
    ModTime time.Time
    Mode    uint32    // permissions
    Hash    string    // SHA‑256 hex (empty if not calculated)
    IsDir   bool
}

// Index is the main index structure.
type Index struct {
    Version   int               // format version
    RootPath  string            // indexed root directory
    CreatedAt time.Time
    Entries   []Entry           // list ordered by Path
    HashMap   map[string][]int  // hash → entry indices (for deduplication)
}

// Query describes search criteria.
type Query struct {
    Name       string // glob pattern for name/path
    MinSize    int64
    MaxSize    int64 // 0 = no upper limit
    Hash       string
    Duplicates bool   // if true, return only duplicates
}
```
---

## Index Construction (`builder.go`)
Re‑uses `ScanDirAsync` and `ChecksumFile` via an adapter:
```go
func BuildFromScan(ctx context.Context, rootPath string, computeHash bool) (*Index, error)
```
- Uses `ScanDirAsync` with `DryRun: true` (no destination directories are created).
- Hash calculation is **opt‑in** via the `computeHash` flag.
- Populates `HashMap` during construction for O(1) hash look‑ups.

---

## Search (`search.go`)
| Operation | Implementation | Complexity |
|-----------|----------------|------------|
| Name (glob) search | `filepath.Match(pattern, entry.Path)` | O(n) |
| Hash search | `idx.HashMap[hash]` (direct lookup) | O(1) |
| Size filter | Linear filter with min/max | O(n) |
| List duplicates | Iterate `HashMap`, filter where length > 1 | O(n) |

Also includes `LookupByPath(relPath string) (Entry, bool)` for future integration with `NeedsCopy`.
---

## Persistence (`storage.go`)
```go
func (idx *Index) Save(path string) error   // gob serialization
func Load(path string) (*Index, error)       // gob deserialization + version check
```
- File format: binary gob (efficient, no external deps).
- Version validation on load for future compatibility.
---

## CLI Integration (`cmd/fastcopy/main.go`)
### Implemented Flags
```text
--index-build    // build an index for the source directory
--index-search   // search term/pattern in the index
--index-path     // path to the .idx file (default: fastcopy.idx)
--index-hash     // compute SHA‑256 during index building
--index-dupes    // list duplicate files
```
### Usage Examples
```bash
# Fast index (metadata only)
fastcopy --index-build /data/backup --index-path backup.idx

# Index with hashes (slower, enables deduplication)
fastcopy --index-build /data/backup --index-hash --index-path backup.idx

# Search files by glob pattern
fastcopy --index-search "*.log" --index-path backup.idx

# List duplicate files
fastcopy --index-dupes --index-path backup.idx
```
---

## Future Integration: Incremental Copy by Hash
The index can replace the `size+mtime` heuristic in `NeedsCopy` with a hash comparison:
```go
// In incremental.go (future improvement)
func NeedsCopyWithIndex(srcPath string, srcInfo os.FileInfo, idx *index.Index) bool {
    entry, found := idx.LookupByPath(srcPath)
    if !found { return true }
    currentHash, _ := ChecksumFile(srcPath)
    return currentHash != entry.Hash
}
```
---

## Pending Tests
| Test Case | Validation |
|-----------|------------|
| Build + Save + Load round‑trip | Index is correctly preserved |
| Glob search (`*.go`) | Returns only `.go` files |
| Hash search | Returns the correct path |
| Duplicate detection | Finds files with identical content |
| Empty directory | No panic (edge case) |
| Corrupted index file | `Load` returns an error |
| Large index (100k+ entries) | < 500 ms for build + search |

> **Note:** Unit tests (`index_test.go`) have not yet been added.
---

## Risks & Mitigations
| Risk | Impact | Mitigation |
|------|--------|------------|
| Large index (> 5 M entries) | High RAM usage | Keep `Entry` minimal; consider migrating to `bbolt` if needed |
| Slow hash calculation on HDD with many files | Long build time | `--index-hash` is opt‑in; without it the build is fast (stat only) |
| Stale index | Incorrect results | Store `CreatedAt`; warn if index is older than 24 h |
| Gob format incompatibility across Go versions | Unreadable index | Include `Version` field; validate on load |
---

## Conclusion
The revised implementation provides a **lightweight, pure‑Go index** with fast look‑ups, pagination support, and a clean CLI API. It is ready for further integration with the GUI and incremental copy logic, and it can be safely published as part of the fastcopy project.
