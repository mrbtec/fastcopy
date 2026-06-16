# Análise de Viabilidade: Interface Gráfica para o fastcopy

Este documento analisa as opções disponíveis no ecossistema Go para criar uma interface gráfica (GUI) para o `fastcopy`, avaliando prós, contras e a recomendação final.

---

## Contexto

O `fastcopy` é um utilitário de cópia de arquivos de alta performance, escrito em Go, que opera atualmente via CLI. A GUI ideal deve:

- Permitir selecionar diretórios de origem e destino.
- Exibir uma barra de progresso em tempo real (bytes/s, ETA, arquivos copiados).
- Oferecer toggles para as opções existentes (checksum, force, archive).
- **Não degradar a performance** do motor de cópia (a GUI deve ser leve).

---

## Frameworks Avaliados

### 1. Fyne (`fyne.io/fyne/v2`)

| Critério | Avaliação |
| :--- | :--- |
| **Maturidade** | ⭐⭐⭐⭐⭐ — Framework mais maduro do ecossistema Go para GUI desktop |
| **Cross-platform** | ✅ Linux, macOS, Windows, Android, iOS — código único |
| **Visual** | Material Design próprio ("Fyne Design") — não é nativo do OS |
| **CGO** | ⚠️ **Requer CGO** — dependência de drivers gráficos nativos (OpenGL/Vulkan) |
| **Compilação** | ⚠️ Primeira compilação lenta (~1-2 min), mas incrementais são rápidas |
| **Widgets prontos** | ✅ FileDialog, ProgressBar, DataBinding, CheckBox, Entry — tudo que precisamos |
| **Goroutines** | ✅ `fyne.Do()` para atualizar UI de goroutines background (thread-safe desde v2.6) |
| **Peso do binário** | ~15-25 MB (inclui stack gráfico completo) |

**Viabilidade para fastcopy**: ✅ **Alta**

A integração seria direta: o motor de cópia (`internal/copier.go`) já emite progresso via `Progress.AddCopiedFile()` com contadores atômicos. Bastaria o frontend Fyne ler esses contadores a cada 100ms via `fyne.Do()` e atualizar um `widget.ProgressBar`.

**Exemplo conceitual de integração**:
```go
progressBar := widget.NewProgressBar()
go func() {
    engine.Run(src, dst)
    for !done {
        fyne.Do(func() {
            progressBar.SetValue(float64(progress.copiedBytes) / float64(progress.totalBytes))
        })
        time.Sleep(100 * time.Millisecond)
    }
}()
```

---

### 2. Wails (`wails.io`)

| Critério | Avaliação |
| :--- | :--- |
| **Maturidade** | ⭐⭐⭐⭐ — Muito popular, mas v3 ainda em alpha |
| **Cross-platform** | ✅ Linux, macOS, Windows |
| **Visual** | 🎨 **Excelente** — Frontend é HTML/CSS/JS (React, Vue, Svelte) |
| **CGO** | ⚠️ Requer CGO (WebView nativo do OS) |
| **Compilação** | ⚠️ Requer Node.js + npm no build pipeline |
| **Widgets prontos** | ✅ Todo o ecossistema Web (Tailwind, Shadcn, etc.) |
| **Goroutines** | ✅ Events system (`EventsEmit`) para comunicação Go ↔ JS |
| **Peso do binário** | ~10-20 MB (usa WebView nativo, não embute Chromium) |

**Viabilidade para fastcopy**: ✅ **Alta, mas overengineered**

A interface ficaria visualmente impecável (é HTML/CSS), porém introduz duas camadas de complexidade desnecessárias:
1. Dependência de Node.js/npm no processo de build.
2. Camada de IPC (Inter-Process Communication) entre Go e JavaScript via WebView.

Para uma ferramenta focada em **velocidade bruta de I/O**, adicionar uma bridge Go↔JS é um overhead arquitetural que não faz sentido.

---

### 3. Gio (`gioui.org`)

| Critério | Avaliação |
| :--- | :--- |
| **Maturidade** | ⭐⭐⭐ — Ativo, mas ecossistema menor |
| **Cross-platform** | ✅ Linux, macOS, Windows, Android, iOS, WASM |
| **Visual** | Custom — estilo próprio, minimalista |
| **CGO** | ✅ **Não requer CGO** — pure Go! |
| **Compilação** | ✅ Rápida (sem CGO) |
| **Widgets prontos** | ⚠️ **Poucos** — modo imediato (immediate-mode), você desenha tudo |
| **Goroutines** | ✅ Funciona bem, mas requer gerenciamento manual de estado |
| **Peso do binário** | ~5-10 MB (mais leve de todos) |

**Viabilidade para fastcopy**: ⚠️ **Média**

