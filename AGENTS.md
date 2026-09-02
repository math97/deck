# AGENTS.md

Instruções para agentes de código trabalhando neste repositório.

O `deck` é um board kanban de terminal em Go, cujas colunas e cards são arquivos
markdown. Ele existe para orquestrar agentes — o que significa que **você
provavelmente foi disparado por ele**, com o prompt de uma coluna e o caminho de
um card.

## Leia primeiro

| arquivo | o que traz |
|---|---|
| [`CLAUDE.md`](CLAUDE.md) | arquitetura, comandos, princípios do projeto |
| [`docs/go-patterns.md`](docs/go-patterns.md) | padrões de Go com exemplos daqui |
| [`README.md`](README.md) | o que o app faz, do ponto de vista de quem usa |

Não repita aqui o que está lá. Se você for adicionar uma regra permanente,
coloque no arquivo certo e referencie.

## Antes de entregar

```sh
go build ./... && go vet ./... && gofmt -l . && go test ./...
```

`gofmt -l .` tem que sair vazio. Os quatro têm que passar. Não entregue com teste
quebrado dizendo que "não tem a ver com a mudança" — se quebrou, ou você quebrou,
ou o teste estava errado; nos dois casos é preciso resolver.

## Regras de trabalho

**Todo comportamento novo entra com teste.** Teste o que você teme, não o que é
fácil de testar. Veja a seção de testes em `docs/go-patterns.md`.

**Não invente formato de JSON de outro programa.** `herdr api schema --json` e
`gh <cmd> --json <campos>` são a autoridade. Testar contra o sistema real já
revelou um erro que o schema não pegaria.

**Ação irreversível ou pública pede confirmação.** Publicar num PR, arquivar,
fechar pane. O padrão da confirmação é **não**.

**Nada do usuário se perde.** Arquive em vez de apagar; preserve campos que você
não conhece; escreva de forma atômica. Se sua mudança pode destruir trabalho,
ela está errada.

**Comente a decisão, não a mecânica.** O comentário existe para o que não dá
para deduzir do código: por que assim, e o que dá errado do outro jeito.

**Português** em comentários, mensagens de erro, texto de UI e commits.
Identificadores em inglês.

## Se você foi disparado pelo deck

O prompt que você recebeu traz, no rodapé:

- **onde gravar sua entrega** — um `<coluna>.md` no diretório do card
- **o que já foi produzido** — os artefatos das colunas anteriores

Leia os artefatos anteriores antes de começar. É isso que faz a esteira
funcionar: o refinamento alimenta a implementação, que alimenta o QA. Refazer
uma decisão já tomada no `refine.md` é retrabalho.

**Grave sua entrega no arquivo indicado.** Se você não gravar, o deck salva a
transcrição do terminal como rede de segurança — que é bem pior de ler do que um
markdown que você escreveu de propósito.

**Não altere o frontmatter do card nem a seção `## Log`.** Aqueles campos são o
estado do board; o log é escrito pelo deck. Editá-los quebra a visão do usuário.

Se você está numa worktree (`deck/<card-id>`), ela é sua: commite ali à vontade.
Não volte para a árvore principal.

## Escopo

Faça o que foi pedido. Se encontrar outro problema pelo caminho, **relate em vez
de consertar por conta própria** — uma mudança que o usuário não pediu, num card
sobre outra coisa, é ruído no diff e no review.

Se o pedido estiver ambíguo de um jeito que mude o resultado, pergunte. Se der
para decidir com bom senso, decida e diga qual suposição você usou.
