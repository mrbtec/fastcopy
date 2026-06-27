# OPCIONES-COMPRESION.md

## Introducción

El proyecto FastCopy es una herramienta de copia de archivos de alta velocidad que busca proporcionar una solución eficiente y confiable para la copia de archivos. En este documento, se presentan las opciones de compresión que pueden utilizarse para reducir el tamaño de los archivos y mejorar la eficiencia de la copia.

## Opciones de Compresión

A continuación, se enumeran las opciones de compresión que pueden utilizarse en el proyecto FastCopy:

1. **Sin Compresión**: No realiza compresión en los archivos, lo que puede ser útil para archivos que no son compresibles o que ya están comprimidos.
2. **Gzip**: Utiliza el algoritmo de compresión Gzip para reducir el tamaño de los archivos. Gzip es un algoritmo de compresión popular que es ampliamente soportado en muchos sistemas operativos.
3. **Bzip2**: Utiliza el algoritmo de compresión Bzip2 para reducir el tamaño de los archivos. Bzip2 es un algoritmo de compresión más eficiente que Gzip, pero puede ser más lento.
4. **Lzma**: Utiliza el algoritmo de compresión Lzma para reducir el tamaño de los archivos. Lzma es un algoritmo de compresión que ofrece una buena relación entre la tasa de compresión y la velocidad de compresión.
5. **Xz**: Utiliza el algoritmo de compresión Xz para reducir el tamaño de los archivos. Xz es un algoritmo de compresión que ofrece una buena relación entre la tasa de compresión y la velocidad de compresión, y es ampliamente soportado en muchos sistemas operativos.

## Parámetros de Compresión

A continuación, se enumeran los parámetros de compresión que pueden utilizarse para personalizar la compresión:

* **Nivel de Compresión**: Especifica el nivel de compresión que debe utilizarse. Un nivel más alto de compresión puede resultar en archivos más comprimidos, pero puede ser más lento.
* **Tamaño del Buffer**: Especifica el tamaño del buffer que debe utilizarse para la compresión. Un tamaño de buffer más grande puede resultar en una compresión más eficiente, pero puede requerir más memoria.
* **Algoritmo de Compresión**: Especifica el algoritmo de compresión que debe utilizarse. Los algoritmos de compresión disponibles son Gzip, Bzip2, Lzma y Xz.

## Ejemplos de Uso

A continuación, se presentan ejemplos de cómo utilizar las opciones de compresión en el proyecto FastCopy:

```bash
# Copiar un archivo sin compresión
fastcopy -c none archivo.txt destino.txt

# Copiar un archivo con compresión Gzip
fastcopy -c gzip -l 6 -b 4096 archivo.txt destino.txt

# Copiar un archivo con compresión Bzip2
fastcopy -c bzip2 -l 9 -b 8192 archivo.txt destino.txt

# Copiar un archivo con compresión Lzma
fastcopy -c lzma -l 7 -b 4096 archivo.txt destino.txt

# Copiar un archivo con compresión Xz
fastcopy -c xz -l 6 -b 4096 archivo.txt destino.txt
```