Gio é o mais leve e o único que não requer CGO, o que é excelente para portabilidade. Porém, não tem widgets prontos como FileDialog ou ProgressBar — você precisa construir tudo do zero. O esforço de desenvolvimento seria 3-5x maior que com Fyne para chegar ao mesmo resultado.

---

### 4. GTK (via `gotk4`)

| Critério | Avaliação |
| :--- | :--- |
| **Maturidade** | ⭐⭐⭐⭐⭐ — GTK é maduro, mas bindings Go são jovens |
| **Cross-platform** | ⚠️ **Excelente no Linux**, doloroso no Windows/macOS |
| **Visual** | 🎨 **Nativo** — aparência perfeita no GNOME/Linux |
| **CGO** | ⚠️ Requer CGO + dependências GTK4 instaladas no sistema |
| **Compilação** | ⚠️ Requer `pkg-config`, libs GTK4 dev, setup complexo |
| **Widgets prontos** | ✅ Todos os widgets GTK (FileChooser, ProgressBar, etc.) |
| **Goroutines** | ⚠️ Requer `glib.IdleAdd()` para atualizações thread-safe |
| **Peso do binário** | ~2-5 MB (mas depende de libs do sistema: ~50MB+) |

**Viabilidade para fastcopy**: ⚠️ **Média (apenas Linux)**

Seria a escolha perfeita se o alvo fosse exclusivamente Linux com desktop GNOME. Porém, a dificuldade de compilação cruzada e a dependência pesada de libs do sistema (`libgtk-4-dev`) tornam esta opção impraticável para distribuição ampla.

---

### 5. Bubble Tea (`github.com/charmbracelet/bubbletea`)

| Critério | Avaliação |
| :--- | :--- |
| **Tipo** | ⚠️ **TUI (Terminal UI)** — não é GUI gráfica |
| **Maturidade** | ⭐⭐⭐⭐⭐ — Padrão-ouro para interfaces de terminal em Go |
| **Cross-platform** | ✅ Qualquer terminal |
| **CGO** | ✅ **Não requer CGO** — pure Go |
| **Visual** | 🎨 Belíssimo para terminal (cores, bordas, animações) |

**Viabilidade para fastcopy**: ✅ **Alta como alternativa intermediária**

Bubble Tea não cria janelas gráficas, mas pode transformar a CLI atual em uma experiência de terminal rica e interativa, com barras de progresso animadas, seleção de diretórios via navegação, e painéis coloridos — tudo sem nenhuma dependência externa.

---

## Tabela Comparativa Final

| Framework | Viabilidade | CGO | Esforço Dev | Visual | Peso Binário | Recomendação |
| :--- | :---: | :---: | :---: | :---: | :---: | :--- |
| **Fyne** | ✅ Alta | Sim | Baixo | Bom | ~20 MB | **🏆 Recomendado** |
| **Wails** | ✅ Alta | Sim | Médio | Excelente | ~15 MB | Overengineered para o caso |
| **Gio** | ⚠️ Média | Não | Alto | Custom | ~8 MB | Bom se CGO for proibido |
| **GTK** | ⚠️ Média | Sim | Médio | Nativo | ~3 MB* | Apenas para Linux |
| **Bubble Tea** | ✅ Alta | Não | Baixo | Terminal | ~5 MB | Alternativa TUI excelente |

> **\*** O binário GTK é pequeno, mas depende de ~50MB+ de bibliotecas do sistema.

---

## Conclusão e Recomendação

### 🏆 Recomendação Principal: **Fyne**

**Fyne é a escolha mais viável** para criar a GUI do `fastcopy` pelos seguintes motivos:

1. **Integração natural com Go**: O motor do `fastcopy` já usa goroutines e contadores atômicos — o modelo `fyne.Do()` se encaixa perfeitamente.
2. **Widgets prontos**: `dialog.ShowFolderOpen()`, `widget.ProgressBar`, `widget.Check` — tudo que precisamos já existe.
3. **Cross-platform real**: Um único binário roda em Linux, macOS e Windows.
4. **Esforço moderado**: Estimativa de ~200-300 linhas de código Go para a GUI completa.

### 🥈 Alternativa: **Bubble Tea (TUI)**

Se a prioridade for **zero dependências externas** e **máxima portabilidade** (sem CGO, sem drivers gráficos), Bubble Tea oferece uma experiência visual rica no terminal com esforço mínimo de desenvolvimento. Ideal para servidores headless.

### Abordagem sugerida: **Dual-mode**

A implementação ideal seria manter ambos os modos:
- `fastcopy src dst` — modo CLI atual (sem mudanças).
- `fastcopy --gui` — abre a interface Fyne.
- `fastcopy --tui` — abre a interface Bubble Tea (interativa no terminal).

Isso mantém a compatibilidade com scripts existentes enquanto adiciona acessibilidade para usuários que preferem interfaces visuais.
