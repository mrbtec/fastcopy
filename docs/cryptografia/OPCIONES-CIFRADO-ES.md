# Opciones de Criptografía para el Proyecto gocopy

Este documento describe posibles enfoques para **cifrar** y **descifrar** archivos dentro del proyecto **gocopy** (implementado en Go). Sirve como referencia para quien desee extender la utilidad con soporte de cifrado, manteniendo el rendimiento y la compatibilidad con los demás módulos.

---

## 1. ¿Por qué añadir cifrado?
- **Protección de datos sensibles** al copiar archivos entre máquinas o almacenar respaldos.
- **Cumplimiento** con políticas de seguridad (ej.: GDPR, LGPD).
- **Integridad**: combinar el cifrado con los checksums ya existentes (SHA-256) para detectar alteraciones.

## 2. Librerías nativas de Go
Go ya incluye paquetes robustos en el módulo estándar (`crypto`). Están bien probados y no requieren dependencias externas.

| Librería | Algoritmo | Uso típico | Comentarios |
|------------|-----------|------------|-------------|
| `crypto/aes` | AES-CBC, AES-GCM | Cifrado simétrico de bloques. | AES-GCM proporciona confidencialidad + integridad (AEAD). |
| `crypto/cipher` | Interfaces de bloque y stream | Construir modos de operación (CBC, CTR, GCM). | Necesario para combinar con `aes`. |
| `crypto/sha256` | SHA-256 | Ya usado para checksums. | Puede usarse como HMAC para autenticación. |
| `crypto/hmac` | HMAC-SHA256 | Autenticación de mensajes. | Combine con una clave secreta para garantizar la integridad. |
| `crypto/rand` | Generador de números aleatorios seguros | Generación de IVs/nonce. | Siempre use `rand.Reader`. |
| `crypto/rsa` | RSA (OAEP, PKCS#1 v1.5) | Cifrado asimétrico. | Ideal para el intercambio de claves, pero más lento. |
| `golang.org/x/crypto/chacha20poly1305` | ChaCha20-Poly1305 | AEAD de alto rendimiento. | Buena elección cuando el hardware AES no está disponible. |

## 3. Estrategia recomendada (simétrica)
1. **Generar una clave secreta** (32 bytes para AES-256) usando `crypto/rand`.
2. **Derivar un nonce/IV** aleatorio para cada archivo (12 bytes para GCM).
3. **Cifrar** con `cipher.NewGCM(aes.NewCipher(key))`.
4. **Persistir** el nonce al inicio del archivo cifrado (ej.: `[nonce][ciphertext]`).
5. **Descifrar** leyendo el nonce, inicializando el GCM y llamando a `Open`.

### Ejemplo de código (esbozo)
```go
package cryptoutil

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "io"
    "os"
)

// EncryptFile cifra srcPath y escribe el resultado en dstPath.
func EncryptFile(srcPath, dstPath string, key []byte) error {
    // 1. Abrir archivo de origen
    in, err := os.Open(srcPath)
    if err != nil { return err }
    defer in.Close()

    // 2. Crear archivo de destino
    out, err := os.Create(dstPath)
    if err != nil { return err }
    defer out.Close()

    // 3. Crear cifrado AES-GCM
    block, err := aes.NewCipher(key)
    if err != nil { return err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return err }

    // 4. Generar nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err = io.ReadFull(rand.Reader, nonce); err != nil { return err }
    // 5. Escribir nonce (debe leerse en el descifrado)
    if _, err = out.Write(nonce); err != nil { return err }

    // 6. Cifrado en stream (lectura en bloques)
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

// DecryptFile revierte el proceso.
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

    // 1. Leer nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err = io.ReadFull(in, nonce); err != nil { return err }

    // 2. Descifrar stream
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

> **Nota:** El código anterior es un punto de partida. En producción, considere:
> - **Autenticación** adicional (HMAC) para proteger contra modificaciones.
> - **Rotación de claves**.
> - **Almacenamiento seguro** de la clave (ej.: `keyring`, variables de entorno, o una bóveda).

## 4. Estrategia alternativa (asimétrica)
- Use RSA-OAEP para cifrar la clave simétrica (AES) y almacenarla en el encabezado del archivo.
- Beneficio: la clave puede distribuirse sin intercambio previo.
- Desventaja: costo computacional mayor; adecuado solo para archivos pequeños o intercambio de claves.

## 5. Integración con el proyecto existente
1. **Añadir nuevo paquete** `fastcopy/internal/crypto` conteniendo las funciones anteriores.
2. **Exponer flags** en el binario principal (`fastcopy` y `fastcopy-gui`):
   - `--encrypt <keyfile>` – cifra antes de copiar.
   - `--decrypt <keyfile>` – descifra después de copiar.
3. **Actualizar la CLI** (`cmd/fastcopy/main.go`) para aceptar las nuevas opciones y llamar al paquete.
4. **Pruebas**: crear pruebas unitarias en `fastcopy/internal/crypto/crypto_test.go` usando archivos temporales.
5. **Documentación**: actualizar `README.md` y `MANUAL.md` con ejemplos de uso.

## 6. Consideraciones de rendimiento
- **AES-GCM** tiene un rendimiento cercano al de la copia pura (≈ 1 GB/s en CPUs modernas).
- **ChaCha20-Poly1305** puede ser más rápido en CPUs sin instrucciones AES.
- **Chunking**: reutilizar la lógica de `chunk_copy.go` para procesar archivos en bloques, evitando la asignación excesiva de memoria.

## 7. Seguridad
- Nunca reutilice el mismo nonce/IV con la misma clave.
- Use `crypto/rand` para generar claves/nonce.
- Proteja la clave en reposo (ej.: `chmod 600` en el archivo de clave).
- Considere usar **libsodium** vía `golang.org/x/crypto/nacl` si necesita primitivas de alto nivel.

---

### Próximos pasos sugeridos
1. Crear el paquete `fastcopy/internal/crypto` con las funciones de ejemplo.
2. Implementar flags de línea de comandos.
3. Escribir pruebas de integración que copien un archivo, cifrándolo y luego verificando la integridad.
4. Actualizar la documentación del proyecto.

Con estas opciones, **gocopy** puede ofrecer un cifrado fuerte sin sacrificar la velocidad de la copia.
