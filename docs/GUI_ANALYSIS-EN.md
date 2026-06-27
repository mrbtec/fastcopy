# Feasibility Analysis: Graphical User Interface for fastcopy

This document analyzes the available options in the Go ecosystem for creating a graphical user interface (GUI) for `fastcopy`, evaluating pros, cons, and the final recommendation.

---

## Context

`fastcopy` is a high-performance file copying utility written in Go, currently operating via CLI. The ideal GUI should:

- Allow selecting source and destination directories.
- Display a real-time progress bar (bytes/s, ETA, files copied).
- Offer toggles for existing options (checksum, force, archive).
- **Not degrade the performance** of the copying engine (the GUI must be lightweight).

---

## Evaluated Frameworks

### 1. Fyne (`fyne.io/fyne/v2`)

| Criterion | Evaluation |
| :--- | :--- |
| **Maturity** | ⭐⭐⭐⭐⭐ — Most mature framework in the Go ecosystem for desktop GUIs |
| **Cross-platform** | ✅ Linux, macOS, Windows, Android, iOS — single codebase |
| **Visual** | Proprietary Material Design ("Fyne Design") — not native to the OS |
| **CGO** | ⚠️ **Requires CGO** — dependency on native graphics drivers (OpenGL/Vulkan) |
| **Compilation** | ⚠️ First compilation is slow (~1-2 min), but incremental builds are fast |
| **Ready-made Widgets** | ✅ FileDialog, ProgressBar, DataBinding, CheckBox, Entry — everything we need |
| **Goroutines** | ✅ `fyne.Do()` to update UI from background goroutines (thread-safe since v2.6) |
| **Binary Size** | ~15-25 MB (includes full graphics stack) |

**Feasibility for fastcopy**: ✅ **High**

Integration would be direct: the copying engine (`internal/copier.go`) already emits progress via `Progress.AddCopiedFile()` with atomic counters. The Fyne frontend would simply read these counters every 100ms via `fyne.Do()` and update a `widget.ProgressBar`.

**Conceptual integration example**:
```go
progressBar := widget.NewProgressBar()
go func() {
    engine.Run(src, dst)
    for !done {
        fyne.Do(func() {
            progressBar.SetValue(float64(progress.copiedBytes) / float64(progress.totalBytes))
        })
        time.Sleep(100 * time.Millisecond)
    }
}()
```

---

### 2. Wails (`wails.io`)

