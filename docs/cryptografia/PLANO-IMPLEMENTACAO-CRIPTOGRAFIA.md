# Plano de Implementação de Criptografia para gocopy

Com base na análise do documento `OPCOES-CRIPTOGRAFIA.md` e da **arquitetura real do projeto**, foi estruturado o plano de ação abaixo. Mudanças significativas foram feitas em relação à versão anterior para corrigir problemas técnicos e alinhar com o código existente.

---

## ⚠️ Problemas Identificados no Plano Anterior

### 1. Bug Crítico: Reutilização de Nonce no Código de Referência
O esboço em `OPCOES-CRIPTOGRAFIA.md` usa o **mesmo nonce para todos os chunks** dentro de um arquivo. Isso é uma **vulnerabilidade fatal** no AES-GCM: reutilizar um nonce com a mesma chave permite ao atacante recuperar a chave de autenticação e decifrar os dados. **Cada chunk deve ter seu próprio nonce único** (ou usar um contador incremental derivado do nonce base).

### 2. Incompatibilidade com Zero-Copy
O motor de cópia do fastcopy usa `copy_file_range` (zero-copy via kernel em Linux) — os dados **nunca transitam pelo userspace**. A criptografia, por definição, precisa processar os dados em userspace. Portanto, o fluxo criptografado **deve desabilitar automaticamente o zero-copy** e usar o fallback (`fallbackCopy`) ou uma pipeline dedicada.

### 3. Incompatibilidade com Cópia Concorrente por Chunks
O `concurrentCopy` em `chunk_copy.go` usa `WriteAt`/`SectionReader` com offsets absolutos. AES-GCM introduz **overhead por chunk** (tag de 16 bytes + nonce), o que altera os offsets de escrita no destino em relação à leitura na origem. A cópia concorrente com criptografia exigiria um mapeamento de offsets complexo. **Na primeira versão, a criptografia deve usar cópia sequencial.**

### 4. Tamanho de Buffer Desalinhado
O `OPCOES-CRIPTOGRAFIA.md` sugere chunks de 64KB, mas o projeto usa buffers de **4MB** (`bufSize` em `filecopy.go`) via `sync.Pool`. O pacote de criptografia deve reutilizar o `bufPool` existente para evitar duplicação de alocações e pressão no GC.

### 5. Limite de Tamanho do GCM
AES-GCM tem um limite prático de ~64GB por nonce (2³² blocos de 16 bytes). Para arquivos maiores, cada chunk deve ser tratado como uma mensagem GCM independente, com seu próprio nonce.

---

## Fase 1: Criação do Pacote `fastcopy/internal/crypto`

**Objetivo:** Primitivas criptográficas corretas e seguras.

### 1.1 Formato do Arquivo Cifrado

```
[HEADER: 4 bytes magic "FCRY"]
[VERSION: 1 byte]
[BASE_NONCE: 12 bytes]
[CHUNK_0: nonce_counter(0) → ciphertext + GCM tag (16 bytes)]
[CHUNK_1: nonce_counter(1) → ciphertext + GCM tag (16 bytes)]
...
[CHUNK_N]
```

- **Magic bytes** permitem identificar se um arquivo foi cifrado pelo fastcopy.
- **Nonce por chunk**: derivar nonce incrementando um contador sobre o `BASE_NONCE` (evita o bug de reutilização).
- **Cada chunk** é uma mensagem GCM autônoma (permite detecção de corrupção granular).

### 1.2 Funções Principais

```go
// crypto.go
package crypto

// EncryptStream(reader io.Reader, writer io.Writer, key []byte) error
//   - Escreve header + base_nonce
//   - Lê chunks de 4MB do reader, cifra cada um com nonce incrementado
//   - Escreve [ciphertext+tag] para cada chunk

// DecryptStream(reader io.Reader, writer io.Writer, key []byte) error
//   - Lê header + base_nonce
//   - Lê chunks cifrados (4MB + 16 bytes overhead), decifra com nonce incrementado
//   - Escreve plaintext no writer

// GenerateKey() ([]byte, error)
//   - Gera 32 bytes criptograficamente seguros

// LoadKey(path string) ([]byte, error)
//   - Lê arquivo, valida tamanho (== 32 bytes), retorna chave
```

> **Nota:** Usar `io.Reader`/`io.Writer` em vez de paths permite integração direta com o pipeline de cópia existente (ex: `io.MultiWriter` para checksum + criptografia simultânea).

### 1.3 Testes (`crypto_test.go`)

| Caso de Teste | Validação |
|---|---|
| Encrypt → Decrypt round-trip | Dados idênticos ao original |
| Arquivo vazio | Não deve falhar (edge case) |
| Arquivo > 4MB (multi-chunk) | Chunks processados corretamente |
| Bit flip no ciphertext | `gcm.Open` retorna erro (integridade GCM) |
| Chave incorreta na decifragem | Falha na autenticação |
| Nonce nunca se repete entre chunks | Verificar incremento monotônico |

