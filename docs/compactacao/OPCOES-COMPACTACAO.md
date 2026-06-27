# OPCOES-COMPACTACAO.md

## Introdução

O projeto FastCopy é uma ferramenta de cópia de arquivos de alta velocidade que visa fornecer uma solução eficiente e confiável para a cópia de arquivos. Neste documento, serão apresentadas as opções de compactação que podem ser utilizadas para reduzir o tamanho dos arquivos e melhorar a eficiência da cópia.

## Opções de Compactação

A seguir, estão listadas as opções de compactação que podem ser utilizadas no projeto FastCopy:

1. **Nenhuma Compactação**: Não realiza compactação nos arquivos, o que pode ser útil para arquivos que não são compressíveis ou que já estão compactados.
2. **Gzip**: Utiliza o algoritmo de compactação Gzip para reduzir o tamanho dos arquivos. O Gzip é um algoritmo de compactação popular que é amplamente suportado em muitos sistemas operacionais.
3. **Bzip2**: Utiliza o algoritmo de compactação Bzip2 para reduzir o tamanho dos arquivos. O Bzip2 é um algoritmo de compactação mais eficiente que o Gzip, mas pode ser mais lento.
4. **Lzma**: Utiliza o algoritmo de compactação Lzma para reduzir o tamanho dos arquivos. O Lzma é um algoritmo de compactação que oferece uma boa relação entre a taxa de compactação e a velocidade de compactação.
5. **Xz**: Utiliza o algoritmo de compactação Xz para reduzir o tamanho dos arquivos. O Xz é um algoritmo de compactação que oferece uma boa relação entre a taxa de compactação e a velocidade de compactação, e é amplamente suportado em muitos sistemas operacionais.

## Parâmetros de Compactação

A seguir, estão listados os parâmetros de compactação que podem ser utilizados para personalizar a compactação:

* **Nível de Compactação**: Especifica o nível de compactação que deve ser utilizado. Um nível mais alto de compactação pode resultar em arquivos mais compactados, mas pode ser mais lento.
* **Tamanho do Buffer**: Especifica o tamanho do buffer que deve ser utilizado para a compactação. Um tamanho de buffer mais grande pode resultar em uma compactação mais eficiente, mas pode exigir mais memória.
* **Algoritmo de Compactação**: Especifica o algoritmo de compactação que deve ser utilizado. Os algoritmos de compactação disponíveis são Gzip, Bzip2, Lzma e Xz.

## Exemplos de Uso

A seguir, estão exemplos de como utilizar as opções de compactação no projeto FastCopy:

```bash
# Copiar um arquivo sem compactação
fastcopy -c none arquivo.txt destino.txt

# Copiar um arquivo com compactação Gzip
fastcopy -c gzip -l 6 -b 4096 arquivo.txt destino.txt

# Copiar um arquivo com compactação Bzip2
fastcopy -c bzip2 -l 9 -b 8192 arquivo.txt destino.txt

# Copiar um arquivo com compactação Lzma
fastcopy -c lzma -l 7 -b 4096 arquivo.txt destino.txt

# Copiar um arquivo com compactação Xz
fastcopy -c xz -l 6 -b 4096 arquivo.txt destino.txt