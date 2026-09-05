---
adr: 0000
titulo: A decisão em uma frase afirmativa
status: aceita
data: 2026-01-01
codigo: caminho/arquivo.go:Simbolo
verificar: go test ./caminho/ -run TestQueProva
---

# ADR-0000 — A decisão em uma frase afirmativa

## Decisão

Uma frase, no imperativo ou no presente. O que vale a partir de agora.

## Contexto

Duas a quatro linhas: que forças estavam em jogo, o que tornava a escolha não
óbvia. Sem história do projeto, sem justificativa — só o problema.

## Alternativas descartadas

- **Y** — por que não. Se foi tentado e falhou, diga *como* falhou.
- **Z** — idem.

O valor do ADR está aqui. Um ADR sem alternativa descartada não precisava
existir.

## Consequências

O que isto custa, não só o que ganha. Um ADR que só lista vantagens não foi
pensado até o fim — toda decisão real tem preço, e quem vier depois vai
esbarrar nele antes de entender por quê.

## Onde vive

`arquivo.go:Simbolo` — e o teste que prova. Quando este parágrafo e o código
discordarem, **o código está certo**: abra um ADR sucessor em vez de editar
este.
