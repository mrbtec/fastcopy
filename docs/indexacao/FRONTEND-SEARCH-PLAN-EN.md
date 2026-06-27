# Frontend Search Implementation Plan (fastcopy‑gui)

## 1. Overview
The **fastcopy‑gui** currently serves only as a frontend for the copy operation. There is no search form, and the search backend (`internal/index/search.go`) is coupled with the CLI. For the search to be **as fast as the copy operation itself**, we need to:

1. **Improve backend performance** (O(1) and O(log N) queries, reduction of work‑loops).
2. **Expose the backend to the GUI** through a simple API.
3. **Implement a search form** in the GUI that executes queries asynchronously and displays paginated/virtualized results.

---

## 2. Identified Problems in the Current Backend
| Problem | Location | Impact |
|----------|-------------|---------|
| General linear search `O(n)` | `search.go` (function `Search`) | Traverses the entire `Entries` list – about 1M iterations for 1M files in any name search. |
| Linear `LookupByPath` | `search.go` (function `LookupByPath`) | Exact path search in `O(n)` time. |
| Missing `PathMap` | `index.go` – only `HashMap` exists | Prevents direct `O(1)` access to an `Entry` by path. |
| Unlimited results | `search.go` – no `Limit/Offset` parameters | Can generate thousands of lines in the GUI, overloading the UI and memory allocation. |
| Builder does not sort `Entries`| `builder.go` | The list remains in the OS discovery order, making binary search (`O(log N)`) for prefixes impossible. |
| GUI Isolation | `cmd/fastcopy-gui/main.go` | The GUI has no access or tabs related to indexing. |

---

## 3. Backend Improvements
### 3.1 Data Structure
1. **Add `PathMap` and ensure sorting:**
   ```go
   type Index struct {
       Version   int
       RootPath  string
       CreatedAt time.Time
       Entries   []Entry            // will be sorted by Path
       HashMap   map[string][]int   // hash → indices in Entries
       PathMap   map[string]int     // relative path → index in Entries (O(1) access)
   }
   ```
2. **Sorting in the Builder:**
   At the end of the `BuildFromScan` function (in `builder.go`), collect all entries, sort them with `sort.Slice` by `Path`, and then scan the array once to populate `PathMap` and `HashMap`.

### 3.2 Optimized Search API (`Query`)
Add pagination support to the existing Query struct:
```go
type Query struct {
    Name       string 
    MinSize    int64
    MaxSize    int64
    Hash       string
    Duplicates bool
    Limit      int   // maximum number of results (0 = no limit)
    Offset     int   // page start
}
```

**Search Strategies in `search.go`:**
- **Hash Search (Exact):** Use `HashMap`. Complexity `O(1)`.
- **Path Search (Exact):** Use `PathMap`. Complexity `O(1)`.
- **Path Prefix Search (e.g., `folder/*`):** Since `Entries` will be alphabetically sorted by `Path`, use binary search (`sort.Search`) to find the first element. Then, iterate sequentially until the prefix no longer matches. Complexity `O(log N + K)`.
- **Glob/Suffix Search (e.g., `*.go`):** Remains `O(N)`, but with `Limit/Offset` stopping early.

---

## 4. GUI Integration (Fyne)
### 4.1 Application State
Avoid global variables. In the GUI (`main.go`), create a state structure to keep the loaded index:

```go
type AppState struct {
    currentIndex *index.Index
    // mutex if necessary for asynchronous reloading
}
```

### 4.2 New Layout: Tab System (Tabs)
The interface should be converted to use `container.NewAppTabs`:
- **Tab 1: Copier** (current interface)
- **Tab 2: Search & Index** (new interface)

### 4.3 Search Form in Tab 2
1. **Controls (Top):**
   - **Load Index** button (opens `.idx` file selector).
   - `Entry` for search term.
   - `Select` for search type (Name/Prefix, Exact Path, Hash).
   - **Search** button.
2. **Results Display (Center):**
   - Use Fyne's native virtualization widget: `widget.NewList`.
   - `widget.NewList` instantiates only the items visible on screen and recycles them upon scrolling, ensuring perfect performance (60 FPS) even with 100,000 results. It does not attempt to load the UI with thousands of individual widgets.
3. **Controls (Bottom):**
   - Labels showing: "Results found: X", "Search time: Y ms".

### 4.4 Asynchronicity
- When the user clicks "Search", the `idx.Search(query)` call must run in a **goroutine**.
- The UI can show a `widget.NewProgressBarInfinite()` while searching.
- Upon completion, the goroutine updates the list and hides the progress bar (data rendering in the List is thread-safe if the base data is replaced before calling `list.Refresh()`).

---

## 5. Execution Plan

| Step | Description | Impacted Files |
|-------|-----------|--------------------|
| 1 | Add `PathMap` to `Index` and ensure sorting (`sort.Slice` and repopulate maps) | `internal/index/index.go`, `builder.go` |
| 2 | Refactor `Search` to use `O(1)` and `O(log N)` searches (Hash, Exact Path, Prefix) | `search.go` |
| 3 | Add support for `Limit` and `Offset` in `Query` and `Search` | `index.go`, `search.go` |
| 4 | Update CLI to compile with new `Query` fields (if applicable) | `cmd/fastcopy/main.go` |
| 5 | Migrate `fastcopy-gui` interface to `AppTabs` (Copy / Search) | `cmd/fastcopy-gui/main.go` |
| 6 | Implement index loading (`Load`) and asynchronous search form in GUI | `cmd/fastcopy-gui/main.go` |
| 7 | Render results using `widget.NewList` (virtualized) | `cmd/fastcopy-gui/main.go` |

---

## 6. Risks & Mitigations
| Risk | Mitigation |
|-------|-----------|
| **Large Index (RAM)** | `PathMap` adds memory overhead. For 1 million files, a `map[string]int` will occupy approx 20-30 MB. This is very acceptable given the performance gain. |
| **UI Blocking during Load** | `idx.Load` takes some time due to `gob`. Loading must be done in a goroutine with a spinner. |
| **Concurrency Errors in Fyne List** | Replace the base data slice (e.g., `searchResults = newResults`) and only then call `list.Refresh()`, avoiding mutating individual elements during list rendering. |
