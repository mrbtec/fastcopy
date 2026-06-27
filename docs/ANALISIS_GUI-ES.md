# Análisis de Viabilidad: Interfaz Gráfica para fastcopy

Este documento analiza las opciones disponibles en el ecosistema Go para crear una interfaz gráfica (GUI) para `fastcopy`, evaluando pros, contras y la recomendación final.

---

## Contexto

`fastcopy` es una utilidad de copia de archivos de alto rendimiento, escrita en Go, que opera actualmente vía CLI. La GUI ideal debe:

- Permitir seleccionar directorios de origen y destino.
- Mostrar una barra de progreso en tiempo real (bytes/s, ETA, archivos copiados).
- Ofrecer selectores para las opciones existentes (checksum, force, archive).
- **No degradar el rendimiento** del motor de copia (la GUI debe ser ligera).

---

## Frameworks Evaluados

### 1. Fyne (`fyne.io/fyne/v2`)

| Criterio | Evaluación |
| :--- | :--- |
| **Madurez** | ⭐⭐⭐⭐⭐ — Framework más maduro del ecosistema Go para GUI de escritorio |
| **Multiplataforma** | ✅ Linux, macOS, Windows, Android, iOS — código único |
| **Visual** | Material Design propio ("Fyne Design") — no es nativo del SO |
| **CGO** | ⚠️ **Requiere CGO** — dependencia de controladores gráficos nativos (OpenGL/Vulkan) |
| **Compilación** | ⚠️ Primera compilación lenta (~1-2 min), pero las incrementales son rápidas |
| **Widgets listos** | ✅ FileDialog, ProgressBar, DataBinding, CheckBox, Entry — todo lo que necesitamos |
| **Goroutines** | ✅ `fyne.Do()` para actualizar la UI desde goroutines en segundo plano (thread-safe desde v2.6) |
| **Peso del binario** | ~15-25 MB (incluye stack gráfico completo) |

**Viabilidad para fastcopy**: ✅ **Alta**

La integración sería directa: el motor de copia (`internal/copier.go`) ya emite progreso vía `Progress.AddCopiedFile()` con contadores atómicos. Bastaría con que el frontend de Fyne lea estos contadores cada 100ms vía `fyne.Do()` y actualice un `widget.ProgressBar`.

**Ejemplo conceptual de integración**:
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

| Criterio | Evaluación |
| :--- | :--- |
| **Madurez** | ⭐⭐⭐⭐ — Muy popular, pero la v3 aún está en alpha |
| **Multiplataforma** | ✅ Linux, macOS, Windows |
| **Visual** | 🎨 **Excelente** — Frontend es HTML/CSS/JS (React, Vue, Svelte) |
| **CGO** | ⚠️ Requiere CGO (WebView nativo del SO) |
| **Compilación** | ⚠️ Requiere Node.js + npm en el pipeline de construcción |
| **Widgets listos** | ✅ Todo el ecosistema Web (Tailwind, Shadcn, etc.) |
| **Goroutines** | ✅ Sistema de eventos (`EventsEmit`) para comunicación Go ↔ JS |
| **Peso del binario** | ~10-20 MB (usa WebView nativo, no embebe Chromium) |

**Viabilidad para fastcopy**: ✅ **Alta, pero sobre-diseñada**

La interfaz quedaría visualmente impecable (HTML/CSS), pero introduce dos capas de complejidad innecesarias:
1. Dependencia de Node.js/npm en el proceso de construcción.
2. Capa de IPC (Inter-Process Communication) entre Go y JavaScript vía WebView.

Para una herramienta enfocada en la **velocidad bruta de E/S**, añadir un puente Go↔JS es una sobrecarga arquitectónica que no tiene sentido.

---

### 3. Gio (`gioui.org`)

| Criterio | Evaluación |
| :--- | :--- |
| **Madurez** | ⭐⭐⭐ — Activo, pero ecosistema más pequeño |
| **Multiplataforma** | ✅ Linux, macOS, Windows, Android, iOS, WASM |
| **Visual** | Personalizado — estilo propio, minimalista |
| **CGO** | ✅ **No requiere CGO** — ¡Go puro! |
| **Compilación** | ✅ Rápida (sin CGO) |
| **Widgets listos** | ⚠️ **Pocos** — modo inmediato (immediate-mode), dibujas todo |
| **Goroutines** | ✅ Funciona bien, pero requiere gestión manual del estado |
| **Peso del binario** | ~5-10 MB (el más ligero de todos) |

**Viabilidad para fastcopy**: ⚠️ **Media**

Gio es el más ligero y el único que no requiere CGO, lo cual es excelente para la portabilidad. Sin embargo, no tiene widgets listos como FileDialog o ProgressBar; hay que construir todo desde cero. El esfuerzo de desarrollo sería 3-5 veces mayor que con Fyne para lograr el mismo resultado.

