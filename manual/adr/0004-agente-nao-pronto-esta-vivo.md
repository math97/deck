---
adr: 0004
titulo: agent_not_ready significa agente vivo, não falha
status: aceita
data: 2026-09-02
codigo: internal/herdr/herdr.go:AgentStart, internal/ui/update.go:deliverPending
verificar: go test ./internal/ui/ -run TestTarefaPendente
---

# ADR-0004 — `agent_not_ready` significa agente vivo, não falha

## Decisão

`AgentStart` trata o erro `agent_not_ready` como **sucesso parcial**: busca o
agente por nome e o devolve junto com o erro. A cadeia de provedores para em
**ter agente**, não em ausência de erro. A tarefa fica pendente e é entregue
sozinha quando o agente destrava.

## Contexto

Um agente pode subir e parar numa pergunta antes de aceitar tarefa. O caso comum
é o "is this a project you trust?" do Claude Code numa worktree nova — que é
exatamente onde o deck sempre põe o agente, então não é caso de borda: é o
caminho comum.

O herdr responde com erro e saída 1, mas `agent list` já traz o agente,
`blocked`, com `launch_pending: true`. Ele **está vivo**.

## Alternativas descartadas

- **Tratar como falha** (o que o código fazia). Falhou em produção, em cadeia:
  o card ficava sem agente, então `f` não tinha onde focar e `c` não tinha o que
  fechar — e a worktree ficava órfã para sempre. Pior, a cadeia de provedores
  tentava o próximo tipo **no mesmo pane**, que a essa altura respondia
  `agent_pane_busy`: a cadeia falhava inteira, justamente no caso para o qual
  ela existe.
- **Perguntar ao usuário se deve adotar.** Um diálogo para uma pergunta cuja
  resposta é sempre sim. Confirmação é para ação irreversível (ADR de `d`, `c`,
  `R`), não para reconciliar estado.

## Consequências

O deck passa a ter estado que o herdr não tem: `pendingPrompts`, a tarefa que
ainda não foi entregue. Ele é reconciliado a cada poll (`deliverPending`), e
morre com o agente — se o agente some antes de destravar, a tarefa é descartada,
porque não há a quem entregar.

A lição que generaliza: **o código do erro carrega informação que a mensagem não
carrega.** Descartá-lo transforma "deu certo de um jeito diferente" em "falhou".
Por isso `herdr.Code(err)` existe e `run()` não engole mais o código.

## Onde vive

`internal/herdr/herdr.go:AgentStart`. Os códigos que mudam decisão são
constantes comentadas no mesmo arquivo (`CodeAgentNotReady`, `CodeAgentBlocked`,
`CodePaneBusy`, `CodeDirtyWorktree`) — ali, e não num documento, porque é onde
não dá para divergir do que o código faz.

`TestAgenteBloqueadoNoStartFicaLigadoAoCard`, `TestTarefaPendenteSoSaiQuandoOAgenteLibera`,
`TestTarefaPendenteMorreComOAgente`.
