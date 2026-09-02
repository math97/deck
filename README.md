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
  columns/
    todo.md          frontmatter = config da coluna, corpo = prompt do agente
    refine.md
    in-progress.md
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
| `n` | novo card na coluna focada |
| `e` | editar o card no `$EDITOR` |
| `p` | editar a coluna — título, config e prompt |
| `a` | nova coluna |
| `r` | renomear a coluna focada |
| `<` `>` | reordenar a coluna |
| `x` | remover a coluna (só se estiver vazia) |
| `?` | ajuda |
| `q` | sair |

## Editar por fora

`.deck/` é feito para ser editado à mão. Um watcher de filesystem redesenha o
board assim que qualquer arquivo muda — inclusive quando um agente edita um card.

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

Fases 1 a 3 concluídas: board navegável, colunas e prompts editáveis, cards
criados e movidos com log automático, artefatos por coluna com promoção
automática a pasta, abas no detalhe, badges de PR via `gh`, e disparo e
acompanhamento de agentes via herdr.

Próxima fase:

4. Captura automática do resultado quando o agente fica `done`: `agent read`
   anexa o resumo ao artefato e ao `## Log`, e `herdr notification` avisa quando
   algo bloqueia

Depois: renderização com glamour, busca, checkboxes marcáveis, e integração com
Jira como terceira fonte ao lado do GitHub.
