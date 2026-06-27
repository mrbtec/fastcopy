# Encryption Implementation Plan for gocopy

Based on the analysis of the `OPCOES-CRIPTOGRAFIA.md` document and the **actual project architecture**, the following action plan has been structured. Significant changes were made compared to the previous version to correct technical issues and align with the existing code.

---

## ⚠️ Problems Identified in the Previous Plan

### 1. Critical Bug: Nonce Reuse in Reference Code
The sketch in `OPCOES-CRIPTOGRAFIA.md` uses the **same nonce for all chunks** within a file. This is a **fatal vulnerability** in AES-GCM: reusing a nonce with the same key allows an attacker to recover the authentication key and decrypt the data. **Each chunk must have its own unique nonce** (or use an incremental counter derived from the base nonce).

### 2. Incompatibility with Zero-Copy
The fastcopy copy engine uses `copy_file_range` (zero-copy via kernel on Linux) — data **never transit through userspace**. Encryption, by definition, needs to process data in userspace. Therefore, the encrypted flow **must automatically disable zero-copy** and use the fallback (`fallbackCopy`) or a dedicated pipeline.

### 3. Incompatibility with Concurrent Chunk Copy
The `concurrentCopy` in `chunk_copy.go` uses `WriteAt`/`SectionReader` with absolute offsets. AES-GCM introduces **overhead per chunk** (16-byte tag + nonce), which alters the write offsets at the destination relative to the read at the source. Concurrent copy with encryption would require complex offset mapping. **In the first version, encryption must use sequential copy.**

### 4. Misaligned Buffer Size
`OPCOES-CRIPTOGRAFIA.md` suggests 64KB chunks, but the project uses **4MB** buffers (`bufSize` in `filecopy.go`) via `sync.Pool`. The encryption package should reuse the existing `bufPool` to avoid duplicate allocations and GC pressure.

### 5. GCM Size Limit
AES-GCM has a practical limit of ~64GB per nonce (2³² blocks of 16 bytes). For larger files, each chunk must be treated as an independent GCM message with its own nonce.

---

## Phase 1: Creation of the `fastcopy/internal/crypto` Package

**Objective:** Correct and secure cryptographic primitives.

### 1.1 Encrypted File Format

```
[HEADER: 4 bytes magic "FCRY"]
[VERSION: 1 byte]
[BASE_NONCE: 12 bytes]
[CHUNK_0: nonce_counter(0) → ciphertext + GCM tag (16 bytes)]
[CHUNK_1: nonce_counter(1) → ciphertext + GCM tag (16 bytes)]
...
[CHUNK_N]
```

- **Magic bytes** allow identifying if a file was encrypted by fastcopy.
- **Nonce per chunk**: derive nonce by incrementing a counter over the `BASE_NONCE` (avoids reuse bug).
- **Each chunk** is an autonomous GCM message (allows granular corruption detection).

### 1.2 Main Functions

```go
// crypto.go
package crypto

// EncryptStream(reader io.Reader, writer io.Writer, key []byte) error
//   - Writes header + base_nonce
//   - Reads 4MB chunks from reader, encrypts each with incremented nonce
//   - Writes [ciphertext+tag] for each chunk

// DecryptStream(reader io.Reader, writer io.Writer, key []byte) error
//   - Reads header + base_nonce
//   - Reads encrypted chunks (4MB + 16 bytes overhead), decrypts with incremented nonce
//   - Writes plaintext to writer

// GenerateKey() ([]byte, error)
//   - Generates 32 cryptographically secure bytes

// LoadKey(path string) ([]byte, error)
//   - Reads file, validates size (== 32 bytes), returns key
```

> **Note:** Using `io.Reader`/`io.Writer` instead of paths allows direct integration with the existing copy pipeline (e.g., `io.MultiWriter` for simultaneous checksum + encryption).

### 1.3 Tests (`crypto_test.go`)

| Test Case | Validation |
|---|---|
| Encrypt → Decrypt round-trip | Data identical to original |
| Empty file | Should not fail (edge case) |
| File > 4MB (multi-chunk) | Chunks processed correctly |
| Bit flip in ciphertext | `gcm.Open` returns error (GCM integrity) |
| Incorrect key during decryption | Authentication failure |
| Nonce never repeats between chunks | Verify monotonic increment |

