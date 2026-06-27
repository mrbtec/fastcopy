# Encryption Options for the gocopy Project

This document describes possible approaches for **encrypting** and **decrypting** files within the **gocopy** project (implemented in Go). It serves as a reference for anyone who wants to extend the utility with encryption support while maintaining performance and compatibility with other modules.

---

## 1. Why add encryption?
- **Protection of sensitive data** when copying files between machines or storing backups.
- **Compliance** with security policies (e.g., GDPR, LGPD).
- **Integrity**: combine encryption with existing checksums (SHA-256) to detect alterations.

## 2. Native Go Libraries
Go already includes robust packages in the standard module (`crypto`). They are well-tested and require no external dependencies.

| Library | Algorithm | Typical Use | Comments |
|------------|-----------|------------|-------------|
| `crypto/aes` | AES-CBC, AES-GCM | Symmetric block encryption. | AES-GCM provides confidentiality + integrity (AEAD). |
| `crypto/cipher` | Block and stream interfaces | Build operation modes (CBC, CTR, GCM). | Necessary to combine with `aes`. |
| `crypto/sha256` | SHA-256 | Already used for checksums. | Can be used as HMAC for authentication. |
| `crypto/hmac` | HMAC-SHA256 | Message authentication. | Combine with a secret key to ensure integrity. |
| `crypto/rand` | Secure random number generator | Generation of IVs/nonces. | Always use `rand.Reader`. |
| `crypto/rsa` | RSA (OAEP, PKCS#1 v1.5) | Asymmetric encryption. | Ideal for key exchange, but slower. |
| `golang.org/x/crypto/chacha20poly1305` | ChaCha20-Poly1305 | High-performance AEAD. | Good choice when AES hardware is not available. |

## 3. Recommended Strategy (Symmetric)
1. **Generate a secret key** (32 bytes for AES-256) using `crypto/rand`.
2. **Derive a random nonce/IV** for each file (12 bytes for GCM).
3. **Encrypt** with `cipher.NewGCM(aes.NewCipher(key))`.
4. **Persist** the nonce at the beginning of the encrypted file (e.g., `[nonce][ciphertext]`).
5. **Decrypt** by reading the nonce, initializing GCM, and calling `Open`.

### Code Example (Sketch)
```go
package cryptoutil

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "io"
    "os"
)

// EncryptFile encrypts srcPath and writes the result to dstPath.
func EncryptFile(srcPath, dstPath string, key []byte) error {
    // 1. Open source file
    in, err := os.Open(srcPath)
    if err != nil { return err }
    defer in.Close()

    // 2. Create destination file
    out, err := os.Create(dstPath)
    if err != nil { return err }
    defer out.Close()

    // 3. Create AES-GCM cipher
    block, err := aes.NewCipher(key)
    if err != nil { return err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return err }

    // 4. Generate nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err = io.ReadFull(rand.Reader, nonce); err != nil { return err }
    // 5. Write nonce (must be read during decryption)
    if _, err = out.Write(nonce); err != nil { return err }

    // 6. Stream encryption (block reading)
    buf := make([]byte, 64*1024) // 64 KB
    for {
        n, rerr := in.Read(buf)
        if n > 0 {
            ciphertext := gcm.Seal(nil, nonce, buf[:n], nil)
            if _, err = out.Write(ciphertext); err != nil { return err }
        }
        if rerr == io.EOF { break }
        if rerr != nil { return rerr }
    }
    return nil
}

// DecryptFile reverses the process.
func DecryptFile(srcPath, dstPath string, key []byte) error {
    in, err := os.Open(srcPath)
    if err != nil { return err }
    defer in.Close()

    out, err := os.Create(dstPath)
    if err != nil { return err }
    defer out.Close()

    block, err := aes.NewCipher(key)
    if err != nil { return err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return err }

    // 1. Read nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err = io.ReadFull(in, nonce); err != nil { return err }

    // 2. Decrypt stream
    buf := make([]byte, 64*1024+gcm.Overhead())
    for {
        n, rerr := in.Read(buf)
        if n > 0 {
            plaintext, err := gcm.Open(nil, nonce, buf[:n], nil)
            if err != nil { return err }
            if _, err = out.Write(plaintext); err != nil { return err }
        }
        if rerr == io.EOF { break }
        if rerr != nil { return rerr }
    }
    return nil
}
```

> **Note:** The code above is a starting point. In production, consider:
> - **Additional Authentication** (HMAC) to protect against modifications.
> - **Key Rotation**.
> - **Secure Key Storage** (e.g., `keyring`, environment variables, or a vault).

## 4. Alternative Strategy (Asymmetric)
- Use RSA-OAEP to encrypt the symmetric key (AES) and store it in the file header.
- Benefit: the key can be distributed without prior sharing.
- Disadvantage: higher computational cost; suitable only for small files or key exchange.

## 5. Integration with the Existing Project
1. **Add new package** `fastcopy/internal/crypto` containing the functions above.
2. **Expose flags** in the main binary (`fastcopy` and `fastcopy-gui`):
   - `--encrypt <keyfile>` – encrypts before copying.
   - `--decrypt <keyfile>` – decrypts after copying.
3. **Update the CLI** (`cmd/fastcopy/main.go`) to accept the new options and call the package.
4. **Tests**: create unit tests in `fastcopy/internal/crypto/crypto_test.go` using temporary files.
5. **Documentation**: update `README.md` and `MANUAL.md` with usage examples.

## 6. Performance Considerations
- **AES-GCM** has performance close to pure copy (≈ 1 GB/s on modern CPUs).
- **ChaCha20-Poly1305** can be faster on CPUs without AES instructions.
- **Chunking**: reuse the logic from `chunk_copy.go` to process files in blocks, avoiding excessive memory allocation.

## 7. Security
- Never reuse the same nonce/IV with the same key.
- Use `crypto/rand` to generate keys/nonces.
- Protect the key at rest (e.g., `chmod 600` on the key file).
- Consider using **libsodium** via `golang.org/x/crypto/nacl` if you need high-level primitives.

---

### Suggested Next Steps
1. Create the `fastcopy/internal/crypto` package with the example functions.
2. Implement command-line flags.
3. Write integration tests that copy a file, encrypting it, and then verifying integrity.
4. Update project documentation.

With these options, **gocopy** can offer strong encryption without sacrificing copy speed.
