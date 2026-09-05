# AGENTS.md

Instruções para agentes de código que trabalham **neste repositório** — ou seja,
para quem está construindo o `deck`, não para quem foi disparado por ele.

O `deck` é um board kanban de terminal em Go: colunas e cards são arquivos
markdown, o TUI é uma view descartável sobre eles, e o board dispara e acompanha
agentes via [herdr](https://herdr.dev).

## Leia primeiro

| arquivo | o que traz |
|---|---|
| [`CLAUDE.md`](CLAUDE.md) | arquitetura, comandos, princípios do projeto, estado |
| [`manual/go-patterns.md`](manual/go-patterns.md) | padrões de Go com exemplos daqui |
| [`manual/`](manual/README.md) | invariantes, modelo de ameaça, o que os sistemas reais ensinaram |
| [`README.md`](README.md) | o que o app faz, do ponto de vista de quem usa |

Não duplique conteúdo entre eles. Regra permanente vai no arquivo certo e é
referenciada dos outros.

## Onde escrever documento

**`manual/` é versionado; `docs/` é local do dono do projeto e está no
`.gitignore`.**

O critério: um agente que chega sem contexto erraria sem este texto? Se sim, vai
para `manual/`. Se não, ou é rascunho (`docs/`), ou não devia ser documento —
comentário na linha que ele explica, ou corpo de commit.

**Decisão arquitetural vira ADR** em [`manual/adr/`](manual/adr/README.md) —
mas só quando você **descartou uma alternativa real** ou **tentou algo que
falhou**. Escolha sem alternativa não é ADR, é comentário no código. Copie o
[template](manual/adr/0000-template.md); no máximo uma tela.

**Não escreva prosa que descreve comportamento.** É isso que envelhece e passa a
mentir. Comportamento vai para teste; o documento aponta para o teste. Se você
está prestes a explicar *o que* o código faz, pare — ou o código não está claro,
ou falta um teste com nome melhor.

**Nunca faça código, teste ou documento versionado depender de `docs/`.** Num
clone limpo aquele diretório não existe. Um comentário que manda ler um arquivo
que não veio é pior que comentário nenhum.

O roadmap do projeto (`docs/HANDOFF.md`) é local pelo mesmo motivo: prioridade é
decisão do dono, não do repositório. Se você o tem à mão, ele diz o que falta e
o que nunca foi validado. Se não tem, o `CLAUDE.md` traz o estado, e trabalhar
sem o roadmap não deve travar nada — quando faltar prioridade, pergunte em vez
de escolher por conta própria.

## Antes de entregar

```sh
go build ./... && go vet ./... && gofmt -l . && go test ./...
go run ./tools/ratchet conferir
```

`gofmt -l .` tem que sair vazio; os quatro têm que passar. Não entregue com teste
quebrado alegando que "não tem a ver com a mudança" — ou você quebrou, ou o teste
estava errado, e nos dois casos é preciso resolver.

A **catraca** recusa achado novo de `golangci-lint` ou `gosec`: o baseline em
`.ci/baseline.txt` só desce. Ela precisa das duas ferramentas no `PATH`:

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
go install github.com/securego/gosec/v2/cmd/gosec@v2.22.9
```

**Não suba o baseline para calar um achado.** Se ele for legítimo, diga por quê
no corpo do commit — o diff do baseline é revisável, e é para isso que ele é um
arquivo e não um número escondido.

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
- README, `CLAUDE.md` e o `manual/` atualizados junto com o código que
  os contradiz
