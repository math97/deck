---
adr: 0002
titulo: gh e herdr são chamados como CLI, não pela API
status: aceita
data: 2026-06-15
codigo: internal/gh/gh.go, internal/herdr/herdr.go:run
verificar: go test ./internal/gh/ ./internal/herdr/
---

# ADR-0002 — `gh` e `herdr` são chamados como CLI, não pela API

## Decisão

Toda conversa com GitHub e com o herdr passa por `exec.Command` no binário
deles. O deck não fala HTTP com ninguém e não guarda credencial nenhuma.

## Contexto

O deck precisa de estado de PR, de publicar comentário e de controlar panes e
agentes. Os dois sistemas oferecem API e CLI.

## Alternativas descartadas

- **Cliente HTTP do GitHub (`go-github` + token).** Exigiria que o deck pedisse,
  guardasse e renovasse um token — um segredo a mais no mundo, e uma tela de
  configuração antes do primeiro uso. Não funcionaria com GitHub Enterprise sem
  configuração extra de host.
- **API HTTP do herdr.** Mesmo problema de autenticação, mais um acoplamento de
  versão de protocolo. O CLI já é a interface que o herdr versiona e documenta
  (`herdr api schema --json`).

## Consequências

Ganha-se a autenticação que o usuário já tem no terminal, Enterprise sem
configuração, e zero credencial no deck.

O preço: **saída de outro programa é contrato frágil**. Custou dois bugs reais
— `agent read` devolve texto puro e não o envelope JSON que todo o resto
devolve, e o `gh` canonicaliza a URL de uma issue que na verdade é PR. Nenhum
dos dois apareceria contra uma API tipada, e nenhum dos dois foi pego pelo
schema: só por rodar contra o sistema real.

Também paga-se latência de processo por chamada, o que empurrou toda consulta
para `tea.Cmd` em segundo plano — ver `New()` e `gh.Authenticated`.

Daí duas regras que não são negociáveis: **não invente o JSON de outro
programa** (`herdr api schema --json` e `gh --json` são a autoridade), e
**nunca construa ID do herdr** — pane fechado não reutiliza ID.

## Onde vive

`internal/herdr/herdr.go:run` — envelope e códigos de erro.
`internal/gh/gh.go:PostComment` — a única escrita, coberta por
`TestLivePostReview` (atrás de `DECK_LIVE_PR`, publica de verdade).