---

### 4. GTK (vía `gotk4`)

| Criterio | Evaluación |
| :--- | :--- |
| **Madurez** | ⭐⭐⭐⭐⭐ — GTK es maduro, pero los bindings de Go son jóvenes |
| **Multiplataforma** | ⚠️ **Excelente en Linux**, doloroso en Windows/macOS |
| **Visual** | 🎨 **Nativo** — apariencia perfecta en GNOME/Linux |
| **CGO** | ⚠️ Requiere CGO + dependencias de GTK4 instaladas en el sistema |
| **Compilación** | ⚠️ Requiere `pkg-config`, libs de desarrollo de GTK4, configuración compleja |
| **Widgets listos** | ✅ Todos los widgets de GTK (FileChooser, ProgressBar, etc.) |
| **Goroutines** | ⚠️ Requiere `glib.IdleAdd()` para actualizaciones thread-safe |
| **Peso del binario** | ~2-5 MB (pero depende de libs del sistema: ~50MB+) |

**Viabilidad para fastcopy**: ⚠️ **Media (solo Linux)**

Sería la elección perfecta si el objetivo fuera exclusivamente Linux con escritorio GNOME. Sin embargo, la dificultad de compilación cruzada y la dependencia pesada de libs del sistema (`libgtk-4-dev`) hacen que esta opción sea impracticable para una distribución amplia.

---

### 5. Bubble Tea (`github.com/charmbracelet/bubbletea`)

| Criterio | Evaluación |
| :--- | :--- |
| **Tipo** | ⚠️ **TUI (Terminal UI)** — no es una GUI gráfica |
| **Madurez** | ⭐⭐⭐⭐⭐ — Estándar de oro para interfaces de terminal en Go |
| **Multiplataforma** | ✅ Cualquier terminal |
| **CGO** | ✅ **No requiere CGO** — Go puro |
| **Visual** | 🎨 Bellísimo para terminal (colores, bordes, animaciones) |

**Viabilidad para fastcopy**: ✅ **Alta como alternativa intermedia**

Bubble Tea no crea ventanas gráficas, pero puede transformar la CLI actual en una experiencia de terminal rica e interactiva, con barras de progreso animadas, selección de directorios vía navegación y paneles coloridos, todo sin ninguna dependencia externa.

---

## Tabla Comparativa Final

| Framework | Viabilidad | CGO | Esfuerzo Dev | Visual | Peso Binario | Recomendación |
| :--- | :---: | :---: | :---: | :---: | :---: | :--- |
| **Fyne** | ✅ Alta | Sí | Bajo | Bueno | ~20 MB | **🏆 Recomendado** |
| **Wails** | ✅ Alta | Sí | Medio | Excelente | ~15 MB | Sobre-diseñado para el caso |
| **Gio** | ⚠️ Media | No | Alto | Custom | ~8 MB | Bueno si CGO está prohibido |
| **GTK** | ⚠️ Media | Sí | Medio | Nativo | ~3 MB* | Solo para Linux |
| **Bubble Tea** | ✅ Alta | No | Bajo | Terminal | ~5 MB | Excelente alternativa TUI |

> **\*** El binario de GTK es pequeño, pero depende de ~50MB+ de bibliotecas del sistema.

---

## Conclusión y Recomendación

### 🏆 Recomendación Principal: **Fyne**

**Fyne es la opción más viable** para crear la GUI de `fastcopy` por las siguientes razones:

1. **Integración natural con Go**: El motor de `fastcopy` ya usa goroutines y contadores atómicos; el modelo `fyne.Do()` encaja perfectamente.
2. **Widgets listos**: `dialog.ShowFolderOpen()`, `widget.ProgressBar`, `widget.Check` — todo lo que necesitamos ya existe.
3. **Multiplataforma real**: Un único binario corre en Linux, macOS y Windows.
4. **Esfuerzo moderado**: Estimación de ~200-300 líneas de código Go para la GUI completa.

### 🥈 Alternativa: **Bubble Tea (TUI)**

Si la prioridad es **cero dependencias externas** y **máxima portabilidad** (sin CGO, sin controladores gráficos), Bubble Tea ofrece una experiencia visual rica en la terminal con un esfuerzo mínimo de desarrollo. Ideal para servidores headless.

### Enfoque sugerido: **Modo Dual**

La implementación ideal sería mantener ambos modos:
- `fastcopy src dst` — modo CLI actual (sin cambios).
- `fastcopy --gui` — abre la interfaz Fyne.
- `fastcopy --tui` — abre la interfaz Bubble Tea (interactiva en la terminal).

Esto mantiene la compatibilidad con scripts existentes mientras añade accesibilidad para usuarios que prefieren interfaces visuales.
