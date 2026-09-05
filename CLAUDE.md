# deck

Board kanban dentro do terminal, em Go. Colunas e cards são arquivos markdown;
o TUI é uma view descartável sobre eles. Integra com o [herdr](https://herdr.dev)
para disparar e acompanhar agentes, e com o `gh` para estado de PR.

**Este arquivo é mapa, não manual.** Ele diz onde procurar. O *porquê* de cada
decisão está nos [ADRs](manual/adr/README.md); o *quê* está no código e no
teste, que são a fonte da verdade quando divergirem daqui.

## Comandos

```sh
go build -o ~/.local/bin/deck ./cmd/deck   # instalar
go test ./...                              # todos os testes
go test ./internal/board/ -run TestNome -v # um teste
go vet ./... && gofmt -l .                 # ambos precisam sair limpos
```

Não há Makefile nem linter extra. `go vet` e `gofmt -l .` são o portão.

## Onde está cada coisa

```
cmd/deck/       CLI: init, ls, new, prompt, e o TUI
internal/board/ modelo e persistência — não conhece TUI nem terminal
internal/ui/    Bubble Tea: model, update, view
internal/gh/    wrapper sobre o CLI do GitHub
internal/skill/ localiza skills do Claude Code para usar como prompt
internal/herdr/ wrapper sobre o CLI do herdr
```

`board` é o núcleo e não importa nada de UI. `ui` orquestra; `gh` e `herdr`
falam com processos externos e não conhecem `board`.

| quero mexer em… | comece por | ADR |
|---|---|---|
| leitura/escrita de arquivo do board | `board/frontmatter.go`, `board/store.go:Save` | [0001](manual/adr/0001-markdown-e-a-fonte-da-verdade.md) |
| tecla nova, fluxo de tecla | `ui/update.go:Update` | — |
| o que aparece na tela | `ui/view.go`, `ui/render.go` | — |
| ordenação, filtro, cursor | `ui/model.go:cardsIn` — **a mesma para view e cursor** | [0001](manual/adr/0001-markdown-e-a-fonte-da-verdade.md) |
| rolagem | `ui/scroll.go:windowCards`, `windowColumns` — puras, sem offset | — |
| confirmação de ação | `ui/update.go`, `modeConfirm` — padrão **não** | — |
| disparar agente | `ui/agents.go:startAgent` | [0003](manual/adr/0003-sem-api-de-modelo.md), [0004](manual/adr/0004-agente-nao-pronto-esta-vivo.md) |
| capturar resultado do agente | `ui/capture.go`, `ui/update.go:detectFinished` | [0004](manual/adr/0004-agente-nao-pronto-esta-vivo.md) |
| estado de PR, publicar review | `gh/gh.go`, `ui/review.go` | [0002](manual/adr/0002-cli-em-vez-de-api.md) |
| chamar o herdr | `herdr/herdr.go:run` | [0002](manual/adr/0002-cli-em-vez-de-api.md) |
| worktree por card | `ui/agents.go`, `herdr worktree create` | — |
| skills como prompt | `skill/skill.go`, `board.Skills` (injetada pelo `cmd`) | — |

## Documentos

| onde | o que | versionado? |
|---|---|---|
| [`manual/adr/`](manual/adr/README.md) | **por que X e não Y**, o que falhou | sim |
| [`manual/regras.md`](manual/regras.md) | 16 invariantes, cada uma com teste | sim |
| [`manual/go-patterns.md`](manual/go-patterns.md) | padrões de Go com exemplos daqui | sim |
| [`manual/security.md`](manual/security.md) | modelo de ameaça e mitigações | sim |
| [`AGENTS.md`](AGENTS.md) | como trabalhar neste repo | sim |
| `docs/` | roadmap e rascunho do dono do projeto | **não** |

**Nada versionado pode depender de `docs/`** — num clone limpo ele não existe.

Documento novo: se um agente sem contexto erraria sem ele, vai para `manual/`;
se registra uma decisão com alternativa descartada, vai para `manual/adr/`. Se
não é nenhum dos dois, vira comentário na linha que explica, ou corpo de commit.

## O que vale em toda seção

Isto se aplica a qualquer mudança, em qualquer pacote.

**Nada do usuário se perde.** Campo desconhecido no frontmatter sobrevive ao
salvar; card órfão vai para a coluna `?` visível; frontmatter sem fechamento
vira corpo em vez de derrubar o board; escrita é atômica (temp + rename); `d`
arquiva em `.deck/archive/`, não apaga. Provado em `board/regras_test.go`.

**Ação irreversível ou pública pede confirmação**, com padrão em **não**: `R`
(publica no PR), `d` (arquiva), `c` (fecha pane). Valide as pré-condições
*antes* de abrir o diálogo.

**Nada caro na abertura.** `gh auth status` custa ~300ms. `New()` usa só
`gh.Installed()` (LookPath); o resto vai para `tea.Cmd` em segundo plano.

**Integração desligada nunca quebra nada.** Sem `gh`, sem herdr, ou com
`github: off`: o board funciona igual, só sem aquilo. Vale também para
integração *quebrada* — URL de PR errada vira badge `PR ?`, não erro.

**Não invente o JSON de outro programa.** `herdr api schema --json` e
`gh <cmd> --json` são a autoridade. E **nunca construa ID do herdr**: pane
fechado não reutiliza ID.

**Um formato só.** Colunas, cards, config e artefatos são markdown com
frontmatter, no mesmo parser. O nome do artefato é a key da coluna que o
produziu — uma regra, zero configuração.

**Idioma:** comentários, erros, texto de UI e commits em **português**;
identificadores em inglês.

## Testes

Todo comportamento novo entra com teste. Os de UI dirigem o `Model` por
`tea.KeyMsg` sem terminal — veja `press` e `typeText` em `internal/ui/ui_test.go`.

- **`plain()` nas asserções sobre texto renderizado** — o glamour intercala ANSI
  entre as palavras, inclusive nos espaços.
- **Não capture o TUI num PTY** — o Bubble Tea segura o stdin e trava. Monte o
  `Model`, fixe `width`/`height`, imprima `m.View()`.
- **Benchmarks de frame:** `go test ./internal/ui/ -bench . -benchtime=50x -run XXX`.
- **Teste que escreve em sistema externo fica atrás de variável de ambiente.**
  `TestLivePostReview` publica num PR de verdade e só roda com `DECK_LIVE_PR`.
  A suíte normal nunca publica nada.

## Estado atual

Funcional e coberto por teste. Todo caminho que fala com sistema externo já foi
exercitado contra o sistema real — o herdr rendeu três correções, o `gh pr
comment` passou sem nenhuma. O que se aprendeu virou
[ADR-0002](manual/adr/0002-cli-em-vez-de-api.md) e
[ADR-0004](manual/adr/0004-agente-nao-pronto-esta-vivo.md).

**O que ainda nunca rodou de verdade é o ciclo do agente fechando dentro de um
pane** — o agente terminar, o deck capturar o artefato e registrar no `## Log`.
Espere ajustes ao exercitar isso.

O backlog priorizado mora em `docs/HANDOFF.md`, que é local. Sem ele, o que está
acima é o suficiente; quando faltar prioridade, pergunte em vez de escolher.
