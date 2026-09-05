# O caminho vivo

O que se aprendeu exercitando o deck contra um **herdr 0.8.2 real**, em vez de
contra `herdr api schema` e testes. Este documento existe porque cada linha dele
foi uma suposição errada, e a próxima integração vai errar do mesmo jeito se não
souber disso.

Método: o TUI não foi dirigido (Bubble Tea segura o stdin — ver `CLAUDE.md`).
O que se dirigiu foi o CLI do herdr e o pacote `internal/herdr` direto, num
repositório git descartável, com uma worktree criada e removida ao final.

---

## O que estava certo

- O envelope `{id, result}` e o erro no stderr como `{id, error:{code,message}}`
  com saída 1 — exatamente como o schema diz.
- `worktree create` devolve tudo de uma vez: `worktree.path`, `worktree.branch`,
  `root_pane.pane_id` e `workspace.workspace_id`. Não é preciso `pane split`.
- **`worktree remove` sem `--force` recusa worktree suja**, com
  `dirty_worktree_requires_force`. A invariante do `c` vale de verdade.
- A detecção de fim: o herdr reporta `done`, não `idle`. A dúvida registrada no
  HANDOFF era infundada.
- `KnownKinds()` casa com a saída real — o herdr imprime `kinds: pi|claude|…`
  na última linha do uso de `herdr agent`.

## Bug 1 — `agent start` devolvia erro com o agente vivo

Um agente pode subir e **parar numa pergunta** antes de aceitar tarefa. O caso
comum é o "Quick safety check: is this a project you trust?" do Claude Code numa
worktree nova — que é exatamente onde o deck sempre põe o agente.

O herdr responde:

    {"error":{"code":"agent_not_ready",
              "message":"agent card-cobaia is blocked during startup and is not ready for prompts"}}

Mas `agent list` já traz o agente, `blocked`, com `launch_pending: true`. Ele
**está vivo**.

`AgentStart` devolvia `nil, err`. Consequência em cadeia:

1. o card ficava sem agente — `f` não tinha onde focar, `c` não tinha o que
   fechar, e a worktree ficava órfã para sempre;
2. a cadeia de provedores tentava o próximo tipo **no mesmo pane**, que a essa
   altura respondia `agent_pane_busy` — então a cadeia falhava inteira,
   justamente no caso para o qual ela existe;
3. a tarefa nunca era entregue.

O comentário da própria função já dizia o comportamento certo ("mantém o nome
utilizável"). O código nunca fez isso.

**Corrigido.** `AgentStart` busca o agente por nome quando o código é
`agent_not_ready` e o devolve junto com o erro. A cadeia passou a parar em
**ter agente**, não em ausência de erro.

## Bug 2 — a tarefa se perdia quando o agente subia bloqueado

Mandar prompt para um agente parado numa pergunta devolve:

    {"error":{"code":"agent_blocked","message":"… requires interactive input"}}

O deck mandava assim mesmo e registrava o erro na barra. O agente ficava no pane
sem tarefa nenhuma enquanto o card afirmava que ele estava trabalhando — a pior
combinação possível: nada acontece e nada avisa.

**Corrigido.** A tarefa fica pendente (`Model.pendingPrompts`, por nome de
agente) e o poller a entrega assim que o agente sai de `blocked`. Se o agente
morrer antes disso, a pendência é descartada.

## Bug 3 — `agent read` não devolve JSON

É o único comando do CLI que imprime o conteúdo cru do pane, sem envelope. O
`AgentRead` fazia `json.Unmarshal` na saída e falhava com "resposta ilegível" —
ou seja, **toda captura de fim de agente estava quebrada**, e nunca chegava ao
fallback de transcrição que existia logo abaixo.

**Corrigido.** `AgentRead` usa a saída crua; o tratamento de erro do stderr
continua o mesmo, extraído para `output()`.

## O que continua sendo limitação, não bug

O Claude Code roda em **tela alternativa**: o que sai da tela não entra no
scrollback do herdr. Um `agent read` depois de uma sessão longa devolve as
bordas do TUI e a barra de status, não a transcrição. Não há o que recuperar
ali, e o log do card dizendo isso é o comportamento correto.

Efeito colateral conhecido: a saída não vem *vazia*, vem com esse resto de
interface, então a heurística de "nada legível" em `capture.go` não a detecta.
O arquivo `.session.md` gerado nesse caso é inútil, mas inofensivo.

## Códigos de erro que mudam decisão

Estão em `internal/herdr` como constantes, e `herdr.Code(err)` os extrai:

| código | significa | o que o deck faz |
|---|---|---|
| `agent_not_ready` | subiu, parado numa pergunta | adota o agente, adia a tarefa |
| `agent_blocked` | vivo, esperando input | não manda prompt agora |
| `agent_pane_busy` | já há processo no pane | não insiste no mesmo pane |
| `dirty_worktree_requires_force` | trabalho não commitado | recusa, e está certo |

**A lição geral:** o código do erro carrega informação que a mensagem não
carrega. Descartá-lo, como o `run()` fazia, transforma "deu certo de um jeito
diferente" em "falhou".

---

## O `R`: `gh pr comment` contra um PR real

Exercitado em 2026-09-05 contra o PR #27 de `math97/personalFinance`, aberto e
de propriedade do usuário. Foi o último caminho de **escrita** do deck que
nunca tinha tocado um sistema real.

Método diferente do resto deste documento: aqui deu para dirigir o `Model`, e
não só o pacote. `TestLivePostReview` (`internal/ui/live_pr_test.go`) monta o
board, escreve um artefato de review de verdade, aperta `R`, confirma com `s`,
executa o `tea.Cmd` devolvido — que é quem chama o `gh` — e alimenta a
`reviewPostedMsg` de volta ao modelo. O mesmo caminho do teclado, sem TUI.

O teste fica atrás de `DECK_LIVE_PR` porque publica comentário público e sem
desfazer. Sem a variável, ele pula.

**Passou de primeira, e nada precisou ser corrigido.** Vale registrar por que
este caminho se comportou diferente dos anteriores: ele tem uma superfície só
(`gh pr comment --body-file`, uma chamada, sem estado a acompanhar depois),
enquanto os bugs do herdr moravam todos em *sequências* — subir, perguntar,
destravar, entregar. Caminho de um passo erra menos.

Verificado na origem, relendo o comentário pela API em vez de confiar na saída
do `gh`:

- o corpo chegou **byte a byte igual** ao artefato, com LF puro — o
  `normalizeBody` do caminho de importação não tem contraparte aqui, e não
  precisa: `--body-file` manda o arquivo como está;
- markdown preservado, inclusive os `- [x]` e o link;
- autoria do usuário, não de um app — é o efeito de chamar o `gh` em vez da
  API, e é o comportamento desejado;
- o `gh` imprime a URL do comentário criado, com o fragmento
  `#issuecomment-<id>`. O `postReview` a usa; a suposição de que ela poderia
  vir vazia (e o fallback para a URL do PR) nunca precisou disparar.

O log do card registrou a URL do comentário, não a do PR — que é o que permite
voltar ao que foi publicado, e não só a onde.

**O que continua sem exercício:** o TUI dentro de um pane do herdr. É a única
peça do caminho vivo que sobra.
