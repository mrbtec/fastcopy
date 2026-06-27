# Manual de fastcopy

`fastcopy` es una utilidad de línea de comandos para Linux (y otras plataformas soportadas) escrita en Go. Ha sido diseñada para copiar archivos a velocidades altísimas, superando las limitaciones de herramientas tradicionales como `cp` y `rsync`.

## Cómo Funciona

`fastcopy` alcanza velocidades extremas combinando múltiples técnicas de vanguardia:

1. **Paralelismo Avanzado**: En lugar de copiar un archivo a la vez de forma serial, la utilidad separa los archivos en dos colas:
   - **Archivos Pequeños (< 64MB)**: Procesados con un paralelismo muy alto (predeterminado: `2 * número_de_CPUs`).
   - **Archivos Grandes (≥ 64MB)**: Procesados con un límite reducido para evitar cuellos de botella (saturación) en la E/S del disco.
2. **Copia de Bloques Concurrente**: Archivos absurdamente grandes (≥ 1GB) no se envían simplemente a una cola; se "fragmentan" lógicamente (chunks de 100MB) y se copian simultáneamente mediante diversos workers, potenciando la escritura.
3. **Transferencia Zero-Copy (En el Kernel)**: En Linux, la utilidad activa directamente la llamada al sistema `copy_file_range`, efectuando la transferencia directamente dentro del Kernel, sin que los bytes necesiten cruzar las fronteras hacia el "espacio de usuario" (userspace). Esto también permite copias instantáneas usando COW (Copy-On-Write) en archivos de sistema Btrfs/XFS (reflink).
4. **Asignación de Espacio Rápida**: Utiliza la llamada al sistema `fallocate` para garantizar bloques contiguos en el disco de destino antes de escribir, eliminando la fragmentación y evitando fallos de disco lleno en medio de copias grandes.
5. **Comunicación Activa con la Caché (fadvise)**: Usa `posix_fadvise` para preparar la caché de lectura y descartar los archivos de la memoria tan pronto como se copian (`FADV_DONTNEED`). Su servidor/escritorio ya no se volverá lento ni se bloqueará porque Linux llenó el 100% de su RAM con caché de lectura mientras transfería 50GB.
6. **Modo Incremental Altamente Optimizado**: Escanea todo el árbol e ignora archivos que ya existen en el destino con el mismo tamaño y fecha de modificación (`mtime`). Los enlaces simbólicos ya existentes en el destino se detectan preventivamente y se ignoran.

---

## Cómo Instalar y Compilar

### Requisitos

*   Lenguaje Go (versión 1.21 o superior).
*   Entorno preferido: **Linux**. Para Windows/macOS, el código entra en modo "fallback" (usa buffers normales y falla graciosamente las llamadas al sistema exclusivas del kernel de Linux).

### Compilación
```bash
cd /camino/de/su/repositorio/fastcopy
go build -o fastcopy ./cmd/fastcopy/
```

Para hacer que la utilidad sea accesible desde cualquier lugar del sistema:
```bash
sudo mv fastcopy /usr/local/bin/
# o
go install ./cmd/fastcopy/
```

### Compilación de la Interfaz Gráfica (Fyne)

La GUI requiere dependencias de desarrollo X11/OpenGL para compilar:

```bash
# Ubuntu/Debian
sudo apt-get install -y \
  libx11-dev libxcursor-dev libxrandr-dev \
  libxinerama-dev libxi-dev libglx-dev \
  libgl1-mesa-dev libxxf86vm-dev

# Fedora/RHEL
sudo dnf install -y \
  libX11-devel libXcursor-devel libXrandr-devel \
  libXinerama-devel libXi-devel mesa-libGL-devel

# Compilar la GUI
go build -o fastcopy-gui ./cmd/fastcopy-gui/
```

---

## Cómo Usar

El uso más básico es similar a ejecutar un `cp -a`:

```bash
fastcopy /camino/de/origen /camino/de/destino
```
*   A diferencia de `cp`, el comportamiento predeterminado es **recursivo**.
*   Se mostrará una barra de progreso en tiempo real con el tiempo estimado y la tasa de copia (MB/s o GB/s).

### Opciones y Flags Disponibles

La utilidad acepta los siguientes parámetros de configuración:

| Flag / Parámetro | Tipo | Predeterminado | Descripción |
| :--- | :--- | :--- | :--- |
| `-w N` | Entero | `NumCPU * 2` | Define el número máximo de "workers" paralelos. Auméntelo si usa HDDs con alto nivel de NCQ, o disminúyalo si hay mucho cuello de botella. |
| `--checksum` | Booleano | `false` | Calcula la integridad de datos vía hash **SHA256** _mientras_ el archivo transita por el canal, generando un informe al final (sin impacto de doble lectura). |
| `--dry-run` | Booleano | `false` | Solo escanea los directorios y lista en la terminal lo que se ejecutaría (útil para auditoría o prueba de incremental). |
| `--force` | Booleano | `false` | Ignora la comprobación inteligente de incremental, forzando la lectura y sobrescritura de **todos los datos y archivos**. |
| `--no-archive` | Booleano | `false` | Impide que la utilidad preserve los permisos, fechas (`mtime`/`atime`) y propietario (ownership) de los archivos de origen. |
| `--quiet` | Booleano | `false` | Elimina la barra de progreso, imprimiendo texto solo al final o solo en caso de error (perfecto para scripts CI/CD y automatización). |
| `--version` | Booleano | `false` | Muestra la versión del sistema y finaliza. |

### Ejemplos Prácticos Avanzados

1.  **Forzando 64 procesos simultáneos y generando hashes verificables SHA256**:
    ```bash
    fastcopy -w 64 --checksum /mnt/servidor/datos /datos_locales/backup
    ```

2.  **Verificando qué se copiaría hoy (modo Simulación Incremental)**:
    ```bash
    fastcopy --dry-run /datos/activos /archivos/historicos
    ```

3.  **Realizando un respaldo para ejecutar silenciosamente en segundo plano mediante Cron**:
    ```bash
    fastcopy --quiet /var/log /backup/log
    ```

---

## Solución de Problemas Comunes

### 1. `non-root users can't change ownership` (Error al cambiar propietario)
Al copiar con el modo "archive" (el predeterminado) activado, el sistema intentará replicar quién es el dueño del archivo (`UID`/`GID`). Si no está usando el usuario `root`, Linux denegará esta operación y podrá ver el mensaje de registro (aunque la utilidad intenta continuar el trabajo graciosamente). Ejecute la utilidad con `sudo` si hay una necesidad real de replicar permisos ajenos.

### 2. Copiando a un disco en red (NFS / Samba)
Los sistemas que envían datos a redes (`nfs`, `cifs`, etc.) no podrán utilizar ciertas lógicas "mágicas" de Zero-Copy vía `copy_file_range` o preasignación (`fallocate`). La utilidad está configurada para "fallar graciosamente" y volver a los buffers estándar de lectura (`io.Copy`), manteniendo su alta velocidad de concurrencia incluso si el sistema de destino no lo soporta.

### 3. La velocidad mostrada en la barra de progreso bajó repentinamente
Cuando los workers comienzan el trabajo en la "Cola de archivos Gigantes", el impacto de E/S en la placa base y el HD se dispara. Si el número de workers en paralelo es alto (en discos mecánicos SATA, por ejemplo), los cabezales del HD causarán mucho Seek, congelando las tasas momentáneamente. Intente bajar el recuento de hilos a `-w 4` o `-w 8` si está tratando exclusivamente con HDDs lentos.
