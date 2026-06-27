# Plano de Implementação da Busca no Frontend (fastcopy‑gui)

## 1. Visão Geral
A GUI **fastcopy‑gui** atualmente serve apenas como um frontend para a operação de cópia. Não há nenhum formulário de busca e o backend de busca (`internal/index/search.go`) está acoplado à CLI. Para que a busca seja **tão rápida quanto a própria operação de cópia**, precisamos:

1. **Melhorar a performance do backend** (consultas O(1) e O(log N), redução de work‑loops).
2. **Expor o backend para a GUI** através de uma API simples.
3. **Implementar um formulário de busca** na GUI que execute consultas de forma assíncrona e exiba resultados paginados/virtualizados.

---

## 2. Problemas Identificados no Backend Atual
| Problema | Localização | Impacto |
|----------|-------------|---------|
| Busca linear `O(n)` geral | `search.go` (função `Search`) | Percorre toda a lista de `Entries` – cerca de 1 M de iterações para 1 M de arquivos em qualquer busca de nome. |
| `LookupByPath` linear | `search.go` (função `LookupByPath`) | Busca por caminho exato em tempo `O(n)`. |
| Falta de `PathMap` | `index.go` – existe apenas `HashMap` | Impossibilita acesso direto `O(1)` a um `Entry` por caminho. |
| Resultados não limitados | `search.go` – nenhum parâmetro `Limit/Offset` | Pode gerar milhares de linhas na GUI, sobrecarregando a UI e alocação de memória. |
| Builder não ordena `Entries`| `builder.go` | A lista fica na ordem de descoberta do SO, inviabilizando busca binária (`O(log N)`) para prefixos. |
| Isolamento da GUI | `cmd/fastcopy-gui/main.go` | A GUI não tem acesso ou abas relacionadas à indexação. |

---

## 3. Melhorias no Backend
### 3.1 Estrutura de Dados
1. **Adicionar `PathMap` e garantir ordenação:**
   ```go
   type Index struct {
       Version   int
       RootPath  string
       CreatedAt time.Time
       Entries   []Entry            // será ordenada por Path
       HashMap   map[string][]int   // hash → índices em Entries
       PathMap   map[string]int     // caminho relativo → índice em Entries (acesso O(1))
   }
   ```
2. **Ordenação no Builder:**
   No final da função `BuildFromScan` (em `builder.go`), coletar todas as entradas, ordená-las com `sort.Slice` pelo `Path`, e em seguida varrer o array uma única vez para popular `PathMap` e `HashMap`.

### 3.2 API de Busca Otimizada (`Query`)
Adicionar suporte a paginação na struct de Query existente:
```go
type Query struct {
    Name       string 
    MinSize    int64
    MaxSize    int64
    Hash       string
    Duplicates bool
    Limit      int   // número máximo de resultados (0 = sem limite)
    Offset     int   // início da página
}
```

**Estratégias de Busca em `search.go`:**
- **Busca por Hash (Exato):** Usar o `HashMap`. Complexidade `O(1)`.
- **Busca por Path (Exato):** Usar o `PathMap`. Complexidade `O(1)`.
- **Busca por Prefixo de Path (ex: `pasta/*`):** Como `Entries` estará ordenado alfabeticamente por `Path`, usar busca binária (`sort.Search`) para achar o primeiro elemento. Depois, iterar sequencialmente até o prefixo não dar mais match. Complexidade `O(log N + K)`.
- **Busca por Glob/Sufixo (ex: `*.go`):** Continua `O(N)`, mas com `Limit/Offset` parando antecipadamente.

---

## 4. Integração na GUI (Fyne)
### 4.1 Estado da Aplicação
Evitar variáveis globais. Na GUI (`main.go`), criar uma estrutura de estado para manter o índice carregado:

```go
type AppState struct {
    currentIndex *index.Index
    // mutex se necessário para recarregamento assíncrono
}
```

### 4.2 Novo Layout: Sistema de Abas (Tabs)
A interface deve ser convertida para usar `container.NewAppTabs`:
- **Aba 1: Copiador** (interface atual)
- **Aba 2: Busca & Índice** (nova interface)

### 4.3 Formulário de Busca na Aba 2
1. **Controles (Topo):**
   - Botão **Carregar Índice** (abre seletor de arquivo `.idx`).
   - `Entry` para termo de busca.
   - `Select` para tipo de busca (Nome/Prefixo, Caminho Exato, Hash).
   - Botão **Buscar**.
2. **Exibição de Resultados (Centro):**
   - Utilizar o widget nativo de virtualização do Fyne: `widget.NewList`.
   - O `widget.NewList` instancia apenas os itens visíveis na tela e os recicla ao rolar, garantindo performance perfeita (60 FPS) mesmo se houverem 100.000 resultados. Não tenta carregar a UI com milhares de widgets individuais.
3. **Controles (Rodapé):**
   - Labels mostrando: "Resultados encontrados: X", "Tempo de busca: Y ms".

### 4.4 Assincronicidade
- Quando o usuário clicar em "Buscar", a chamada `idx.Search(query)` deve rodar em uma **goroutine**.
- A UI pode mostrar um `widget.NewProgressBarInfinite()` enquanto busca.
- Ao concluir, a goroutine atualiza a lista e oculta a barra de progresso (a renderização de dados no List é thread-safe se os dados base forem substituídos antes de chamar `list.Refresh()`).

---

## 5. Plano de Execução

| Etapa | Descrição | Arquivos Impactados |
|-------|-----------|--------------------|
| 1 | Adicionar `PathMap` ao `Index` e garantir ordenação (`sort.Slice` e repopular maps) | `internal/index/index.go`, `builder.go` |
| 2 | Refatorar `Search` para usar buscas `O(1)` e `O(log N)` (Hash, Path exato, Prefixo) | `search.go` |
| 3 | Adicionar suporte a `Limit` e `Offset` em `Query` e `Search` | `index.go`, `search.go` |
| 4 | Atualizar CLI para compilar com os novos campos da `Query` (se aplicável) | `cmd/fastcopy/main.go` |
| 5 | Migrar interface do `fastcopy-gui` para `AppTabs` (Cópia / Busca) | `cmd/fastcopy-gui/main.go` |
| 6 | Implementar carregamento do índice (`Load`) e formulário de busca assíncrono na GUI | `cmd/fastcopy-gui/main.go` |
| 7 | Renderizar resultados usando `widget.NewList` (virtualizado) | `cmd/fastcopy-gui/main.go` |

---

## 6. Riscos & Mitigações
| Risco | Mitigação |
|-------|-----------|
| **Índice Grande (RAM)** | O `PathMap` adiciona overhead de memória. Para 1 milhão de arquivos, um `map[string]int` ocupará aprox 20-30 MB. É muito aceitável dado o ganho de performance. |
| **Bloqueio da UI ao Carregar** | `idx.Load` demora um pouco por causa do `gob`. O carregamento deve ser feito em goroutine com spinner. |
| **Erros de Concorrência na Lista Fyne** | Substituir o slice de dados base (ex: `searchResults = newResults`) e só então chamar `list.Refresh()`, evitando mutar os elementos individuais durante o render da lista. |
