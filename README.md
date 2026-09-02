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
deck init    # cria .deck/ com as cinco colunas padrão
deck         # abre o board
deck ls      # lista os cards em texto puro, sem TUI
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
    corrigir-login.md  frontmatter = estado, corpo = contexto
```

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

## Teclas

| tecla | ação |
|---|---|
| `h` `l` | coluna anterior / próxima |
| `j` `k` | card abaixo / acima |
| `g` `G` | primeiro / último card |
| `H` `L` | mover o card para a coluna vizinha |
| `J` `K` | reordenar o card dentro da coluna |
| `enter` | abrir o card |
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

Fase 1 concluída: board navegável, colunas e prompts editáveis, cards criados e
movidos, log automático.

Próximas fases:

2. Detalhe do card renderizado com glamour, busca, checkboxes marcáveis no TUI
3. Integração com o [herdr](https://herdr.dev): `a` dispara um agente no prompt
   da coluna, badge de estado (`working` / `blocked` / `done`) por card, `f`
   pula para o pane
4. Captura do resultado do agente de volta na seção `## Log` do card