| Criterion | Evaluation |
| :--- | :--- |
| **Maturity** | ⭐⭐⭐⭐ — Very popular, but v3 is still in alpha |
| **Cross-platform** | ✅ Linux, macOS, Windows |
| **Visual** | 🎨 **Excellent** — Frontend is HTML/CSS/JS (React, Vue, Svelte) |
| **CGO** | ⚠️ Requires CGO (OS native WebView) |
| **Compilation** | ⚠️ Requires Node.js + npm in the build pipeline |
| **Ready-made Widgets** | ✅ Entire Web ecosystem (Tailwind, Shadcn, etc.) |
| **Goroutines** | ✅ Events system (`EventsEmit`) for Go ↔ JS communication |
| **Binary Size** | ~10-20 MB (uses native WebView, doesn't embed Chromium) |

**Feasibility for fastcopy**: ✅ **High, but overengineered**

The interface would look visually flawless (HTML/CSS), but it introduces two unnecessary layers of complexity:
1. Dependency on Node.js/npm in the build process.
2. IPC (Inter-Process Communication) layer between Go and JavaScript via WebView.

For a tool focused on **raw I/O speed**, adding a Go↔JS bridge is an architectural overhead that doesn't make sense.

---

### 3. Gio (`gioui.org`)

| Criterion | Evaluation |
| :--- | :--- |
| **Maturity** | ⭐⭐⭐ — Active, but smaller ecosystem |
| **Cross-platform** | ✅ Linux, macOS, Windows, Android, iOS, WASM |
| **Visual** | Custom — proprietary, minimalist style |
| **CGO** | ✅ **Does not require CGO** — pure Go! |
| **Compilation** | ✅ Fast (no CGO) |
| **Ready-made Widgets** | ⚠️ **Few** — immediate-mode; you draw everything |
| **Goroutines** | ✅ Works well, but requires manual state management |
| **Binary Size** | ~5-10 MB (lightest of all) |

**Feasibility for fastcopy**: ⚠️ **Medium**

Gio is the lightest and the only one that doesn't require CGO, which is excellent for portability. However, it lacks ready-made widgets like FileDialog or ProgressBar — you have to build everything from scratch. The development effort would be 3-5x higher than with Fyne to achieve the same result.

---

### 4. GTK (via `gotk4`)

| Criterion | Evaluation |
| :--- | :--- |
| **Maturity** | ⭐⭐⭐⭐⭐ — GTK is mature, but Go bindings are young |
| **Cross-platform** | ⚠️ **Excellent on Linux**, painful on Windows/macOS |
| **Visual** | 🎨 **Native** — perfect appearance on GNOME/Linux |
| **CGO** | ⚠️ Requires CGO + GTK4 dependencies installed on the system |
| **Compilation** | ⚠️ Requires `pkg-config`, GTK4 dev libs, complex setup |
| **Ready-made Widgets** | ✅ All GTK widgets (FileChooser, ProgressBar, etc.) |
| **Goroutines** | ⚠️ Requires `glib.IdleAdd()` for thread-safe updates |
| **Binary Size** | ~2-5 MB (but depends on system libs: ~50MB+) |

**Feasibility for fastcopy**: ⚠️ **Medium (Linux only)**

It would be the perfect choice if the target were exclusively Linux with GNOME desktop. However, the cross-compilation difficulty and heavy dependency on system libs (`libgtk-4-dev`) make this option impractical for wide distribution.

---

### 5. Bubble Tea (`github.com/charmbracelet/bubbletea`)

| Criterion | Evaluation |
| :--- | :--- |
| **Type** | ⚠️ **TUI (Terminal UI)** — not a graphical GUI |
| **Maturity** | ⭐⭐⭐⭐⭐ — Gold standard for terminal interfaces in Go |
| **Cross-platform** | ✅ Any terminal |
| **CGO** | ✅ **Does not require CGO** — pure Go |
| **Visual** | 🎨 Beautiful for terminal (colors, borders, animations) |

**Feasibility for fastcopy**: ✅ **High as an intermediate alternative**

Bubble Tea doesn't create graphical windows, but it can transform the current CLI into a rich and interactive terminal experience, with animated progress bars, directory selection via navigation, and colored panels — all without any external dependencies.

---

## Final Comparative Table

| Framework | Feasibility | CGO | Dev Effort | Visual | Binary Size | Recommendation |
| :--- | :---: | :---: | :---: | :---: | :---: | :--- |
| **Fyne** | ✅ High | Yes | Low | Good | ~20 MB | **🏆 Recommended** |
| **Wails** | ✅ High | Yes | Medium | Excellent | ~15 MB | Overengineered for the case |
| **Gio** | ⚠️ Medium | No | High | Custom | ~8 MB | Good if CGO is forbidden |
| **GTK** | ⚠️ Medium | Yes | Medium | Native | ~3 MB* | Only for Linux |
| **Bubble Tea** | ✅ High | No | Low | Terminal | ~5 MB | Excellent TUI alternative |

> **\*** The GTK binary is small, but depends on ~50MB+ of system libraries.

---

## Conclusion and Recommendation

### 🏆 Primary Recommendation: **Fyne**

**Fyne is the most viable choice** for creating the `fastcopy` GUI for the following reasons:

1. **Natural integration with Go**: The `fastcopy` engine already uses goroutines and atomic counters — the `fyne.Do()` model fits perfectly.
2. **Ready-made widgets**: `dialog.ShowFolderOpen()`, `widget.ProgressBar`, `widget.Check` — everything we need already exists.
3. **Real cross-platform**: A single binary runs on Linux, macOS, and Windows.
4. **Moderate effort**: Estimated ~200-300 lines of Go code for the complete GUI.

### 🥈 Alternative: **Bubble Tea (TUI)**

If the priority is **zero external dependencies** and **maximum portability** (no CGO, no graphics drivers), Bubble Tea offers a rich visual experience in the terminal with minimal development effort. Ideal for headless servers.

### Suggested Approach: **Dual-mode**

The ideal implementation would be to maintain both modes:
- `fastcopy src dst` — current CLI mode (no changes).
- `fastcopy --gui` — opens the Fyne interface.
- `fastcopy --tui` — opens the Bubble Tea interface (interactive in terminal).

This maintains compatibility with existing scripts while adding accessibility for users who prefer visual interfaces.
