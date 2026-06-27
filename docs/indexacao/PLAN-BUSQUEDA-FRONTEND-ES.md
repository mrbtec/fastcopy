# Plan de Implementación de la Búsqueda en el Frontend (fastcopy‑gui)

## 1. Visión General
La GUI **fastcopy‑gui** actualmente sirve solo como un frontend para la operación de copia. No hay ningún formulario de búsqueda y el backend de búsqueda (`internal/index/search.go`) está acoplado a la CLI. Para que la búsqueda sea **tan rápida como la propia operación de copia**, necesitamos:

1. **Mejorar el rendimiento del backend** (consultas O(1) y O(log N), reducción de bucles de trabajo).
2. **Exponer el backend a la GUI** a través de una API simple.
3. **Implementar un formulario de búsqueda** en la GUI que ejecute consultas de forma asíncrona y muestre resultados paginados/virtualizados.

---

## 2. Problemas Identificados en el Backend Actual
| Problema | Localización | Impacto |
|----------|-------------|---------|
| Búsqueda lineal `O(n)` general | `search.go` (función `Search`) | Recorre toda la lista de `Entries` – aproximadamente 1 M de iteraciones para 1 M de archivos en cualquier búsqueda de nombre. |
| `LookupByPath` lineal | `search.go` (función `LookupByPath`) | Búsqueda por camino exacto en tiempo `O(n)`. |
| Falta de `PathMap` | `index.go` – solo existe `HashMap` | Imposibilita el acceso directo `O(1)` a una `Entry` por camino. |
| Resultados no limitados | `search.go` – ningún parámetro `Limit/Offset` | Puede generar miles de líneas en la GUI, sobrecargando la UI y la asignación de memoria. |
| El Builder no ordena `Entries`| `builder.go` | La lista queda en el orden de descubrimiento del SO, imposibilitando la búsqueda binaria (`O(log N)`) para prefijos. |
| Aislamiento de la GUI | `cmd/fastcopy-gui/main.go` | La GUI no tiene acceso ni pestañas relacionadas con la indexación. |

---

## 3. Mejoras en el Backend
### 3.1 Estructura de Datos
1. **Agregar `PathMap` y garantizar la ordenación:**
   ```go
   type Index struct {
       Version   int
       RootPath  string
       CreatedAt time.Time
       Entries   []Entry            // será ordenada por Path
       HashMap   map[string][]int   // hash → índices en Entries
       PathMap   map[string]int     // camino relativo → índice en Entries (acceso O(1))
   }
   ```
2. **Ordenación en el Builder:**
   Al final de la función `BuildFromScan` (en `builder.go`), recolectar todas las entradas, ordenarlas con `sort.Slice` por `Path`, y luego recorrer el array una sola vez para poblar `PathMap` y `HashMap`.

### 3.2 API de Búsqueda Optimizada (`Query`)
Agregar soporte a la paginación en la struct de Query existente:
```go
type Query struct {
    Name       string 
    MinSize    int64
    MaxSize    int64
    Hash       string
    Duplicates bool
    Limit      int   // número máximo de resultados (0 = sin límite)
    Offset     int   // inicio de la página
}
```

**Estrategias de Búsqueda en `search.go`:**
- **Búsqueda por Hash (Exacto):** Usar el `HashMap`. Complejidad `O(1)`.
- **Búsqueda por Path (Exacto):** Usar el `PathMap`. Complejidad `O(1)`.
- **Búsqueda por Prefijo de Path (ej: `carpeta/*`):** Como `Entries` estará ordenado alfabéticamente por `Path`, usar búsqueda binaria (`sort.Search`) para encontrar el primer elemento. Después, iterar secuencialmente hasta que el prefijo ya no coincida. Complejidad `O(log N + K)`.
- **Búsqueda por Glob/Sufijo (ej: `*.go`):** Sigue siendo `O(N)`, pero con `Limit/Offset` deteniéndose anticipadamente.

