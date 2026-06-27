# COMPRESSION-OPTIONS.md

## Introduction

The FastCopy project is a high-speed file copying tool that aims to provide an efficient and reliable solution for copying files. In this document, the compression options that can be used to reduce file size and improve copying efficiency are presented.

## Compression Options

The following compression options can be used in the FastCopy project:

1. **No Compression**: Does not perform compression on files, which can be useful for files that are not compressible or are already compressed.
2. **Gzip**: Uses the Gzip compression algorithm to reduce file size. Gzip is a popular compression algorithm widely supported on many operating systems.
3. **Bzip2**: Uses the Bzip2 compression algorithm to reduce file size. Bzip2 is a more efficient compression algorithm than Gzip, but it can be slower.
4. **Lzma**: Uses the Lzma compression algorithm to reduce file size. Lzma is a compression algorithm that offers a good balance between compression ratio and compression speed.
5. **Xz**: Uses the Xz compression algorithm to reduce file size. Xz is a compression algorithm that offers a good balance between compression ratio and compression speed, and is widely supported on many operating systems.

## Compression Parameters

The following compression parameters can be used to customize compression:

* **Compression Level**: Specifies the compression level to be used. A higher compression level can result in more compressed files but may be slower.
* **Buffer Size**: Specifies the buffer size to be used for compression. A larger buffer size can result in more efficient compression but may require more memory.
* **Compression Algorithm**: Specifies the compression algorithm to be used. Available compression algorithms are Gzip, Bzip2, Lzma, and Xz.

## Usage Examples

Below are examples of how to use compression options in the FastCopy project:

```bash
# Copy a file without compression
fastcopy -c none file.txt destination.txt

# Copy a file with Gzip compression
fastcopy -c gzip -l 6 -b 4096 file.txt destination.txt

# Copy a file with Bzip2 compression
fastcopy -c bzip2 -l 9 -b 8192 file.txt destination.txt

# Copy a file with Lzma compression
fastcopy -c lzma -l 7 -b 4096 file.txt destination.txt

# Copy a file with Xz compression
fastcopy -c xz -l 6 -b 4096 file.txt destination.txt
```
