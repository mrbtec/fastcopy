# Plan de Implementación de Criptografía para gocopy

Basado en el análisis del documento `OPCOES-CRIPTOGRAFIA.md` y la **arquitectura real del proyecto**, se ha estructurado el siguiente plan de acción. Se realizaron cambios significativos respecto a la versión anterior para corregir problemas técnicos y alinearse con el código existente.

---

## ⚠️ Problemas Identificados en el Plan Anterior

### 1. Error Crítico: Reutilización de Nonce en el Código de Referencia
El esbozo en `OPCOES-CRIPTOGRAFIA.md` usa el **mismo nonce para todos los chunks** dentro de un archivo. Esto es una **vulnerabilidad fatal** en AES-GCM: reutilizar un nonce con la misma clave permite al atacante recuperar la clave de autenticación y descifrar los datos. **Cada chunk debe tener su propio nonce único** (o usar un contador incremental derivado del nonce base).

### 2. Incompatibilidad con Zero-Copy
El motor de copia de fastcopy usa `copy_file_range` (zero-copy vía kernel en Linux) — los datos **nunca transitan por el espacio de usuario (userspace)**. La criptografía, por definición, necesita procesar los datos en userspace. Por lo tanto, el flujo cifrado **debe deshabilitar automáticamente el zero-copy** y usar el fallback (`fallbackCopy`) o una pipeline dedicada.

### 3. Incompatibilidad con Copia Concurrente por Chunks
El `concurrentCopy` en `chunk_copy.go` usa `WriteAt`/`SectionReader` con offsets absolutos. AES-GCM introduce un **overhead por chunk** (tag de 16 bytes + nonce), lo que altera los offsets de escritura en el destino en relación con la lectura en el origen. La copia concurrente con criptografía requeriría un mapeo de offsets complejo. **En la primera versión, la criptografía debe usar copia secuencial.**

### 4. Tamaño de Buffer Desalineado
`OPCOES-CRIPTOGRAFIA.md` sugiere chunks de 64KB, pero el proyecto usa buffers de **4MB** (`bufSize` en `filecopy.go`) vía `sync.Pool`. El paquete de criptografía debe reutilizar el `bufPool` existente para evitar la duplicación de asignaciones y la presión en el GC.

### 5. Límite de Tamaño del GCM
AES-GCM tiene un límite práctico de ~64GB por nonce (2³² bloques de 16 bytes). Para archivos más grandes, cada chunk debe ser tratado como un mensaje GCM independiente, con su propio nonce.

---

## Fase 1: Creación del Paquete `fastcopy/internal/crypto`

**Objetivo:** Primitivas criptográficas correctas y seguras.

### 1.1 Formato del Archivo Cifrado

```
[HEADER: 4 bytes magic "FCRY"]
[VERSION: 1 byte]
[BASE_NONCE: 12 bytes]
[CHUNK_0: nonce_counter(0) → ciphertext + GCM tag (16 bytes)]
[CHUNK_1: nonce_counter(1) → ciphertext + GCM tag (16 bytes)]
...
[CHUNK_N]
```

- **Magic bytes** permiten identificar si un archivo fue cifrado por fastcopy.
- **Nonce por chunk**: derivar nonce incrementando un contador sobre el `BASE_NONCE` (evita el error de reutilización).
- **Cada chunk** es un mensaje GCM autónomo (permite la detección de corrupción granular).

### 1.2 Funciones Principales

```go
// crypto.go
package crypto

// EncryptStream(reader io.Reader, writer io.Writer, key []byte) error
//   - Escribe header + base_nonce
//   - Lee chunks de 4MB del reader, cifra cada uno con nonce incrementado
//   - Escribe [ciphertext+tag] para cada chunk

// DecryptStream(reader io.Reader, writer io.Writer, key []byte) error
//   - Lee header + base_nonce
//   - Lee chunks cifrados (4MB + 16 bytes overhead), descifra con nonce incrementado
//   - Escribe plaintext en el writer

// GenerateKey() ([]byte, error)
//   - Genera 32 bytes criptográficamente seguros

// LoadKey(path string) ([]byte, error)
//   - Lee el archivo, valida el tamaño (== 32 bytes), retorna la clave
```

> **Nota:** Usar `io.Reader`/`io.Writer` en lugar de rutas permite la integración directa con la pipeline de copia existente (ej: `io.MultiWriter` para checksum + criptografía simultánea).

### 1.3 Pruebas (`crypto_test.go`)

| Caso de Prueba | Validación |
|---|---|
| Encrypt → Decrypt round-trip | Datos idénticos al original |
| Archivo vacío | No debe fallar (caso límite) |
| Archivo > 4MB (multi-chunk) | Chunks procesados correctamente |
| Bit flip en el ciphertext | `gcm.Open` retorna error (integridad GCM) |
| Clave incorrecta en el descifrado | Fallo en la autenticación |
| El nonce nunca se repite entre chunks | Verificar incremento monotónico |

