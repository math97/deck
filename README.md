# deck

Board kanban dentro do terminal, definido em markdown, feito para não sair do
terminal enquanto se organiza e se toca trabalho com agentes.

O markdown é a fonte da verdade. O TUI é só uma view: se o board quebrar, os
arquivos continuam lá, legíveis e versionáveis no git.

## Instalação

```sh
go build -o ~/.local/bin/deck ./cmd/deck
```

## Uso

```sh
deck init            # cria .deck/ com as cinco colunas padrão
deck                 # abre o board
deck ls              # lista os cards em texto puro, sem TUI
deck prompt <card>   # imprime o prompt da coluna atual, já renderizado
```

`deck prompt` é a ponte com um agente antes da integração com o herdr:

```sh
deck prompt migrar-oidc | claude -p
```

`deck` procura `.deck/` a partir do diretório atual e sobe a árvore, como o git
faz com `.git`.

## Estrutura

```
.deck/
  config.md          o que está ligado: github, herdr
  columns/
    todo.md          frontmatter = config da coluna, corpo = prompt do agente
    refine.md
    in-progress.md
    code-review.md
    qa.md
    done.md
  cards/
    corrigir-login.md      card simples: um arquivo
    migrar-oidc/           card que acumulou trabalho
      card.md              o card em si
      refine.md            saída da coluna Refine
      in-progress.md       plano e registro da implementação
      qa.md                plano de testes e resultado
```

Um card nasce como `<id>.md`. Na primeira vez que algo precisa gravar um
artefato, o deck promove para `<id>/card.md` sozinho — você nunca decide isso.

**O nome do artefato é a key da coluna que o produziu.** Uma regra, zero
configuração: a coluna já define o prompt, agora define também onde a saída
daquele prompt mora.

### Coluna

```markdown
---
title: Refine
order: 20
agent_kind: claude
wip_limit: 3
---

Você vai refinar esta tarefa comigo. Leia o card em {{card_path}}.
Faça uma pergunta por vez até ter critério de aceite claro.
```

A ordem no board vem de `order`. Uma coluna **sem corpo** é um ponto de parada:
entrar nela não dispara agente nenhum (é o caso de To Do e Done).

Variáveis disponíveis no prompt: `{{card_path}}`, `{{card_title}}`,
`{{card_id}}`, `{{from_column}}`, `{{to_column}}`, `{{cwd}}`.

### Card

```markdown
---
id: corrigir-login
title: Corrigir login
column: in-progress
order: 0
created: 2026-09-01T10:00:00-03:00
updated: 2026-09-01T14:20:00-03:00
---

## Contexto

## Critério de aceite

- [ ] ...

## Log
- 2026-09-01 14:02 · To Do → Refine
```

A seção `## Log` é escrita automaticamente a cada transição. É o que faz um card
parado há duas semanas voltar a fazer sentido quando você o reabre.

## A esteira

Cada coluna recebe, no rodapé do prompt, a lista dos artefatos que as colunas
anteriores produziram. O agente de In Progress lê o `refine.md`; o de QA lê os
dois. É isso que transforma cinco prompts soltos numa corrente — e por isso o
formato tem que ser markdown legível, não log de sessão.

Esse rodapé é anexado **sempre**, independente do que você escreveu no prompt
da coluna. A esteira não pode depender de você lembrar de uma variável.

Variáveis disponíveis: `{{card_path}}`, `{{card_dir}}`, `{{card_id}}`,
`{{card_title}}`, `{{output_path}}`, `{{artifacts}}`, `{{column}}`,
`{{to_column}}`, `{{cwd}}`, `{{github_pr}}`.

## Configuração

`.deck/config.md` é o arquivo central — o que o board tem ligado:

```markdown
---
github: auto          # on | off | auto
herdr: auto           # on | off | auto
github_auto_post: off # publicar review no PR sem perguntar
---
```

`auto` liga a integração quando a ferramenta está disponível. É o padrão, e faz
o board funcionar sem configuração nenhuma; `off` desliga mesmo com a ferramenta
instalada. O corpo do arquivo é livre — anote ali o que quiser sobre o board.

## GitHub

Ponha o link no frontmatter do card:

```yaml
github_pr: https://github.com/org/repo/pull/123
github_issue: https://github.com/org/repo/issues/45
```

O deck consulta o `gh` a cada 60s e mostra um badge no card: `✓ CI ok`,
`✗ CI 2/5`, `⏳ review`, `↩ mudanças`, `◆ merged`, `draft`. No detalhe do card
aparece o resumo completo, e `o` abre o PR no browser.

Requer `gh auth login`. Sem isso o deck simplesmente não mostra badges — nada
quebra.

### Coluna de Code Review

A coluna **Code Review** tem `post_review: on` no frontmatter. O agente dela
escreve o review em `code-review.md`, já em markdown pronto para publicação, e
`R` publica esse mesmo conteúdo como comentário no PR.

O conteúdo é um só, em dois lugares: fica no PR para o time e no card como
documento, ao lado do refinamento e do plano.

**`R` pede confirmação antes de publicar.** Comentar num PR é público e não dá
para desfazer direito, então não é algo que deva acontecer por um toque de tecla
distraído. Para abrir mão da confirmação, ligue `github_auto_post` no
`config.md`.

