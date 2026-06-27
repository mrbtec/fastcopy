# Opções de Criptografia para o Projeto gocopy - GPT OSS 120 (HF)

Este documento descreve possíveis abordagens para **criptografar** e **descriptografar** arquivos dentro do projeto **gocopy** (implementado em Go). Ele serve como referência para quem quiser estender o utilitário com suporte a criptografia, mantendo a performance e a compatibilidade com os demais módulos.

---

## 1. Por que adicionar criptografia?
- **Proteção de dados sensíveis** ao copiar arquivos entre máquinas ou armazenar backups.
- **Conformidade** com políticas de segurança (ex.: GDPR, LGPD).
- **Integridade**: combinar criptografia com checksums já existentes (SHA‑256) para detectar alterações.

## 2. Bibliotecas nativas do Go
O Go já inclui pacotes robustos no módulo padrão (`crypto`). Eles são bem testados e não exigem dependências externas.

| Biblioteca | Algoritmo | Uso típico | Comentários |
|------------|-----------|------------|-------------|
| `crypto/aes` | AES‑CBC, AES‑GCM | Criptografia simétrica de blocos. | AES‑GCM fornece confidencialidade + integridade (AEAD). |
| `crypto/cipher` | Interfaces de bloco e stream | Construir modos de operação (CBC, CTR, GCM). | Necessário para combinar com `aes`. |
| `crypto/sha256` | SHA‑256 | Já usado para checksums. | Pode ser usado como HMAC para autenticação. |
| `crypto/hmac` | HMAC‑SHA256 | Autenticação de mensagens. | Combine com chave secreta para garantir integridade. |
| `crypto/rand` | Gerador de números aleatórios seguros | Geração de IVs/nonce. | Sempre use `rand.Reader`. |
| `crypto/rsa` | RSA (OAEP, PKCS#1 v1.5) | Criptografia assimétrica. | Ideal para troca de chaves, mas mais lento. |
| `golang.org/x/crypto/chacha20poly1305` | ChaCha20‑Poly1305 | AEAD de alta performance. | Boa escolha quando hardware AES não está disponível. |

## 3. Estratégia recomendada (simétrica)
1. **Gerar uma chave secreta** (32 bytes para AES‑256) usando `crypto/rand`.
2. **Derivar um nonce/IV** aleatório para cada arquivo (12 bytes para GCM).
3. **Criptografar** com `cipher.NewGCM(aes.NewCipher(key))`.
4. **Persistir** o nonce junto ao início do arquivo criptografado (ex.: `[nonce][ciphertext]`).
5. **Descriptografar** lendo o nonce, inicializando o GCM e chamando `Open`.

### Exemplo de código (esboço)
```go
package cryptoutil

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "io"
    "os"
)

// EncryptFile encrypts srcPath and writes the result to dstPath.
func EncryptFile(srcPath, dstPath string, key []byte) error {
    // 1. Open source file
    in, err := os.Open(srcPath)
    if err != nil { return err }
    defer in.Close()

    // 2. Create destination file
    out, err := os.Create(dstPath)
    if err != nil { return err }
    defer out.Close()

    // 3. Create AES‑GCM cipher
    block, err := aes.NewCipher(key)
    if err != nil { return err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return err }

    // 4. Generate nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err = io.ReadFull(rand.Reader, nonce); err != nil { return err }
    // 5. Write nonce (precisa ser lido na descriptografia)
    if _, err = out.Write(nonce); err != nil { return err }

    // 6. Stream encryption (leitura em blocos)
    buf := make([]byte, 64*1024) // 64 KB
    for {
        n, rerr := in.Read(buf)
        if n > 0 {
            ciphertext := gcm.Seal(nil, nonce, buf[:n], nil)
            if _, err = out.Write(ciphertext); err != nil { return err }
        }
        if rerr == io.EOF { break }
        if rerr != nil { return rerr }
    }
    return nil
}

// DecryptFile reverses the process.
func DecryptFile(srcPath, dstPath string, key []byte) error {
    in, err := os.Open(srcPath)
    if err != nil { return err }
    defer in.Close()

    out, err := os.Create(dstPath)
    if err != nil { return err }
    defer out.Close()

    block, err := aes.NewCipher(key)
    if err != nil { return err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return err }

    // 1. Read nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err = io.ReadFull(in, nonce); err != nil { return err }

    // 2. Decrypt stream
    buf := make([]byte, 64*1024+gcm.Overhead())
    for {
        n, rerr := in.Read(buf)
        if n > 0 {
            plaintext, err := gcm.Open(nil, nonce, buf[:n], nil)
            if err != nil { return err }
            if _, err = out.Write(plaintext); err != nil { return err }
        }
        if rerr == io.EOF { break }
        if rerr != nil { return rerr }
    }
    return nil
}
```

> **Nota:** O código acima é um ponto de partida. Em produção, considere:
> - **Autenticação** adicional (HMAC) para proteger contra modificações.
> - **Rotação de chaves**.
> - **Armazenamento seguro** da chave (ex.: `keyring`, variáveis de ambiente, ou um cofre).

## 4. Estratégia alternativa (assimétrica)
- Use RSA‑OAEP para criptografar a chave simétrica (AES) e armazená‑la no cabeçalho do arquivo.
- Benefício: a chave pode ser distribuída sem compartilhamento prévio.
- Desvantagem: custo computacional maior; adequado apenas para arquivos pequenos ou troca de chaves.

## 5. Integração com o projeto existente
1. **Adicionar novo pacote** `fastcopy/internal/crypto` contendo as funções acima.
2. **Expor flags** no binário principal (`fastcopy` e `fastcopy-gui`):
   - `--encrypt <keyfile>` – criptografa antes de copiar.
   - `--decrypt <keyfile>` – descriptografa após copiar.
3. **Atualizar a CLI** (`cmd/fastcopy/main.go`) para aceitar as novas opções e chamar o pacote.
4. **Testes**: criar testes unitários em `fastcopy/internal/crypto/crypto_test.go` usando arquivos temporários.
5. **Documentação**: atualizar `README.md` e `MANUAL.md` com exemplos de uso.

## 6. Considerações de desempenho
- **AES‑GCM** tem desempenho próximo ao da cópia pura (≈ 1 GB/s em CPUs modernas).
- **ChaCha20‑Poly1305** pode ser mais rápido em CPUs sem instruções AES.
- **Chunking**: reutilizar a lógica de `chunk_copy.go` para processar arquivos em blocos, evitando alocação de memória excessiva.

## 7. Segurança
- Nunca reutilize o mesmo nonce/IV com a mesma chave.
- Use `crypto/rand` para gerar chaves/nonce.
- Proteja a chave em repouso (ex.: `chmod 600` no arquivo de chave).
- Considere usar **libsodium** via `golang.org/x/crypto/nacl` se precisar de primitives de alto nível.

---

### Próximos passos sugeridos
1. Criar o pacote `fastcopy/internal/crypto` com as funções de exemplo.
2. Implementar flags de linha de comando.
3. Escrever testes de integração que copiem um arquivo, criptografando‑o e depois verificando a integridade.
4. Atualizar a documentação do projeto.

Com essas opções, o **gocopy** pode oferecer criptografia forte sem sacrificar a velocidade da cópia.


