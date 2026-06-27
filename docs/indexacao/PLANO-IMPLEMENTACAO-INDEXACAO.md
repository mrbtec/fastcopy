# Plano de Implementação de Indexação para gocopy

Com base na análise do `OPCOES-INDEXACAO.md` e da **arquitetura real do projeto**, este plano foi revisado para corrigir problemas técnicos e alinhar com o código existente.

---

## ⚠️ Problemas Identificados e Corrigidos na Implementação

### 1. Código Não Compilava — 5 Erros de Sintaxe
A implementação original tinha **duas arquiteturas misturadas** (gob + SQLite) que causavam erros fatais de compilação:

| Arquivo | Erro | Ação |
|---------|------|------|
| `core.go` | `package index` duplicado (linha 1 e 2); tipo `Query` conflita com `index.go` | **Removido** — tipos já definidos em `index.go` |
| `storage.go` | Duas implementações concatenadas: gob (L1-55) + SQLite (L56-97) | **Reescrito** — mantida apenas a versão gob |
| `suffix_trie.go` | `package index` duplicado (linha 1 e 2) | **Removido** — placeholder vazio |

### 2. Dependência SQLite Inexistente no `go.mod`
4 arquivos (`file_index.go`, `content_index.go`, `hash_index.go`, `meta_index.go`) importavam `database/sql` e `github.com/mattn/go-sqlite3`, que **não existem no `go.mod`** do projeto. Ação: todos removidos.

### 3. Arquivos Placeholder Sem Funcionalidade
`prefix_trie.go` e `suffix_trie.go` eram structs vazias sem métodos. Ação: removidos.

### 4. `main.go` Referenciava `idx.NewStorage` Inexistente
A CLI chamava `idx.NewStorage()` (API da implementação SQLite removida). Ação: reescrito para usar `idx.Save()`/`idx.Load()` diretamente.

---

## Arquitetura Final (Validada — Compila ✅)

```
fastcopy/internal/index/
├── index.go      # Tipos Entry, Index, Query + FindDuplicates
├── builder.go    # BuildFromScan (reutiliza scanner + checksum existentes)
├── search.go     # Search (glob, size, hash) + LookupByPath
└── storage.go    # Save/Load via encoding/gob
```

**Zero dependências externas.** Persistência via `encoding/gob` (stdlib).

---

## Tipos Centrais (`index.go`)

```go
// Entry representa um arquivo no índice.
type Entry struct {
    Path    string     // caminho relativo à raiz indexada
    Size    int64
    ModTime time.Time
    Mode    uint32     // permissões
    Hash    string     // SHA-256 hex (vazio se não calculado)
    IsDir   bool
}

// Index é a estrutura principal do índice.
type Index struct {
    Version   int               // versão do formato
    RootPath  string            // diretório raiz indexado
    CreatedAt time.Time
    Entries   []Entry           // lista ordenada por Path
    HashMap   map[string][]int  // hash → índices em Entries (para dedup)
}

// Query descreve critérios de busca.
type Query struct {
    Name       string // padrão glob para nome/caminho
    MinSize    int64
    MaxSize    int64  // 0 = sem limite
    Hash       string
    Duplicates bool   // se true, retorna apenas duplicatas
}
```

---

## Construção do Índice (`builder.go`)

Reutiliza `ScanDirAsync` e `ChecksumFile` existentes via adaptador:

```go
func BuildFromScan(ctx context.Context, rootPath string, computeHash bool) (*Index, error)
```

- Usa `ScanDirAsync` com `DryRun: true` (não cria diretórios no destino)
- Hash é **opt-in** via parâmetro `computeHash`
- Popula `HashMap` durante a construção para lookup O(1) por hash

---

## Busca (`search.go`)

| Operação | Implementação | Complexidade |
|----------|---------------|--------------|
| Busca por nome (glob) | `filepath.Match(pattern, entry.Path)` | O(n) |
| Busca por hash | `idx.HashMap[hash]` (lookup direto) | O(1) |
| Busca por tamanho | Filtro linear com min/max | O(n) |
| Listar duplicatas | Iterar `HashMap`, filtrar len > 1 | O(n) |

Também inclui `LookupByPath(relPath string) (Entry, bool)` para integração futura com `NeedsCopy`.

---

## Persistência (`storage.go`)

```go
func (idx *Index) Save(path string) error   // serializa via gob
func Load(path string) (*Index, error)       // deserializa + valida versão
```

- Formato: binário gob (eficiente, sem dependências externas)
- Validação de versão no `Load` para compatibilidade futura

---

## Integração com a CLI (`cmd/fastcopy/main.go`)

### Flags Implementadas

```go
--index-build    // constrói índice para o diretório fonte
--index-search   // busca termo/padrão no índice
--index-path     // caminho do arquivo de índice (default: fastcopy.idx)
--index-hash     // calcula SHA-256 durante construção
--index-dupes    // lista arquivos duplicados
```

### Exemplos de Uso

```bash
# Construir índice rápido (apenas metadados)
fastcopy --index-build /data/backup --index-path backup.idx

# Construir índice com hashes (mais lento, permite dedup)
fastcopy --index-build /data/backup --index-hash --index-path backup.idx

# Buscar arquivos por padrão glob
fastcopy --index-search "*.log" --index-path backup.idx

# Listar duplicatas
fastcopy --index-dupes --index-path backup.idx
```

---

## Integração Futura: Cópia Incremental por Hash

O índice pode substituir a heurística `size+mtime` do `NeedsCopy` por comparação de hash:

```go
// Em incremental.go (futura melhoria)
func NeedsCopyWithIndex(srcPath string, srcInfo os.FileInfo, idx *index.Index) bool {
    entry, found := idx.LookupByPath(srcPath)
    if !found { return true }
    currentHash, _ := ChecksumFile(srcPath)
    return currentHash != entry.Hash
}
```

---

## Testes Pendentes

| Caso de Teste | Validação |
|---|---|
| Build + Save + Load round-trip | Índice preservado corretamente |
| Busca por glob (`*.go`) | Retorna apenas arquivos .go |
| Busca por hash | Retorna path correto |
| Detecção de duplicatas | Encontra arquivos com conteúdo idêntico |
| Diretório vazio | Não falha (edge case) |
| Arquivo de índice corrompido | `Load` retorna erro |
| Índice com 100K+ entradas | Performance < 500ms para build + search |

> **Nota:** Testes unitários (`index_test.go`) ainda não foram implementados.

---

## Riscos e Mitigações

| Risco | Impacto | Mitigação |
|-------|---------|-----------|
| Índice grande em memória (> 5M arquivos) | Alto uso de RAM | Limitar a estrutura `Entry` ao mínimo; considerar migrar para `bbolt` se necessário |
| Hash lento em HDD com muitos arquivos | Build demorado | Flag `--index-hash` é opt-in; sem ela o build é rápido (apenas stat) |
| Índice desatualizado | Resultados incorretos | Armazenar `CreatedAt`; exibir aviso se índice > 24h |
| Formato gob quebra entre versões do Go | Índice ilegível | Campo `Version` no `Index`; validar na leitura |
