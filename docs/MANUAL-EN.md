# fastcopy Manual

`fastcopy` is a command-line utility for Linux (and other supported platforms) written in Go. It is designed to copy files at extremely high speeds, overcoming the limitations of traditional tools like `cp` and `rsync`.

## How It Works

`fastcopy` achieves extreme speeds by combining several cutting-edge techniques:

1. **Advanced Parallelism**: Instead of copying files one by one serially, the utility separates files into two queues:
   - **Small Files (< 64MB)**: Processed with very high parallelism (default: `2 * number_of_CPUs`).
   - **Large Files (≥ 64MB)**: Processed with a reduced limit to avoid I/O saturation on the disk.
2. **Concurrent Block Copying**: Absurdly large files (≥ 1GB) are not just sent to a queue; they are logically "sliced" (100MB chunks) and copied simultaneously by multiple workers, boosting write speeds.
3. **Zero-Copy Transfer (In-Kernel)**: On Linux, the utility directly triggers the `copy_file_range` system call, performing the transfer directly within the Kernel, without bytes needing to cross the boundary into "userspace". This also enables instant copies using COW (Copy-On-Write) on Btrfs/XFS system files (reflink).
4. **Fast Space Allocation**: Uses the `fallocate` system call to ensure contiguous blocks on the destination disk before writing, eliminating fragmentation and avoiding "disk full" failures in the middle of large copies.
5. **Active Cache Communication (fadvise)**: Uses `posix_fadvise` to prepare the read cache and discard files from memory as soon as they are copied (`FADV_DONTNEED`). Your server/desktop will no longer slow down or freeze because Linux filled 100% of your RAM with read cache while transferring 50GB.
6. **Highly Optimized Incremental Mode**: Scans the entire tree and ignores files that already exist at the destination with the exact same size and modification date (`mtime`). Existing symbolic links at the destination are detected preventively and ignored.

---

## Installation and Compilation

### Requirements

*   Go Language (version 1.21 or higher).
*   Preferred environment: **Linux**. For Windows/macOS, the code enters "fallback" mode (uses normal buffers and gracefully fails Linux-exclusive kernel system calls).

### Compilation
```bash
cd /path/to/your/repository/fastcopy
go build -o fastcopy ./cmd/fastcopy/
```

To make the utility accessible from anywhere in the system:
```bash
sudo mv fastcopy /usr/local/bin/
# or
go install ./cmd/fastcopy/
```

### Graphical Interface Compilation (Fyne)

The GUI requires X11/OpenGL development dependencies to compile:

```bash
# Ubuntu/Debian
sudo apt-get install -y \
  libx11-dev libxcursor-dev libxrandr-dev \
  libxinerama-dev libxi-dev libglx-dev \
  libgl1-mesa-dev libxxf86vm-dev

# Fedora/RHEL
sudo dnf install -y \
  libX11-devel libXcursor-devel libXrandr-devel \
  libXinerama-devel libXi-devel mesa-libGL-devel

# Compile the GUI
go build -o fastcopy-gui ./cmd/fastcopy-gui/
```

---

## How to Use

The most basic usage is similar to running `cp -a`:

```bash
fastcopy /path/to/source /path/to/destination
```
*   Unlike `cp`, the default behavior is **recursive**.
*   A real-time progress bar will be displayed with estimated time and copy rate (MB/s or GB/s).

### Available Options and Flags

The utility accepts the following configuration parameters:

| Flag / Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-w N` | Integer | `NumCPU * 2` | Defines the maximum number of parallel "workers". Increase if using HDDs with high NCQ levels, or decrease if there is significant bottlenecking. |
| `--checksum` | Boolean | `false` | Calculates data integrity via **SHA256** hash _while_ the file is in transit, generating a report at the end (no double-read impact). |
| `--dry-run` | Boolean | `false` | Only scans directories and lists in the terminal what would be executed (useful for auditing or incremental testing). |
| `--force` | Boolean | `false` | Ignores the intelligent incremental check, forcing the reading and overwriting of **all data and files**. |
| `--no-archive` | Boolean | `false` | Prevents the utility from preserving permissions, dates (`mtime`/`atime`), and ownership of source files. |
| `--quiet` | Boolean | `false` | Removes the progress bar, printing text only at the end or if an error occurs (perfect for CI/CD scripts and automation). |
| `--version` | Boolean | `false` | Shows system version and exits. |

### Advanced Practical Examples

1.  **Forcing 64 simultaneous processes and generating verifiable SHA256 hashes**:
    ```bash
    fastcopy -w 64 --checksum /mnt/server/data /local_data/backup
    ```

2.  **Checking what would be copied today (Incremental Simulation mode)**:
    ```bash
    fastcopy --dry-run /data/active /files/historical
    ```

3.  **Performing a backup to run silently in the background via Cron**:
    ```bash
    fastcopy --quiet /var/log /backup/log
    ```

---

## Common Troubleshooting

### 1. `non-root users can't change ownership` (Error changing owner)
When copying with "archive" mode (the default) enabled, the system will try to replicate who owns the file (`UID`/`GID`). If you are not using the `root` user, Linux will deny this operation and you may see the log message (although the utility tries to continue the work gracefully). Run the utility with `sudo` if there is a real need to replicate other users' permissions.

### 2. Copying to a network disk (NFS / Samba)
Systems that send data to networks (`nfs`, `cifs`, etc.) will not be able to use certain "magic" Zero-Copy logic via `copy_file_range` or pre-allocation (`fallocate`). The utility is configured to "fail gracefully" and revert to standard read buffers (`io.Copy`), maintaining its high concurrency speed even if the destination system does not support these features.

### 3. The speed shown in the progress bar dropped suddenly
When workers start working on the "Giant Files Queue", the I/O impact on the motherboard and HDD spikes. If the number of parallel workers is high (on mechanical SATA HDDs, for example), the HDD heads will cause significant Seek, momentarily freezing the rates. Try lowering the thread count to `-w 4` or `-w 8` if dealing exclusively with slow HDDs.
