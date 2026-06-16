# Manual do fastcopy

`fastcopy` é um utilitário de linha de comando para Linux (e outras plataformas suportadas) escrito em Go. Ele foi projetado para copiar arquivos em velocidades altíssimas, superando as limitações de ferramentas tradicionais como `cp` e `rsync`.

## Como Funciona

O `fastcopy` atinge velocidades extremas combinando múltiplas técnicas de ponta:

1. **Paralelismo Avançado**: Em vez de copiar um arquivo por vez de forma serial, o utilitário separa os arquivos em duas filas:
   - **Arquivos Pequenos (< 64MB)**: Processados em altíssimo paralelismo (padrão de `2 * número_de_CPUs`).
   - **Arquivos Grandes (≥ 64MB)**: Processados com limite reduzido para evitar gargalos (saturation) no I/O do disco.
2. **Cópia de Blocos Concorrente**: Arquivos absurdamente grandes (≥ 1GB) não são apenas enviados para uma fila; eles são "fatiados" logicamente (chunks de 100MB) e copiados simultaneamente por diversos workers, turbinando a escrita.
3. **Transferência Zero-Copy (No Kernel)**: No Linux, o utilitário aciona diretamente a chamada de sistema `copy_file_range`, efetuando a transferência diretamente dentro do Kernel, sem que os bytes precisem cruzar as fronteiras para o "espaço do usuário" (userspace). Isso também possibilita cópias instantâneas usando COW (Copy-On-Write) em arquivos de sistema Btrfs/XFS (reflink).
4. **Alocação de Espaço Rápida**: Utiliza a chamada de sistema `fallocate` para garantir blocos contíguos no disco de destino antes de escrever, acabando com a fragmentação e evitando falhas de disco cheio no meio de grandes cópias.
5. **Comunicação Ativa com a Cache (fadvise)**: Usa o `posix_fadvise` para preparar o cache de leitura e descartar os arquivos da memória assim que copiados (`FADV_DONTNEED`). O seu servidor/desktop não ficará mais lento e travando porque o Linux encheu 100% da sua RAM com cache de leitura enquanto transferia 50GB.
6. **Modo Incremental Altamente Otimizado**: Escaneia a árvore inteira e ignora arquivos que já existem no destino com o exato mesmo tamanho e data de modificação (`mtime`). Links simbólicos já existentes no destino são detectados preventivamente e ignorados.

---

## Como Instalar e Compilar

### Requisitos

*   Linguagem Go (versão 1.21 ou superior).
*   Ambiente preferencial: **Linux**. Para Windows/macOS, o código entra no modo "fallback" (usa buffers normais e falha graciosamente as chamadas de sistema exclusivas do kernel Linux).

### Compilação
```bash
cd /caminho/do/seu/repositorio/fastcopy
go build -o fastcopy ./cmd/fastcopy/
```

Para tornar o utilitário acessível de qualquer lugar do sistema:
```bash
sudo mv fastcopy /usr/local/bin/
# ou
go install ./cmd/fastcopy/
```

### Compilação da Interface Gráfica (Fyne)

A GUI requer dependências de desenvolvimento X11/OpenGL para compilar:

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

# Compilar a GUI
go build -o fastcopy-gui ./cmd/fastcopy-gui/
```

---

## Como Usar

O uso mais básico é semelhante a rodar um `cp -a`:

```bash
fastcopy /caminho/da/origem /caminho/do/destino
```
*   Diferente do `cp`, o comportamento padrão é **recursivo**.
*   A barra de progresso em tempo real será exibida com tempo estimado e taxa de cópia (MB/s ou GB/s).

### Opções e Flags Disponíveis

O utilitário aceita os seguintes parâmetros de configuração:

| Flag / Parâmetro | Tipo | Padrão | Descrição |
| :--- | :--- | :--- | :--- |
| `-w N` | Inteiro | `NumCPU * 2` | Define o número máximo de "workers" paralelos. Aumente se estiver usando HDDs com alto nível de NCQ, ou diminua se houver muito gargalo. |
| `--checksum` | Booleano | `false` | Calcula a integridade de dados via hash **SHA256** _enquanto_ o arquivo transita no canal, gerando um relatório no final (sem impacto de double-read). |
| `--dry-run` | Booleano | `false` | Apenas varre os diretórios e lista no terminal o que seria executado (útil para auditoria ou teste de incremental). |
| `--force` | Booleano | `false` | Ignora a checagem inteligente de incremental, forçando a leitura e sobrescrita de **todos os dados e arquivos**. |
| `--no-archive` | Booleano | `false` | Impede que o utilitário preserve as permissões, datas (`mtime`/`atime`) e proprietário (ownership) dos arquivos de origem. |
| `--quiet` | Booleano | `false` | Remove a barra de progresso, imprimindo texto apenas no final ou apenas caso dê erro (perfeito para scripts CI/CD e automação). |
| `--version` | Booleano | `false` | Mostra a versão do sistema e encerra. |

### Exemplos Práticos Avançados

1.  **Forçando 64 processos simultâneos e gerando hashes verificáveis SHA256**:
    ```bash
    fastcopy -w 64 --checksum /mnt/servidor/dados /dados_locais/backup
    ```

2.  **Verificando o que iria ser copiado hoje (modo Simulação Incremental)**:
    ```bash
    fastcopy --dry-run /dados/ativos /arquivos/historicos
    ```

3.  **Realizando um backup para rodar silencioso em Background pelo Cron**:
    ```bash
    fastcopy --quiet /var/log /backup/log
    ```

---

## Solução de Problemas Comuns

### 1. `non-root users can't change ownership` (Erro ao mudar proprietário)
Ao copiar com o modo "archive" (o padrão) ativado, o sistema tentará replicar quem é o dono do arquivo (`UID`/`GID`). Se você não estiver usando o usuário `root`, o Linux irá negar essa operação e você pode ver a mensagem de log (embora o utilitário tente continuar o trabalho graciosamente). Execute o utilitário com `sudo` se houver real necessidade de replicar permissões alheias.

### 2. Copiando para um disco em rede (NFS / Samba)
Sistemas que enviam dados para redes (`nfs`, `cifs`, etc.) não vão conseguir utilizar certas lógicas mágicas de Zero-Copy via `copy_file_range` ou pre-alocação (`fallocate`). O utilitário está configurado para "falhar com graciosidade" e voltar aos buffers padrões de leitura (`io.Copy`), mantendo sua alta velocidade de concorrência mesmo se o sistema de destino não suportar.

### 3. A velocidade mostrada na barra de progresso abaixou repentinamente
Quando os workers iniciam o trabalho na "Fila de arquivos Gigantes", o impacto de IO na placa mãe e no HD dispara. Se o número de workers em paralelo for alto (em discos mecânicos SATA, por exemplo), as agulhas do HD causarão muito Seek, travando as taxas momentaneamente. Tente abaixar a thread count para `-w 4` ou `-w 8` se estiver lidando exclusivamente com HDDs lentos.
