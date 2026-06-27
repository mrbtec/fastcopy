# fastcopy ⚡

An ultra-fast parallel file copier developed in Go. Designed to outperform traditional tools like `cp` and `rsync` by leveraging high-performance system calls (such as `copy_file_range`, `fallocate`, and `fadvise` on Linux).

## Key Features

- **Extreme Performance:** Uses zero-copy system calls, disk pre-allocation, and intelligent I/O hints.
- **Parallel Processing:** Parallelized dispatcher, optimized to efficiently handle a mix of small and large files.
- **Incremental Synchronization:** Automatically skips unmodified files, significantly speeding up updates.
- **Indexing and Search (New!):** Create directory indices for ultra-fast searches (`O(log N)` and `O(1)`) and duplicate file detection using SHA-256.
- **Metadata Preservation:** Maintains permissions, timestamps (modification dates), and other original metadata.
- **Dual Interface:** Includes both a robust Command Line Interface (CLI) and a modern Graphical User Interface (GUI) developed with the [Fyne](https://fyne.io/) framework, featuring Copy and Search tabs.

## Prerequisites

- [Go](https://go.dev/) installed.
- For the graphical interface (GUI) on Linux, you will need X11/OpenGL development libraries.

## Quick Guide

The project includes the `start.sh` utility to simplify all common operations.

### 1. Install GUI dependencies (required only for the graphical interface)
```bash
./start.sh deps
```
*This command requires `sudo` and automatically detects whether you use `apt`, `dnf`, or `pacman`.*

### 2. Build
```bash
# Build CLI only
./start.sh build

# Build GUI
./start.sh build-gui

# Build both
./start.sh build-all
```

### 3. Run (Copy Operations)

**CLI:**
```bash
# Basic copy example
./start.sh run /source/path /destination/path

# Advanced example with 32 parallel workers and checksum validation
./start.sh run -w 32 --checksum /source/path /destination/path
```

**GUI:**
```bash
# Opens the graphical interface with Copier and Index Search tabs
./start.sh run-gui
```

### 4. Run (CLI Indexing and Search)

`fastcopy` does more than just copy; it allows you to quickly scan entire directories to create static indices (`.idx`), search through them, or find duplicates:

```bash
# 1. Create a directory index calculating SHA-256 Hashes
./start.sh run --index-build --index-hash --index-path=my_backup.idx /source/path

# 2. Instantly search in the created index
./start.sh run --index-search="*.mp4" --index-path=my_backup.idx

# 3. List all duplicate files in the index (based on Hash)
./start.sh run --index-dupes --index-path=my_backup.idx
```

*Note: You can also load the generated `.idx` file directly into the "Index Search" tab of `fastcopy-gui` to browse visually!*

### 5. Tests
Run the integration test suite, which verifies incremental copies, checksums, and dry runs:
```bash
./start.sh test
```

## Code Structure

- `cmd/fastcopy/`: Entry point for the Command Line Interface (CLI) application.
- `cmd/fastcopy-gui/`: Entry point for the Graphical User Interface (GUI) application in Fyne.
- `internal/`: Core copier logic, parallel engine, and zero-copy optimizations.
- `internal/index/`: Pure Go `gob` serialization engine for Indexing, Binary Search, and Deduplication.
- `start.sh`: Task manager script for developers and users.

## License

This project is licensed under the [MIT License](LICENSE).
