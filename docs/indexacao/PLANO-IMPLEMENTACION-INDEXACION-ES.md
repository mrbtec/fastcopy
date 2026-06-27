# Plan de Implementación de Indexación para gocopy

Basado en el análisis de `OPCOES-INDEXACAO.md` y la **arquitectura real del proyecto**, este plan ha sido revisado para corregir problemas técnicos y alinearse con el código existente.

---

## ⚠️ Problemas Identificados y Corregidos en la Implementación

### 1. El Código No Compilaba — 5 Errores de Sintaxis
La implementación original tenía **dos arquitecturas mezcladas** (gob + SQLite) que causaban errores fatales de compilación:

| Archivo | Error | Acción |
|---------|------|------|
| `core.go` | `package index` duplicado (línea 1 y 2); el tipo `Query` confligía con `index.go` | **Eliminado** — tipos ya definidos en `index.go` |
| `storage.go` | Dos implementaciones concatenadas: gob (L1-55) + SQLite (L56-97) | **Reescrito** — se mantuvo solo la versión gob |
| `suffix_trie.go` | `package index` duplicado (línea 1 y 2) | **Eliminado** — placeholder vacío |

### 2. Dependencia de SQLite Inexistente en `go.mod`
4 archivos (`file_index.go`, `content_index.go`, `hash_index.go`, `meta_index.go`) importaban `database/sql` y `github.com/mattn/go-sqlite3`, que **no existen en el `go.mod`** del proyecto. Acción: todos eliminados.

### 3. Archivos Placeholder Sin Funcionalidad
`prefix_trie.go` y `suffix_trie.go` eran structs vacías sin métodos. Acción: eliminados.

### 4. `main.go` Referenciaba `idx.NewStorage` Inexistente
La CLI llamaba a `idx.NewStorage()` (API de la implementación de SQLite eliminada). Acción: reescrito para usar `idx.Save()`/`idx.Load()` directamente.

---

## Arquitectura Final (Validada — Compila ✅)

```
fastcopy/internal/index/
├── index.go      # Tipos Entry, Index, Query + FindDuplicates
├── builder.go    # BuildFromScan (reutiliza scanner + checksum existentes)
├── search.go     # Search (glob, size, hash) + LookupByPath
└── storage.go    # Save/Load vía encoding/gob
```

**Cero dependencias externas.** Persistencia vía `encoding/gob` (stdlib).

---

## Tipos Centrales (`index.go`)

```go
// Entry representa un archivo en el índice.
type Entry struct {
    Path    string     // camino relativo a la raíz indexada
    Size    int64
    ModTime time.Time
    Mode    uint32     // permisos
    Hash    string     // SHA-256 hex (vacío si no fue calculado)
    IsDir   bool
}

// Index es la estructura principal del índice.
type Index struct {
    Version   int               // versión del formato
    RootPath  string            // directorio raíz indexado
    CreatedAt time.Time
    Entries   []Entry           // lista ordenada por Path
    HashMap   map[string][]int  // hash → índices en Entries (para dedup)
}

// Query describe los criterios de búsqueda.
type Query struct {
    Name       string // patrón glob para nombre/camino
    MinSize    int64
    MaxSize    int64  // 0 = sin límite
    Hash       string
    Duplicates bool   // si es true, retorna solo duplicados
}
```

---

## Construcción del Índice (`builder.go`)

Reutiliza `ScanDirAsync` y `ChecksumFile` existentes vía adaptador:

```go
func BuildFromScan(ctx context.Context, rootPath string, computeHash bool) (*Index, error)
```

- Usa `ScanDirAsync` con `DryRun: true` (no crea directorios en el destino)
- El hash es **opt-in** vía el parámetro `computeHash`
- Popula `HashMap` durante la construcción para búsqueda O(1) por hash

---

## Búsqueda (`search.go`)

| Operación | Implementación | Complejidad |
|----------|---------------|--------------|
| Búsqueda por nombre (glob) | `filepath.Match(pattern, entry.Path)` | O(n) |
| Búsqueda por hash | `idx.HashMap[hash]` (búsqueda directa) | O(1) |
| Búsqueda por tamaño | Filtro lineal con min/max | O(n) |
| Listar duplicados | Iterar `HashMap`, filtrar len > 1 | O(n) |

También incluye `LookupByPath(relPath string) (Entry, bool)` para integración futura con `NeedsCopy`.

---

## Persistencia (`storage.go`)

```go
func (idx *Index) Save(path string) error   // serializa vía gob
func Load(path string) (*Index, error)       // deserializa + valida versión
```

- Formato: binario gob (eficiente, sin dependencias externas)
- Validación de versión en `Load` para compatibilidad futura

---

## Integración con la CLI (`cmd/fastcopy/main.go`)

### Flags Implementadas

```go
--index-build    // construye el índice para el directorio fuente
--index-search   // busca término/patrón en el índice
--index-path     // camino al archivo de índice (default: fastcopy.idx)
--index-hash     // calcula SHA-256 durante la construcción
--index-dupes    // lista archivos duplicados
```

### Ejemplos de Uso

```bash
# Construir índice rápido (solo metadatos)
fastcopy --index-build /data/backup --index-path backup.idx

# Construir índice con hashes (más lento, permite dedup)
fastcopy --index-build /data/backup --index-hash --index-path backup.idx

# Buscar archivos por patrón glob
fastcopy --index-search "*.log" --index-path backup.idx

# Listar duplicados
fastcopy --index-dupes --index-path backup.idx
```

---

## Integración Futura: Copia Incremental por Hash

El índice puede sustituir la heurística `size+mtime` de `NeedsCopy` por comparación de hash:

```go
// En incremental.go (mejora futura)
func NeedsCopyWithIndex(srcPath string, srcInfo os.FileInfo, idx *index.Index) bool {
    entry, found := idx.LookupByPath(srcPath)
    if !found { return true }
    currentHash, _ := ChecksumFile(srcPath)
    return currentHash != entry.Hash
}
```

---

## Pruebas Pendientes

| Caso de Prueba | Validación |
|---|---|
| Build + Save + Load round-trip | Índice preservado correctamente |
| Búsqueda por glob (`*.go`) | Retorna solo archivos .go |
| Búsqueda por hash | Retorna el camino correcto |
| Detección de duplicados | Encuentra archivos con contenido idéntico |
| Directorio vacío | No falla (caso borde) |
| Archivo de índice corrupto | `Load` retorna error |
| Índice con 100K+ entradas | Performance < 500ms para build + search |

> **Nota:** Las pruebas unitarias (`index_test.go`) aún no han sido implementadas.

---

## Riesgos y Mitigaciones

| Riesgo | Impacto | Mitigación |
|-------|---------|-----------|
| Índice grande en memoria (> 5M archivos) | Alto uso de RAM | Limitar la estructura `Entry` al mínimo; considerar migrar a `bbolt` si es necesario |
| Hash lento en HDD con muchos archivos | Construcción demorada | La flag `--index-hash` es opt-in; sin ella la construcción es rápida (solo stat) |
| Índice desactualizado | Resultados incorrectos | Almacenar `CreatedAt`; mostrar aviso si el índice tiene > 24h |
| Formato gob falla entre versiones de Go | Índice ilegible | Campo `Version` en `Index`; validar en la lectura |
