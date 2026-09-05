# Regras do board

As invariantes do deck num lugar só. Cada regra aqui tem um teste nomeado por
ela em [`internal/board/regras_test.go`](../internal/board/regras_test.go) —
**uma regra sem teste é uma regra que ninguém decidiu de propósito.**

O documento também registra o que foi decidido *não* ser regra, e por quê. Essa
metade é a que mais evita retrabalho: sem ela, alguém reintroduz a restrição
seis meses depois achando que era esquecimento.

---

## O que limita o movimento de um card

| # | regra | teste |
|---|---|---|
| R1 | Um card só entra numa coluna que existe. | `TestRegraCardSoEntraEmColunaExistente` |
| R2 | O limite de WIP é da coluna de **destino** e vale em qualquer direção, inclusive voltando. | `TestRegraLimiteDeWIPValeEmQualquerDirecao` |
| R3 | Não há sentido único: um card volta de Done para Refine, ou para onde for. | `TestRegraCardVoltaParaQualquerColuna` |
| R4 | Todo movimento entra no `## Log` do card. | `TestRegraTodoMovimentoEntraNoLog` |

R3 é uma decisão, não uma ausência. Uma esteira obrigatória seria uma regra que
o markdown contorna: basta editar `column:` no frontmatter. **Regra que o
arquivo desfaz não é regra** — vira uma restrição que só atrapalha quem usa o
TUI. O board acompanha o trabalho; não o autoriza.

## O que preserva o que é seu

| # | regra | teste |
|---|---|---|
| R6 | `d` arquiva: move para `.deck/archive/`, nunca apaga. | `TestRegraArquivarPreservaOCard` |
| R7 | Card apontando para coluna inexistente vai para a coluna `?` e continua visível, com aviso na barra. | `TestRegraCardOrfaoNuncaSome` |
| R8 | Campo desconhecido no frontmatter (`jira:`, `assignee:`) sobrevive ao salvar. | `TestRegraCampoDesconhecidoSobrevive` |

R7 tem um efeito colateral que virou propriedade de segurança: como `column:` é
validado contra as colunas existentes antes de compor qualquer caminho, um valor
como `../../etc` nunca chega a virar nome de arquivo. Ver
[`security.md`](security.md) §3.

## O que limita uma coluna

| # | regra | teste |
|---|---|---|
| R5 | Uma coluna só é arquivada se estiver vazia, e nunca a última do board. | `TestRegraColunaSoArquivaVaziaENuncaAUltima` |
| R9 | Só dispara agente a coluna que tem prompt — próprio ou vindo de skill. | `TestRegraColunaSemPromptNaoDisparaAgente` |
| R10 | O nome do artefato é a key da coluna que o produziu. | `TestRegraArtefatoLevaAKeyDaColuna` |

R5 **não valia de verdade até este documento existir.** `ArchiveColumn`
verificava `len(b.Columns) <= 1` mas nunca removia a coluna arquivada da lista
em memória; arquivando uma a uma dava para zerar o board. O TUI escapava por
recarregar entre as teclas. Escrever a regra e testá-la foi o que revelou.

## O que limita um agente

| # | regra | teste |
|---|---|---|
| R11 | Dois cards nunca dividem branch: o branch sai do id do card, único por construção. | `TestRegraCadaCardTemBranchProprio` |
| R12 | Um card tem no máximo um agente vivo. Com agente não-`done`, `s` recusa e manda usar `f`. | `ui.startAgentForCard` |
| R13 | Um agente que sobe parado numa pergunta é do card mesmo assim; a tarefa espera ele liberar. | `TestAgenteBloqueadoNoStartFicaLigadoAoCard` |
| R14 | `c` fecha o pane e remove a worktree **sem `--force`**: com trabalho não commitado o herdr recusa, e recusar é o certo. | verificado contra herdr vivo |

## O que limita a publicação

| # | regra | teste |
|---|---|---|
| R15 | `R` só publica com: coluna marcada `post_review`, card com `github_pr`, e artefato não vazio. | `TestRegraPublicarReviewExigeAsTresCondicoes` |
| R16 | Publicar pede confirmação, cujo padrão é **não**. `github_auto_post: true` no `config.md` abre mão disso, explicitamente. | `TestRegraPublicarPedeConfirmacaoComPadraoNao` |

R15 é a segunda regra que este documento consertou. "Artefato não vazio" era
`info.Size() == 0`, mas `WriteArtifact` garante a quebra de linha final: um
review sem uma palavra escrita tinha 1 byte, passava, e ia para o PR. Agora o
vazio é medido pelo conteúdo.

---

## O que foi decidido não ser regra

**Mover um card com agente rodando é permitido.** Pelo mesmo motivo de R3: o
usuário pode mover editando o arquivo. Mas o agente continua trabalhando contra
a coluna de onde subiu e vai gravar o artefato **dela**, então o board avisa na
barra em vez de bloquear. Bloquear daria a impressão de uma garantia que não
existe.

**Um card pode sair de Refine sem critério de aceite.** O deck não lê o conteúdo
do card para decidir se ele está pronto. Isso é convenção de time, não invariante
de board — e um board que reprova texto seria um board que as pessoas contornam
escrevendo "- [x] ok".

**Não há dono, nem prazo, nem prioridade.** O board tem ordem dentro da coluna e
mais nada. Quem precisar de dono põe `assignee:` no frontmatter, que R8 preserva
sem o deck precisar conhecer o campo.

**Não há limite de cards por board, nem de colunas.** Rolagem resolve; limite
seria número inventado.

---

## Quando mexer aqui

Regra nova entra com número, linha na tabela e teste nomeado por ela. Regra que
sai, sai daqui junto com o teste — e o motivo fica registrado no commit, porque
"por que isso não é mais proibido" é a pergunta que aparece depois.
