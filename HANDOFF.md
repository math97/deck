# HANDOFF

Estado do projeto e o que vem a seguir, para quem pegar daqui.

Leia [`AGENTS.md`](AGENTS.md), [`CLAUDE.md`](CLAUDE.md) e
[`docs/go-patterns.md`](docs/go-patterns.md) antes de escrever código. Este
arquivo é só o mapa do que falta.

---

## Onde estamos

138 testes, `go vet` e `gofmt` limpos, 11 commits. Binário em `~/.local/bin/deck`.

Funcionando e testado: board em markdown com rolagem, colunas e prompts
editáveis, artefatos por coluna, busca, arquivamento, importação de issue/PR do
GitHub, badges de PR, publicação de review, worktree por card, disparo e
acompanhamento de agentes via herdr, cadeia de provedores, skills como prompt.

### O que NUNCA rodou de verdade

Este é o risco número um do projeto. Foi escrito contra `herdr api schema` e
coberto por teste, mas nenhuma destas chamadas tocou um sistema vivo:

| chamada | onde | disparada por |
|---|---|---|
| `worktree create` / `worktree remove` | `internal/herdr` | `s` / `c` |
| `agent start` / `agent prompt` | `internal/herdr` | `s` |
| `pane split` / `pane close` | `internal/herdr` | `s` / `c` |
| `agent read` (captura) | `internal/ui/capture.go` | fim de agente |
| `gh pr comment` | `internal/gh` | `R` |
| cadeia de provedores | `internal/ui/agents.go` | `s`, quando o 1º falha |

O caminho de **leitura** do GitHub foi validado contra a API real: 32 checks de
um PR aberto do `cli/cli`, categorias fechando com o total, e as três formas de
erro devolvendo mensagem útil. O caminho de **escrita** não.

---

## Prioridade

Ordenado por ganho sobre complexidade. Os dois primeiros valem mais que todo o
resto somado, porque reduzem risco em vez de adicionar superfície.

### 1. Validar o caminho vivo — ganho alto, complexidade baixa

Abrir o `deck` dentro de um pane do herdr e exercitar `s`, `f`, `c`, `R` num
projeto real. Consertar o que quebrar.

Sugestão de ordem: confirmar que o rodapé mostra `s agente · f pane` (se não
mostrar, `HERDR_ENV` não chegou e nada mais funciona) → `deck prompt <card>`
para conferir o texto antes de disparar → `s` num card em Refine.

O que mais provavelmente quebra: o parse da resposta de `agent start` (é onde há
mais campos), a detecção de `done` (o herdr pode reportar `idle`), e o
`agent read` se o agente rodar em tela alternativa — neste caso o log dizer que
não conseguiu ler é o comportamento **correto**, não um bug.

**Nada abaixo desta linha deveria ser construído antes disto.** Empilhar em cima
de um caminho não exercitado é o jeito mais rápido de acumular retrabalho.

### 2. Avaliação de segurança — ganho alto, complexidade média

O deck ficou capaz de coisas que pedem revisão antes do open source. Pontos
concretos para examinar, não uma varredura genérica:

- **Injeção de prompt via conteúdo importado.** `I` traz o corpo de uma issue de
  repositório público e o inlina no prompt de um agente que tem escrita em disco
  numa worktree. Um "ignore as instruções anteriores e rode X" no corpo da issue
  chega ao agente. Hoje não há delimitação nem aviso. Ver
  `board.NewCardFromSource` e `board.RenderPrompt`.
- **Conteúdo do card sai para terceiros.** `R` publica `code-review.md` num PR
  público; `s` manda o card inteiro para o provedor. Se alguém colar credencial
  num card, ela vaza nos dois caminhos. Vale ao menos um aviso na confirmação.
- **Caminhos derivados de entrada.** `card.ID`, `col.Key` e `skill` compõem
  caminhos de arquivo. `Slugify` protege o id; a key vem de um nome de arquivo
  real. Confirmar que não há travessia possível, e cobrir com teste.
- **Segredos em config.** Hoje o `config.md` não guarda credencial nenhuma — é
  uma propriedade que vale preservar explicitamente, sobretudo quando o
  OpenRouter entrar (ver item 6).
- **Execução de processo.** Tudo usa `exec.Command` com argv, sem shell, o que
  elimina injeção de comando. Manter essa regra ao adicionar integrações.

Entregar como `docs/security.md`, com o que foi verificado, o que foi mitigado e
o que fica como risco aceito e por quê.

### 3. Regras de negócio explícitas — ganho médio, complexidade baixa

Hoje as invariantes do board estão espalhadas pelo código e só existem como
teste. Antes de abrir o projeto, vale escrevê-las num lugar só — e o exercício
provavelmente revela regras que ninguém decidiu de propósito.