---

## Phase 2: Integration with the Copy Engine

**Objective:** Insert encryption into the pipeline without breaking the existing architecture.

### 2.1 Modifications in `filecopy.go`

The `CopyFile` function is the correct integration point. The encrypted flow will be:

```
src → [read] → [encrypt/decrypt] → [write] → dst
```

When `opts.Encrypt` or `opts.Decrypt` is active:
- **Skip** `platformCopyFile` (zero-copy is incompatible).
- **Skip** `concurrentCopy` (offsets misaligned due to GCM overhead).
- Use `crypto.EncryptStream` or `crypto.DecryptStream` with file descriptors directly.
- Maintain compatibility with `--checksum`: use `io.TeeReader` to feed SHA-256 **over the original data** (not over the ciphertext).

### 2.2 Modifications in `Options` (struct)

```go
type Options struct {
    // ... existing fields ...
    EncryptKey []byte // If non-nil, encrypts during copy
    DecryptKey []byte // If non-nil, decrypts during copy
}
```

### 2.3 `incremental.go` — No Changes

The incremental logic compares **size and mtime** of the file at the destination. Since the encrypted file will have a different size than the original, the incremental behavior will already work correctly (it will detect change and recopy). No change to the logic is necessary.

### 2.4 Use of Existing `bufPool`

The `crypto` package should import and use `getBuf()`/`putBuf()` from `filecopy.go` (or expose the pool via a public function). This ensures that there aren't two 4MB pools competing for memory.

---

## Phase 3: CLI Exposure

### 3.1 New Flags in `cmd/fastcopy/main.go`

```go
encryptKey := flag.String("encrypt", "", "path to 32-byte key file for AES-256-GCM encryption")
decryptKey := flag.String("decrypt", "", "path to 32-byte key file for AES-256-GCM decryption")
genKey     := flag.String("gen-key", "", "generate a new random key and save to this path, then exit")
```

**Mandatory Validations:**
- `--encrypt` and `--decrypt` are mutually exclusive.
- The key file must exist and contain exactly 32 bytes.
- Display warning if keyfile permissions are more open than `0600`.

### 3.2 `--gen-key` Command

Add a sub-action that generates a secure key and exits:

```bash
fastcopy --gen-key /path/to/my_key.bin
# Generates 32 bytes with crypto/rand, saves, sets chmod 600
```

This eliminates the dependency on `openssl` for the user.

### 3.3 User Feedback

- In standard output, display `🔒 Encryption enabled (AES-256-GCM)` when active.
- In the final summary, show estimated overhead (additional time vs pure copy).

---

## Phase 4: Documentation

1. **README.md** — "Encryption" section with quick examples.
2. **MANUAL.md** — details on encrypted file format, compatibility, limitations.
3. **Best practices:**
   - Key: `chmod 600`, secure backup.
   - Loss of key = definitive loss of data.
   - Do not use `--encrypt` + `--checksum` if the checksum is compared later (the destination file is encrypted, checksum differs).

---

## Recommended Execution Order

| Step | Files | Dependency |
|-------|----------|-------------|
| 1 | `internal/crypto/crypto.go` | None |
| 2 | `internal/crypto/crypto_test.go` | Step 1 |
| 3 | `internal/filecopy.go` (Options + integration) | Step 1 |
| 4 | `cmd/fastcopy/main.go` (flags) | Step 3 |
| 5 | End-to-end integration tests | Steps 1-4 |
| 6 | Documentation | Step 5 |

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|-------|---------|-----------|
| Nonce reuse | Total security breach | Monotonic counter per chunk + unique base nonce per file |
| Partially encrypted file (crash) | Irrecoverable data | Write to temporary file, rename at the end (atomic write) |
| Performance degradation on huge files | Slowness vs pure copy | Mandatory benchmark; accept that sequential encryption is ~30-50% slower than zero-copy |
| Key leaked in memory after use | Exposure in core dump | Zero out the key `[]byte` with a loop after use; Go doesn't guarantee this via GC |
