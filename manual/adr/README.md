# ADRs

Registro de decisões arquiteturais: **por que X e não Y**, e **o que já
tentamos que falhou**.

## Por que isto não envelhece como spec envelhece

Uma spec descreve **o que o código faz** — e o código muda, então ela mente com
o tempo. Um ADR registra **uma decisão tomada numa data**, que é fato histórico:
não vira mentira, no máximo vira decisão revogada — e aí ganha status
`substituída`, não uma edição por cima.

Daí as três regras que sustentam isso:

1. **ADR aceito não se reescreve.** Mudou de ideia? Escreve outro, marca o
   antigo como `substituída_por`. O caminho errado que se percorreu é a parte
   mais cara e a que mais se esquece.
2. **Nenhum ADR descreve comportamento em detalhe.** Isso é papel do teste. O
   ADR diz o porquê e aponta para onde a garantia mora.
3. **Todo ADR carrega `verificar:`** — um comando que prova que a decisão
   continua valendo. Se ele falhar, o código mudou e o ADR precisa de sucessor.

## Formato

Frontmatter para a máquina, corpo para a pessoa. É o mesmo formato do resto do
projeto — markdown com frontmatter, que é o que o `deck` já lê.

**Máximo uma tela (~60 linhas).** Se não couber, ou são duas decisões, ou está
descrevendo comportamento em vez de decidir.

Copie [`0000-template.md`](0000-template.md).

| campo | para que serve |
|---|---|
| `adr` | número, sequencial, nunca reaproveitado |
| `titulo` | a decisão em uma frase afirmativa, não um tema |
| `status` | `aceita`, `substituída`, `revogada` |
| `data` | quando foi decidida, absoluta |
| `substituida_por` / `substitui` | a corrente de decisões |
| `codigo` | onde a decisão vive, `arquivo:símbolo` |
| `verificar` | comando que falha se a decisão foi desfeita |

Título afirmativo importa: "O deck não fala com API de modelo" se lê inteiro no
índice; "Sobre APIs de modelo" obriga a abrir o arquivo. Vale para pessoa e vale
para agente.

## Índice

| # | decisão | status |
|---|---|---|
| [0001](0001-markdown-e-a-fonte-da-verdade.md) | Markdown com frontmatter é a fonte da verdade | aceita |
| [0002](0002-cli-em-vez-de-api.md) | `gh` e `herdr` são chamados como CLI, não pela API | aceita |
| [0003](0003-sem-api-de-modelo.md) | O deck não fala com API de modelo | aceita |
| [0004](0004-agente-nao-pronto-esta-vivo.md) | `agent_not_ready` significa agente vivo, não falha | aceita |
| [0005](0005-catraca-so-afrouxa-depois-do-merge.md) | A catraca só afrouxa depois do merge | aceita |
| [0006](0006-govulncheck-nao-bloqueia-pr.md) | `govulncheck` não bloqueia PR | aceita |

## Quando escrever um

Quando você **descartou uma alternativa real**, ou quando **tentou algo e não
funcionou**. Não escreva ADR para escolha sem alternativa — isso é comentário no
código, na linha que explica.

Na dúvida: daqui a seis meses, alguém vai olhar esse código e pensar "por que
não fizeram do jeito óbvio?" Se sim, é ADR.