---

## Fase 2: Integración con el Motor de Copia

**Objetivo:** Insertar la criptografía en la pipeline sin romper la arquitectura existente.

### 2.1 Modificaciones en `filecopy.go`

La función `CopyFile` es el punto de integración correcto. El flujo con criptografía será:

```
src → [lectura] → [cifrado/descifrado] → [escritura] → dst
```

Cuando `opts.Encrypt` o `opts.Decrypt` esté activo:
- **Omitir** `platformCopyFile` (zero-copy es incompatible).
- **Omitir** `concurrentCopy` (offsets desalineados por overhead GCM).
- Usar `crypto.EncryptStream` o `crypto.DecryptStream` con los descriptores de archivo directamente.
- Mantener compatibilidad con `--checksum`: usar `io.TeeReader` para alimentar SHA-256 **sobre los datos originales** (no sobre el ciphertext).

### 2.2 Modificaciones en `Options` (struct)

```go
type Options struct {
    // ... campos existentes ...
    EncryptKey []byte // Si no es nil, cifra durante la copia
    DecryptKey []byte // Si no es nil, descifra durante la copia
}
```

### 2.3 `incremental.go` — Sin Cambios

La lógica incremental compara **tamaño y mtime** del archivo en el destino. Como el archivo cifrado tendrá un tamaño diferente al original, el comportamiento incremental ya funcionará correctamente (detectará el cambio y volverá a copiar). No es necesario alterar la lógica.

### 2.4 Uso del `bufPool` Existente

El paquete `crypto` debe importar y usar `getBuf()`/`putBuf()` de `filecopy.go` (o exponer el pool vía función pública). Esto garantiza que no haya dos pools de 4MB compitiendo por memoria.

---

## Fase 3: Exposición en la CLI

### 3.1 Nuevas Flags en `cmd/fastcopy/main.go`

```go
encryptKey := flag.String("encrypt", "", "ruta al archivo de clave de 32 bytes para cifrado AES-256-GCM")
decryptKey := flag.String("decrypt", "", "ruta al archivo de clave de 32 bytes para descifrado AES-256-GCM")
genKey     := flag.String("gen-key", "", "genera una nueva clave aleatoria y la guarda en esta ruta, luego sale")
```

**Validaciones obligatorias:**
- `--encrypt` y `--decrypt` son mutuamente excluyentes.
- El archivo de clave debe existir y contener exactamente 32 bytes.
- Mostrar advertencia si los permisos del archivo de clave son más abiertos que `0600`.

### 3.2 Comando `--gen-key`

Agregar una subacción que genera una clave segura y sale:

```bash
fastcopy --gen-key /ruta/mi_clave.bin
# Genera 32 bytes con crypto/rand, guarda, define chmod 600
```

Esto elimina la dependencia de `openssl` para el usuario.

### 3.3 Feedback al Usuario

- En la salida estándar, mostrar `🔒 Cifrado activado (AES-256-GCM)` cuando esté activo.
- En el resumen final, mostrar el overhead estimado (tiempo adicional vs copia pura).

---

## Fase 4: Documentación

1. **README.md** — sección "Criptografía" con ejemplos rápidos.
2. **MANUAL.md** — detalles sobre el formato del archivo cifrado, compatibilidad, limitaciones.
3. **Buenas prácticas:**
   - Clave: `chmod 600`, respaldo seguro.
   - No perder la clave = pérdida definitiva de los datos.
   - No usar `--encrypt` + `--checksum` si el checksum se comparará después (el archivo destino está cifrado, el checksum difiere).

---

## Orden de Ejecución Recomendado

| Etapa | Archivos | Dependencia |
|-------|----------|-------------|
| 1 | `internal/crypto/crypto.go` | Ninguna |
| 2 | `internal/crypto/crypto_test.go` | Etapa 1 |
| 3 | `internal/filecopy.go` (Options + integración) | Etapa 1 |
| 4 | `cmd/fastcopy/main.go` (flags) | Etapa 3 |
| 5 | Pruebas de integración end-to-end | Etapas 1-4 |
| 6 | Documentación | Etapa 5 |

---

## Riesgos y Mitigaciones

| Risco | Impacto | Mitigación |
|-------|---------|-----------|
| Reutilización de Nonce | Ruptura total de la seguridad | Contador monotónico por chunk + nonce base único por archivo |
| Archivo parcialmente cifrado (crash) | Datos irrecuperables | Escribir en archivo temporal, renomar al final (escritura atómica) |
| Rendimiento degradado en archivos enormes | Lentitud vs copia pura | Benchmark obligatorio; aceptar que el cifrado secuencial es ~30-50% más lento que zero-copy |
| Clave filtrada en la memoria después del uso | Exposición en core dump | Poner a cero el `[]byte` de la clave con un bucle después del uso; Go no garantiza esto mediante el GC |