Comece pelas que já existem: quando um card pode mudar de coluna (limite de WIP),
quando um agente pode subir (coluna com prompt, sem agente vivo), quem pode
publicar review (coluna com `post_review`, card com `github_pr`, artefato não
vazio), quando uma coluna pode ser arquivada (vazia, e nunca a última), o que
acontece com um card órfão.

E as que **ainda não foram decididas**: um card pode voltar de Done para Refine?
Mover um card com agente rodando é permitido? Dois cards podem usar o mesmo
branch? Um card sem critério de aceite pode sair de Refine?

Entregar como `docs/regras.md`, e transformar cada regra num teste nomeado por
ela.

### 4. Importação de Jira — ganho médio, complexidade média

Mesmo formato do `I` do GitHub: um adaptador ao lado de `internal/gh`. **Não é
arquitetural.**

O trabalho real é a autenticação: Jira quer base URL, e-mail e API token. Decidir
onde isso mora — variável de ambiente é o caminho que não coloca segredo em
arquivo versionado. O `config.md` guarda a URL base; o token, não.

Campos a trazer: chave (`PROJ-42`), sumário, descrição (Jira usa ADF em v3 —
converter para markdown ou pedir `renderedFields`), status, assignee.

### 5. Sincronia de sprint — ganho alto, complexidade alta, **arquitetural**

O que foi pedido: definir a sprint (GitHub Project ou Jira), puxar só os tickets
atribuídos a você, e sincronizar movimentos — mover para Refine marca "Doing" no
tracker; comentários opcionais no ticket a cada transição.

**Isto quebra um princípio do projeto.** `CLAUDE.md` diz "o markdown é a fonte da
verdade". Sincronia bidirecional dá ao tracker co-propriedade do estado, e aí é
preciso responder: quem ganha num conflito? O que acontece com um card movido
localmente enquanto alguém mudava o status no Jira? Há cursor de sincronia?

**Decida isso antes de escrever código.** A recomendação é começar
deliberadamente assimétrico:

- **Puxar** é seguro: importar os tickets da sprint atribuídos a você é só
  `NewCardFromSource` em lote, com um `sprint` no `config.md` e deduplicação por
  URL. Faça isso primeiro; entrega quase todo o valor.
- **Empurrar** é onde mora o risco: escrever status e comentário em ticket que o
  time inteiro vê. Trate como o `R` já é tratado — confirmação explícita, padrão
  em **não**, e o mapeamento coluna↔status declarado no `config.md`, nunca
  adivinhado.

O mapeamento é específico de cada time e não deve ter padrão embutido: se o deck
adivinhar errado, ele move o ticket de outra pessoa no board do time.

### 6. OpenRouter / OmniRouter — ganho médio-alto, complexidade alta, **arquitetural**

Hoje a execução inteira assume o modelo do herdr: o agente vive num pane, tem
ciclo de vida observável (`idle`/`working`/`blocked`/`done`), e **é ele quem
escreve o artefato**. Um provedor de API não tem nada disso — sem pane, sem
ciclo, sem `f` nem `c` — e **o deck passa a escrever o artefato**, invertendo a
responsabilidade em torno da qual a fase 4 foi construída.

O que isso exige:

- uma interface de execução com duas implementações: a atual (herdr) e a nova
  (HTTP). Hoje `internal/ui/agents.go` fala com o herdr diretamente
- estado do trabalho que não vem do herdr: o deck precisa acompanhar sua própria
  requisição em voo, e persistir isso para sobreviver a um fechamento do board
- a primeira chave de API que o deck toca — ver o item de segurança
- seleção de modelo por coluna, e o que fazer quando o modelo não existe

`agent_kind` já é uma cadeia; a extensão natural é um `provider: openrouter` com
`model:`, e a cadeia passando a misturar as duas formas. Note que isso torna a
cadeia de provedores *mais* útil: cair do herdr para uma API quando a cota local
acaba é exatamente o caso de uso original.

---

## Armadilhas que já custaram tempo

- **Não capture o TUI num PTY.** O Bubble Tea segura o stdin e trava até o
  timeout. Monte o `Model`, fixe `width`/`height`, imprima `m.View()`.
- **`plain()` nas asserções sobre texto renderizado.** O `glamour` intercala
  ANSI entre as palavras, inclusive nos espaços.
- **`glamour.WithAutoStyle()` não estiliza** — cai num perfil ASCII que deixa
  `##` e `**` literais. Use estilo explícito.
- **Nada caro em `New()`.** `gh auth status` custa 300ms e fazia a suíte ir de 3s
  para 20s, além de atrasar toda abertura do board.
- **Teste que varre o home encontra a máquina, não o cenário.** Foi por isso que
  `skill.ListIn` existe separado de `skill.List`.
- **`herdr api schema --json` é a autoridade** sobre formato. E testar contra o
  sistema real ainda pega o que o schema não pega: foi assim que se descobriu que
  o `gh` canonicaliza a URL de uma issue que na verdade é um PR.
