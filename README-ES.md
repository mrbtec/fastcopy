# fastcopy ⚡

Un copiador de archivos paralelo ultra-rápido desarrollado en Go. Diseñado para superar el rendimiento de herramientas tradicionales como `cp` y `rsync`, aprovechando llamadas al sistema de alto rendimiento (como `copy_file_range`, `fallocate` y `fadvise` en Linux).

## Características Principales

- **Rendimiento Extremo:** Utiliza llamadas al sistema zero-copy, preasignación de disco y sugerencias inteligentes de E/S.
- **Procesamiento Paralelo:** Despachador (dispatcher) paralelizado, optimizado para manejar de manera eficiente una mezcla de archivos pequeños y grandes.
- **Sincronización Incremental:** Omite automáticamente los archivos no modificados, acelerando significativamente las actualizaciones.
- **Indexación y Búsqueda (¡Nuevo!):** Cree índices de directorios para búsquedas ultra-rápidas (`O(log N)` y `O(1)`) y detección de archivos duplicados usando SHA-256.
- **Preservación de Metadatos:** Mantiene los permisos, marcas de tiempo (fechas de modificación) y otros metadatos originales.
- **Interfaz Dual:** Incluye tanto una herramienta de Línea de Comandos (CLI) robusta, como una Interfaz Gráfica (GUI) moderna desarrollada con el framework [Fyne](https://fyne.io/) que contiene pestañas de Copia y Búsqueda.

## Prerrequisitos

- [Go](https://go.dev/) instalado.
- Para la interfaz gráfica (GUI) en Linux, necesitará las librerías de desarrollo de X11/OpenGL.

## Guía Rápida

El proyecto cuenta con la utilidad `start.sh` para facilitar todas las operaciones comunes.

### 1. Instalar dependencias de la GUI (necesario solo para la interfaz gráfica)
```bash
./start.sh deps
```
*Este comando requiere `sudo` y detecta automáticamente si usa `apt`, `dnf` o `pacman`.*

### 2. Compilar
```bash
# Compilar solo la CLI
./start.sh build

# Compilar la GUI
./start.sh build-gui

# Compilar ambos
./start.sh build-all
```

### 3. Ejecutar (Operaciones de Copia)

**CLI:**
```bash
# Ejemplo básico de copia
./start.sh run /ruta/origen /ruta/destino

# Ejemplo avanzado con 32 workers paralelos y validación de checksum
./start.sh run -w 32 --checksum /ruta/origen /ruta/destino
```

**GUI:**
```bash
# Abre la interfaz gráfica con pestañas de Copiador y Búsqueda de Índice
./start.sh run-gui
```

### 4. Ejecutar (Indexación y Búsqueda CLI)

`fastcopy` no solo copia, sino que permite escanear directorios enteros rápidamente para crear índices estáticos (`.idx`), buscar en ellos o encontrar duplicados:

```bash
# 1. Crear un índice del directorio calculando Hashes SHA-256
./start.sh run --index-build --index-hash --index-path=mi_respaldo.idx /ruta/origen

# 2. Buscar instantáneamente en el índice creado
./start.sh run --index-search="*.mp4" --index-path=mi_respaldo.idx

# 3. Listar todos los archivos duplicados en el índice (basado en Hash)
./start.sh run --index-dupes --index-path=mi_respaldo.idx
```

*Nota: ¡También puede cargar el archivo `.idx` generado directamente en la pestaña "Index Search" de `fastcopy-gui` para navegar visualmente!*

### 5. Pruebas
Ejecutar la suite de pruebas de integración, que verifica copias incrementales, checksums y copias en seco (dry run):
```bash
./start.sh test
```

## Estructura del Código

- `cmd/fastcopy/`: Punto de entrada de la aplicación de Línea de Comandos (CLI).
- `cmd/fastcopy-gui/`: Punto de entrada de la aplicación Gráfica (GUI) en Fyne.
- `internal/`: Lógica central del copiador, motor paralelo y optimizaciones zero-copy.
- `internal/index/`: Motor de serialización `gob` puramente en Go para Indexación, Búsqueda Binaria y Deduplicación.
- `start.sh`: Script gestor de tareas para desarrolladores y usuarios.

## Licencia

Este proyecto está licenciado bajo la [MIT License](LICENSE).
