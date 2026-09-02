# AGENTS.md

Instruções para agentes de código que trabalham **neste repositório** — ou seja,
para quem está construindo o `deck`, não para quem foi disparado por ele.

O `deck` é um board kanban de terminal em Go: colunas e cards são arquivos
markdown, o TUI é uma view descartável sobre eles, e o board dispara e acompanha
agentes via [herdr](https://herdr.dev).

## Leia primeiro

| arquivo | o que traz |
|---|---|
| [`HANDOFF.md`](HANDOFF.md) | o que falta, priorizado, e o que nunca foi validado |
| [`CLAUDE.md`](CLAUDE.md) | arquitetura, comandos, princípios do projeto |
| [`docs/go-patterns.md`](docs/go-patterns.md) | padrões de Go com exemplos daqui |
| [`README.md`](README.md) | o que o app faz, do ponto de vista de quem usa |

Não duplique conteúdo entre eles. Regra permanente vai no arquivo certo e é
referenciada dos outros.

## Antes de entregar

```sh
go build ./... && go vet ./... && gofmt -l . && go test ./...
```

`gofmt -l .` tem que sair vazio; os quatro têm que passar. Não entregue com teste
quebrado alegando que "não tem a ver com a mudança" — ou você quebrou, ou o teste
estava errado, e nos dois casos é preciso resolver.

Para mexer em render ou em algo no caminho de cada frame, rode também:

```sh
go test ./internal/ui/ -bench . -benchtime=50x -run XXX
```

## O que este projeto valoriza

**Nada do usuário se perde.** Arquive em vez de apagar; preserve campos de
frontmatter que você não conhece; escreva de forma atômica. Se sua mudança pode
destruir trabalho de alguém, ela está errada — reveja antes de continuar.

**Ação irreversível ou pública pede confirmação**, com o padrão em **não**.
Publicar num PR, arquivar card ou coluna, fechar pane. Valide as pré-condições
*antes* de abrir o diálogo: perguntar e depois falhar desperdiça a decisão.

**Nunca descarte um erro em silêncio.** Num TUI, o pior defeito é a tecla que
parece não fazer nada. Descarte só com comentário dizendo por quê.

**Integração desligada não pode quebrar nada.** Sem `gh`, sem herdr, sem git: o
board funciona igual, só sem aquilo.

**Nada caro na abertura.** Chamada de rede ou subprocesso lento vai para um
`tea.Cmd` em segundo plano, nunca para `New()`.

**Todo comportamento novo entra com teste** — e teste o que você teme, não o que
é fácil de testar.

**Comente a decisão, não a mecânica.** O código já diz o que faz; o comentário
diz por que assim e o que dá errado do outro jeito.

## Cuidados específicos

- **Não invente o JSON de outro programa.** `herdr api schema --json` e
  `gh <cmd> --json <campos>` são a autoridade. Testar contra o sistema real já
  revelou um erro que o schema não pegaria.
- **Nunca construa ID do herdr.** Pane fechado não reutiliza ID; pane movido de
  workspace ganha ID novo. Leia sempre da resposta.
- **`--force` nunca.** Se o herdr ou o git recusam por causa de trabalho não
  commitado, recusar é o comportamento certo.
- **Asserção sobre texto renderizado passa por `plain()`.** O `glamour` intercala
  ANSI entre as palavras.
- **Não capture o TUI num PTY.** O Bubble Tea segura o stdin e trava. Monte o
  `Model`, fixe `width`/`height` e imprima `m.View()`.

## Escopo

Faça o que foi pedido. Se encontrar outro problema pelo caminho, **relate em vez
de consertar por conta própria** — mudança não pedida é ruído no diff e no review.

Se o pedido estiver ambíguo de um jeito que mude o resultado, pergunte. Se der
para decidir com bom senso, decida e diga qual suposição usou.

## Idioma

Comentários, mensagens de erro, texto de UI, documentação e commits em
**português**. Identificadores em inglês.

Mensagem de commit explica **por quê**, não o quê — o diff já mostra o quê.
Registre a alternativa que você descartou e o motivo.

## Contribuindo

Este projeto vai para open source. Ao propor mudança:

- um commit por ideia, com corpo explicando a decisão
- sem dependência nova sem justificativa no corpo do commit
- README, `CLAUDE.md` e `docs/go-patterns.md` atualizados junto com o código que
  os contradiz
