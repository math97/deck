---
adr: 0001
titulo: Markdown com frontmatter é a fonte da verdade
status: aceita
data: 2026-05-20
codigo: internal/board/frontmatter.go, internal/board/store.go:Save
verificar: go test ./internal/board/ -run TestRegra
---

# ADR-0001 — Markdown com frontmatter é a fonte da verdade

## Decisão

Colunas, cards, config e artefatos são arquivos markdown com frontmatter, lidos
pelo mesmo parser. Não há banco, índice nem estado paralelo. O TUI é uma view
descartável: tudo que ele faz tem que dar para fazer editando arquivo à mão.

## Contexto

Um board precisa de ordenação, filtro, WIP, relação card↔coluna e histórico —
tudo o que normalmente pede um banco. Mas o board tem que conviver com git,
`grep`, editor e agente escrevendo no mesmo arquivo ao mesmo tempo.

## Alternativas descartadas

- **SQLite.** Resolveria consulta e transação, mas criaria um estado que só o
  deck sabe ler: o card deixaria de ser diffável, revisável em PR e editável
  pelo agente que o board mesmo dispara — que é o uso central.
- **Markdown + índice em cache.** Duas fontes que podem divergir. Divergiram na
  primeira semana, no cursor: a view ordenava de um jeito e o cursor de outro,
  e você selecionava um card e movia outro. A correção foi *apagar* a segunda
  fonte, não sincronizá-la (`Model.cardsIn` passou a servir aos dois).

## Consequências

Não há transação: escrita é arquivo temporário + rename, e a atomicidade
termina no arquivo. Não há consulta indexada — tudo é varredura, o que só
funciona porque um board humano tem centenas de cards, não milhões.

O preço maior é a **escrita concorrente**: o agente disparado pelo board edita
o mesmo `card.md` que a captura escreve. É risco aceito, não resolvido.

Em troca, quase toda robustez vira regra de parser, e é onde estão os testes:
campo desconhecido sobrevive, frontmatter sem fechamento vira corpo, card órfão
vai para a coluna `?`.

## Onde vive

`internal/board/frontmatter.go` — parser único.
`TestRegraCampoDesconhecidoSobrevive`, `TestParseDocUnterminatedFrontmatter`,
`TestRegraCardOrfaoNuncaSome` provam as bordas.
