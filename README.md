# fastcopy ⚡

Um copiador de arquivos paralelo ultra-rápido desenvolvido em Go. Projetado para superar o desempenho de ferramentas tradicionais como `cp` e `rsync`, aproveitando chamadas de sistema de alto desempenho (como `copy_file_range`, `fallocate` e `fadvise` no Linux).

## Recursos Principais

- **Desempenho Extremo:** Utiliza chamadas de sistema zero-copy, pré-alocação de disco e dicas inteligentes de I/O.
- **Processamento Paralelo:** Despachante (dispatcher) paralelizado, otimizado para lidar de forma eficiente com uma mistura de arquivos pequenos e grandes.
- **Sincronização Incremental:** Ignora automaticamente arquivos não modificados, acelerando muito as atualizações.
- **Indexação e Busca (Novo!):** Crie índices de diretórios para buscas ultrarrápidas (`O(log N)` e `O(1)`) e detecção de arquivos duplicados usando SHA-256.
- **Preservação de Metadados:** Mantém permissões, timestamps (datas de modificação) e outros metadados originais.
- **Interface Dual:** Inclui tanto uma ferramenta de Linha de Comando (CLI) robusta, quanto uma Interface Gráfica (GUI) moderna desenvolvida com o framework [Fyne](https://fyne.io/) contendo abas de Cópia e Busca.

## Pré-requisitos

- [Go](https://go.dev/) instalado.
- Para a interface gráfica (GUI) no Linux, você precisará das bibliotecas de desenvolvimento do X11/OpenGL.

## Guia Rápido

O projeto conta com o utilitário `start.sh` para facilitar todas as operações comuns.

### 1. Instalar dependências da GUI (necessário apenas para a interface gráfica)
```bash
./start.sh deps
```
*Este comando requer `sudo` e detecta automaticamente se você usa `apt`, `dnf` ou `pacman`.*

### 2. Compilar
```bash
# Compilar apenas a CLI
./start.sh build

# Compilar a GUI
./start.sh build-gui

# Compilar ambos
./start.sh build-all
```

### 3. Executar (Operações de Cópia)

**CLI:**
```bash
# Exemplo básico de cópia
./start.sh run /caminho/origem /caminho/destino

# Exemplo avançado com 32 workers paralelos e validação de checksum
./start.sh run -w 32 --checksum /caminho/origem /caminho/destino
```

**GUI:**
```bash
# Abre a interface gráfica com abas de Copiador e Busca de Índice
./start.sh run-gui
```

### 4. Executar (Indexação e Busca CLI)

O `fastcopy` não apenas copia, mas permite varrer diretórios inteiros rapidamente para criar índices estáticos (`.idx`), pesquisar neles ou encontrar duplicatas:

```bash
# 1. Criar um índice do diretório calculando Hashes SHA-256
./start.sh run --index-build --index-hash --index-path=meu_backup.idx /caminho/origem

# 2. Buscar instantaneamente no índice criado
./start.sh run --index-search="*.mp4" --index-path=meu_backup.idx

# 3. Listar todos os arquivos duplicados no índice (baseado em Hash)
./start.sh run --index-dupes --index-path=meu_backup.idx
```

*Nota: Você também pode carregar o arquivo `.idx` gerado diretamente na aba "Index Search" do `fastcopy-gui` para navegar visualmente!*

### 5. Testes
Rodar a suíte de testes de integração, que verifica cópias incrementais, checksums e cópias a seco (dry run):
```bash
./start.sh test
```

## Estrutura do Código

- `cmd/fastcopy/`: Ponto de entrada da aplicação de Linha de Comando (CLI).
- `cmd/fastcopy-gui/`: Ponto de entrada da aplicação Gráfica (GUI) em Fyne.
- `internal/`: Lógica central do copiador, engine paralelo e otimizações zero-copy.
- `internal/index/`: Motor de serialização `gob` puramente em Go para Indexação, Busca Binária e Deduplicação.
- `start.sh`: Script gerenciador de tarefas para desenvolvedores e usuários.

## Licença

Este projeto é licenciado sob a [MIT License](LICENSE).