---

## 4. Integración en la GUI (Fyne)
### 4.1 Estado de la Aplicación
Evitar variables globales. En la GUI (`main.go`), crear una estructura de estado para mantener el índice cargado:

```go
type AppState struct {
    currentIndex *index.Index
    // mutex si es necesario para recarga asíncrona
}
```

### 4.2 Nuevo Diseño: Sistema de Pestañas (Tabs)
La interfaz debe convertirse para usar `container.NewAppTabs`:
- **Pestaña 1: Copiador** (interfaz actual)
- **Pestaña 2: Búsqueda e Índice** (nueva interfaz)

### 4.3 Formulario de Búsqueda en la Pestaña 2
1. **Controles (Superior):**
   - Botón **Cargar Índice** (abre selector de archivo `.idx`).
   - `Entry` para el término de búsqueda.
   - `Select` para el tipo de búsqueda (Nombre/Prefijo, Camino Exacto, Hash).
   - Botón **Buscar**.
2. **Exhibición de Resultados (Centro):**
   - Utilizar el widget nativo de virtualización de Fyne: `widget.NewList`.
   - `widget.NewList` instancia solo los elementos visibles en pantalla y los recicla al desplazar, garantizando un rendimiento perfecto (60 FPS) incluso si hay 100,000 resultados. No intenta cargar la UI con miles de widgets individuales.
3. **Controles (Inferior):**
   - Etiquetas mostrando: "Resultados encontrados: X", "Tiempo de búsqueda: Y ms".

### 4.4 Asincronía
- Cuando el usuario haga clic en "Buscar", la llamada `idx.Search(query)` debe ejecutarse en una **goroutine**.
- La UI puede mostrar un `widget.NewProgressBarInfinite()` mientras busca.
- Al finalizar, la goroutine actualiza la lista y oculta la barra de progreso (la renderización de datos en el List es thread-safe si los datos base se sustituyen antes de llamar a `list.Refresh()`).

---

## 5. Plan de Ejecución

| Etapa | Descripción | Archivos Impactados |
|-------|-----------|--------------------|
| 1 | Agregar `PathMap` al `Index` y garantizar ordenación (`sort.Slice` y repoblar maps) | `internal/index/index.go`, `builder.go` |
| 2 | Refactorizar `Search` para usar búsquedas `O(1)` y `O(log N)` (Hash, Path exacto, Prefijo) | `search.go` |
| 3 | Agregar soporte a `Limit` y `Offset` en `Query` y `Search` | `index.go`, `search.go` |
| 4 | Actualizar CLI para compilar con los nuevos campos de la `Query` (si aplica) | `cmd/fastcopy/main.go` |
| 5 | Migrar interfaz de `fastcopy-gui` a `AppTabs` (Copia / Búsqueda) | `cmd/fastcopy-gui/main.go` |
| 6 | Implementar carga del índice (`Load`) y formulario de búsqueda asíncrono en la GUI | `cmd/fastcopy-gui/main.go` |
| 7 | Renderizar resultados usando `widget.NewList` (virtualizado) | `cmd/fastcopy-gui/main.go` |

---

## 6. Riesgos y Mitigaciones
| Riesgo | Mitigación |
|-------|-----------|
| **Índice Grande (RAM)** | El `PathMap` añade sobrecarga de memoria. Para 1 millón de archivos, un `map[string]int` ocupará aprox 20-30 MB. Es muy aceptable dado el aumento de rendimiento. |
| **Bloqueo de la UI al Cargar** | `idx.Load` tarda un poco debido a `gob`. La carga debe hacerse en goroutine con spinner. |
| **Errores de Concurrencia en la Lista Fyne** | Sustituir el slice de datos base (ej: `searchResults = newResults`) y solo entonces llamar a `list.Refresh()`, evitando mutar los elementos individuales durante el renderizado de la lista. |