Qualquer coluna pode virar coluna de review: basta `post_review: on` no
frontmatter dela.

## Agentes (herdr)

Com o deck aberto **dentro de um pane do [herdr](https://herdr.dev)**, `s` num
card dispara um agente:

1. `pane split --no-focus` — direção escolhida pela geometria do pane, como o
   herdr recomenda: pane largo divide à direita, estreito divide para baixo
2. `agent start card-<id> --kind <agent_kind da coluna>`
3. `agent prompt` com o prompt da coluna já renderizado, artefatos anteriores
   incluídos

O nome e o pane do agente ficam gravados no frontmatter do card, e a transição
entra no `## Log`.

Um poller de 2s casa `agent list` com os cards e mostra o estado:
`● trabalhando` · `◐ bloqueado` · `✓ pronto` · `○ ocioso`. **Cards bloqueados ou
prontos sobem para o topo da coluna** — é o que precisa de você. `f` pula para o
pane do agente.

Fora de uma sessão do herdr o board funciona igual; `s` e `f` avisam em vez de
tentar. O deck nunca dirige uma sessão de fora dela.

## Quando o agente termina

O deck detecta a transição para `done` e fecha o loop. Duas coisas distintas:

| | quem escreve | o que é |
|---|---|---|
| `<coluna>.md` | **o agente** | a entrega — o refinamento, o plano, o resultado |
| `<coluna>.session.md` | o deck | transcrição do pane, **só se a entrega não veio** |

O deck **nunca** sobrescreve a entrega do agente com transcrição de terminal.
Ele tira um retrato do artefato antes de disparar; se o arquivo mudou, a
entrega veio e só entra uma linha no `## Log`:

```
- 2026-09-01 16:40 · agente terminou · refine.md gravado (2.1 KB)
```

Se o agente terminou sem gravar — travou, foi interrompido, ou ignorou a
instrução — aí a transcrição é salva ao lado, e o log diz onde:

```
- 2026-09-01 16:40 · agente terminou SEM gravar refine.md · transcrição em refine.session.md
```

`herdr notification` avisa quando um agente bloqueia ou termina, para você não
precisar estar olhando o board.

## Teclas

| tecla | ação |
|---|---|
| `h` `l` | coluna anterior / próxima |
| `j` `k` | card abaixo / acima |
| `g` `G` | primeiro / último card |
| `H` `L` | mover o card para a coluna vizinha |
| `J` `K` | reordenar o card dentro da coluna |
| `enter` | abrir o card (abas: card + um artefato por coluna) |
| `tab` | próxima aba no detalhe do card |
| `o` | abrir o PR do card no browser |
| `s` | subir um agente com o prompt da coluna |
| `f` | pular para o pane do agente |
| `R` | publicar o review no PR (pede confirmação) |
| `n` | novo card na coluna focada |
| `e` | editar o card no `$EDITOR` |
| `p` | editar a coluna — título, config e prompt |
| `a` | nova coluna |
| `r` | renomear a coluna focada |
| `<` `>` | reordenar a coluna |
| `d` | arquivar o card (pede confirmação) |
| `u` | colar o link do PR no card |
| `/` | buscar em título, id e corpo; `esc` limpa |
| `c` | fechar o pane do agente (pede confirmação) |
| `x` | remover a coluna (só se estiver vazia) |
| `?` | ajuda |
| `q` | sair |

## Editar por fora

`.deck/` é feito para ser editado à mão. Um watcher de filesystem redesenha o
board assim que qualquer arquivo muda — inclusive quando um agente edita um card.

## Arquivar

`d` tira o card do board movendo-o (com os artefatos, quando houver) para
`.deck/archive/`. **Nada é apagado** — trabalho refinado, implementado e
revisado não pode sumir por causa de uma tecla. Para apagar de verdade, apague
a pasta você mesmo, fora do TUI.

## Quando não cabe na tela

Colunas com mais cards do que a altura permite mostram `↑ N` e `↓ N`; a janela
acompanha o cursor. O mesmo vale na horizontal: num pane estreito o board
mostra `‹ N` e `N ›` e rola conforme você anda entre as colunas.

No detalhe do card, `j`/`k` rolam o corpo e `g` volta ao topo.

## Garantias

- **Nada se perde.** Um card apontando para uma coluna que não existe mais vai
  para uma coluna `?` visível, com aviso na barra de status — nunca some.
- **Campos desconhecidos sobrevivem.** Se você (ou um agente) adicionar
  `jira: PROJ-42` no frontmatter, o deck preserva ao salvar.
- **Escrita atômica.** Arquivo temporário + rename, para que uma falha no meio
  da gravação não deixe um card truncado.
- **Renomear coluna não orfana card.** O título muda; a key (nome do arquivo)
  permanece.

## Estado

As quatro fases estão concluídas: board navegável, colunas e prompts editáveis,
cards com log automático, artefatos por coluna, badges de PR, disparo e
acompanhamento de agentes, e captura do desfecho de volta no card.

Ideias para depois: renderização com glamour, busca, checkboxes marcáveis no
TUI, e Jira como terceira fonte ao lado do GitHub.