---

## Fase 2: Integração com o Motor de Cópia

**Objetivo:** Inserir a criptografia no pipeline sem quebrar a arquitetura existente.

### 2.1 Modificações em `filecopy.go`

A função `CopyFile` é o ponto de integração correto. O fluxo com criptografia será:

```
src → [read] → [encrypt/decrypt] → [write] → dst
```

Quando `opts.Encrypt` ou `opts.Decrypt` estiver ativo:
- **Pular** `platformCopyFile` (zero-copy é incompatível).
- **Pular** `concurrentCopy` (offsets desalinhados por overhead GCM).
- Usar `crypto.EncryptStream` ou `crypto.DecryptStream` com os file descriptors diretamente.
- Manter compatibilidade com `--checksum`: usar `io.TeeReader` para alimentar SHA-256 **sobre os dados originais** (não sobre o ciphertext).

### 2.2 Modificações em `Options` (struct)

```go
type Options struct {
    // ... campos existentes ...
    EncryptKey []byte // Se não-nil, cifra durante a cópia
    DecryptKey []byte // Se não-nil, decifra durante a cópia
}
```

### 2.3 `incremental.go` — Sem Mudanças

A lógica incremental compara **tamanho e mtime** do arquivo no destino. Como o arquivo cifrado terá tamanho diferente do original, o comportamento incremental já funcionará corretamente (irá detectar mudança e reccopiar). Não é necessário alterar a lógica.

### 2.4 Uso do `bufPool` Existente

O pacote `crypto` deve importar e usar `getBuf()`/`putBuf()` de `filecopy.go` (ou expor o pool via função pública). Isso garante que não haja dois pools de 4MB competindo por memória.

---

## Fase 3: Exposição na CLI

### 3.1 Novas Flags em `cmd/fastcopy/main.go`

```go
encryptKey := flag.String("encrypt", "", "path to 32-byte key file for AES-256-GCM encryption")
decryptKey := flag.String("decrypt", "", "path to 32-byte key file for AES-256-GCM decryption")
genKey     := flag.String("gen-key", "", "generate a new random key and save to this path, then exit")
```

**Validações obrigatórias:**
- `--encrypt` e `--decrypt` são mutuamente exclusivos.
- O arquivo de chave deve existir e conter exatamente 32 bytes.
- Exibir aviso se as permissões do keyfile forem mais abertas que `0600`.

### 3.2 Comando `--gen-key`

Adicionar uma subação que gera uma chave segura e sai:

```bash
fastcopy --gen-key /caminho/minha_chave.bin
# Gera 32 bytes com crypto/rand, salva, define chmod 600
```

Isso elimina a dependência do `openssl` para o usuário.

### 3.3 Feedback ao Usuário

- Na saída padrão, exibir `🔒 Encryption enabled (AES-256-GCM)` quando ativo.
- No summary final, mostrar overhead estimado (tempo adicional vs cópia pura).

---

## Fase 4: Documentação

1. **README.md** — seção "Criptografia" com exemplos rápidos.
2. **MANUAL.md** — detalhes sobre formato do arquivo cifrado, compatibilidade, limitações.
3. **Boas práticas:**
   - Chave: `chmod 600`, backup seguro.
   - Perda da chave = perda definitiva dos dados.
   - Não usar `--encrypt` + `--checksum` se o checksum for comparado depois (o arquivo destino é cifrado, checksum difere).

---

## Ordem de Execução Recomendada

| Etapa | Arquivos | Dependência |
|-------|----------|-------------|
| 1 | `internal/crypto/crypto.go` | Nenhuma |
| 2 | `internal/crypto/crypto_test.go` | Etapa 1 |
| 3 | `internal/filecopy.go` (Options + integração) | Etapa 1 |
| 4 | `cmd/fastcopy/main.go` (flags) | Etapa 3 |
| 5 | Testes de integração end-to-end | Etapas 1-4 |
| 6 | Documentação | Etapa 5 |

---

## Riscos e Mitigações

| Risco | Impacto | Mitigação |
|-------|---------|-----------|
| Nonce reuse | Quebra total da segurança | Contador monotônico por chunk + nonce base único por arquivo |
| Arquivo parcialmente cifrado (crash) | Dados irrecuperáveis | Escrever em arquivo temporário, renomear no final (atomic write) |
| Performance degradada em arquivos enormes | Lentidão vs cópia pura | Benchmark obrigatório; aceitar que criptografia sequencial é ~30-50% mais lento que zero-copy |
| Chave vazada na memória após uso | Exposição em core dump | Zerar `[]byte` da chave com loop após uso; Go não garante isso pelo GC |
