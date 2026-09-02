# deck

Board kanban dentro do terminal, em Go. Colunas e cards são arquivos markdown;
o TUI é uma view descartável sobre eles. Integra com o [herdr](https://herdr.dev)
para disparar e acompanhar agentes, e com o `gh` para estado de PR.

## Comandos

```sh
go build -o ~/.local/bin/deck ./cmd/deck   # instalar
go test ./...                              # todos os testes
go test ./internal/board/ -run TestNome -v # um teste
go vet ./... && gofmt -l .                 # ambos precisam sair limpos
```

Não há Makefile nem linter extra. `go vet` e `gofmt -l .` são o portão.

## Arquitetura

```
cmd/deck/       CLI: init, ls, new, prompt, e o TUI
internal/board/ modelo e persistência — não conhece TUI nem terminal
internal/ui/    Bubble Tea: model, update, view
internal/gh/    wrapper sobre o CLI do GitHub
internal/herdr/ wrapper sobre o CLI do herdr
```

`board` é o núcleo e não importa nada de UI. `ui` orquestra; `gh` e `herdr`
falam com processos externos e não conhecem `board`.

## Princípios que o código segue

Se você for mexer aqui, estas decisões já foram tomadas e têm teste:

**O markdown é a fonte da verdade.** Nada de banco, nada de estado escondido.
Qualquer coisa que o TUI faça tem que dar para fazer editando arquivo à mão.

**Nada do usuário se perde.**
- Campo desconhecido no frontmatter (`jira:`, `assignee:`) sobrevive ao salvar
- Card apontando para coluna inexistente vai para a coluna `?` visível, com
  aviso na barra — nunca some
- Frontmatter sem fechamento vira corpo em vez de derrubar o board
- Escrita atômica: arquivo temporário + rename
- Renomear coluna troca só o título; a key permanece, então nada orfana
- `d` arquiva (move para `.deck/archive/`), não apaga

**Ação irreversível ou pública pede confirmação.** Publicar review no PR (`R`),
arquivar card (`d`) e fechar o pane de um agente (`c`) passam por `modeConfirm`,
cujo padrão é **não**: só `s`/`y`/`enter` seguem adiante.

**A view e o cursor usam a mesma função de ordenação** (`Model.cardsIn`). Se
divergissem, você selecionaria um card e moveria outro. Filtro e "bloqueados no
topo" entram os dois ali.

**Rolagem não guarda offset.** `windowCards` e `windowColumns` são puras: o que
está em foco determina o que aparece. Um offset persistido teria como se
dessincronizar do cursor.

**Um formato só.** Colunas, cards, config e artefatos são todos markdown com
frontmatter, lidos pelo mesmo parser (`internal/board/frontmatter.go`).

**O nome do artefato é a key da coluna que o produziu.** Uma regra, zero
configuração. Um card vira pasta automaticamente na primeira gravação.

**Integração desligada nunca quebra nada.** Sem `gh` autenticado, sem herdr, ou
com `github: off` no config: o board funciona igual, só sem aquilo.

## Falando com processos externos

`gh` e `herdr` são chamados como CLI, não via API. É deliberado: herda a
autenticação que o usuário já tem, funciona com Enterprise sem configuração, e
o deck não guarda credencial nenhuma.

**O schema do herdr é a autoridade.** Rode `herdr api schema --json` para os
formatos; não adivinhe. Envelope é `{id, result}`; erro vai para o stderr como
`{id, error:{code,message}}` com saída 1.

**Nunca construa IDs do herdr.** Pane fechado não reutiliza ID, e pane movido de
workspace ganha ID novo. Leia sempre da resposta.

**O deck só controla o herdr de dentro dele** (`HERDR_ENV=1`). Fora, as ações
avisam em vez de tentar.

## Testes

Todo comportamento novo entra com teste. Os de UI dirigem o `Model` por
`tea.KeyMsg` sem terminal — veja `press` e `typeText` em `internal/ui/ui_test.go`.

**Asserções sobre o corpo do card precisam de `plain()`.** O glamour intercala
ANSI entre as palavras, inclusive nos espaços; `strings.Contains` na saída crua
falha mesmo com o texto na tela.

Para inspecionar visualmente, escreva um teste temporário que monte o `Model`,
fixe `width`/`height` e imprima `m.View()`. Não tente capturar o TUI num PTY —
o Bubble Tea segura o stdin e trava.

## Idioma

Comentários, mensagens de erro, texto de UI e commits em **português**.
Identificadores em inglês.

## Estado atual

Funcional e coberto por teste. **Mas o caminho de agentes e o de PR nunca
rodaram de verdade** — foram escritos contra `herdr api schema` e testes, sem um
herdr vivo nem um PR real. Espere ajustes ao exercitar `s`, `f`, `c` e `R`.
