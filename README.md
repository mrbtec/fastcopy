# fastcopy ⚡

Um copiador de arquivos paralelo ultra-rápido desenvolvido em Go. Projetado para superar o desempenho de ferramentas tradicionais como `cp` e `rsync`, aproveitando chamadas de sistema de alto desempenho (como `copy_file_range`, `fallocate` e `fadvise` no Linux).

## Recursos Principais

- **Desempenho Extremo:** Utiliza chamadas de sistema zero-copy, pré-alocação de disco e dicas inteligentes de I/O.
- **Processamento Paralelo:** Despachante (dispatcher) paralelizado, otimizado para lidar de forma eficiente com uma mistura de arquivos pequenos e grandes.
- **Sincronização Incremental:** Ignora automaticamente arquivos não modificados, acelerando muito as atualizações.
- **Preservação de Metadados:** Mantém permissões, timestamps (datas de modificação) e outros metadados originais.
- **Interface Dual:** Inclui tanto uma ferramenta de Linha de Comando (CLI) rápida e flexível, quanto uma Interface Gráfica de Usuário (GUI) moderna desenvolvida com o framework [Fyne](https://fyne.io/).

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

### 3. Executar
Você pode utilizar o script para compilar e rodar em um único passo:

**CLI:**
```bash
# Exemplo básico
./start.sh run /caminho/origem /caminho/destino

# Exemplo avançado com 32 workers paralelos e validação de checksum
./start.sh run -w 32 --checksum /caminho/origem /caminho/destino
```

**GUI:**
```bash
./start.sh run-gui
```

### 4. Testes
Rodar a suíte de testes de integração, que verifica cópias incrementais, checksums e cópias a seco (dry run):
```bash
./start.sh test
```

## Estrutura do Código
//
- `cmd/fastcopy/`: Ponto de entrada da aplicação de Linha de Comando (CLI).
- `cmd/fastcopy-gui/`: Ponto de entrada da aplicação Gráfica (GUI).
- `internal/`: Lógica central do copiador, onde residem os workers paralelos, as otimizações de I/O e a sincronização.
- `start.sh`: O script gerenciador de tarefas para desenvolvedores e usuários.
