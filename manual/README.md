# manual

O que é preciso saber **antes** de mexer neste código. Versionado de propósito:
sem isto, um agente ou uma pessoa nova refaz decisões que já custaram caro.

| arquivo | o que traz | por que é versionado |
|---|---|---|
| [`go-patterns.md`](go-patterns.md) | padrões de Go com exemplos daqui | o `AGENTS.md` manda ler antes de escrever código |
| [`regras.md`](regras.md) | 16 invariantes numeradas | `internal/board/regras_test.go` cita cada uma pelo número |
| [`security.md`](security.md) | modelo de ameaça e mitigações | explica por que partes do código são defensivas |
| [`caminho-vivo.md`](caminho-vivo.md) | o que o herdr e o `gh` reais ensinaram | seis bugs; sem ele, voltam |

## O que entra aqui, o que não

**Entra o que muda o que alguém faz:** uma invariante que tem teste, um formato
externo que já enganou, um padrão que o código segue, uma decisão e a
alternativa descartada.

**Não entra rascunho.** Roteiro de teste em aberto, anotação de investigação,
lista de coisas para tentar — isso vive em `docs/`, que é **local e não
versionado**. A diferença não é formalidade: o que está aqui é lido por quem
chega sem contexto, então tem que estar certo hoje. Rascunho errado aqui é pior
que rascunho nenhum.

**O teste é a autoridade, não este diretório.** Quando um documento daqui e o
código discordarem, o código está certo e o documento está velho — conserte o
documento. A garantia mora em `go test ./...`.

Uma nota que ninguém precisa para trabalhar não pertence a nenhum dos dois: ou
vira comentário na linha que ela explica, ou vira corpo de commit.
