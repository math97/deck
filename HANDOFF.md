# HANDOFF

Estado do projeto e o que vem a seguir, para quem pegar daqui.

Leia [`AGENTS.md`](AGENTS.md), [`CLAUDE.md`](CLAUDE.md) e
[`docs/go-patterns.md`](docs/go-patterns.md) antes de escrever código. Este
arquivo é só o mapa do que falta.

`docs/` **não é versionado** — as referências a ele daqui para baixo só abrem na
máquina de quem trabalha no projeto.

---

## Onde estamos

164 testes, `go vet` e `gofmt` limpos. Binário em `~/.local/bin/deck`.

Funcionando e testado: board em markdown com rolagem, colunas e prompts
editáveis, artefatos por coluna, busca, arquivamento, importação de issue/PR do
GitHub, badges de PR, publicação de review, worktree por card, disparo e
acompanhamento de agentes via herdr, cadeia de provedores, skills como prompt.

### O caminho vivo, exercitado

O risco número um do projeto foi reduzido: as chamadas ao herdr foram
exercitadas contra um herdr 0.8.2 vivo (fora do TUI, dirigindo o CLI e o pacote
`internal/herdr` direto). O que se aprendeu está em
[`docs/caminho-vivo.md`](docs/caminho-vivo.md).

| chamada | estado |
|---|---|
| `worktree create` / `remove` | ✅ verificado, inclusive a recusa de worktree suja sem `--force` |
| `agent start` | ✅ verificado — **três bugs achados e corrigidos** |
| `agent prompt` / `agent get` / `agent list` | ✅ verificado |
| `agent read` (captura) | ✅ verificado — devolve texto puro, não JSON; estava quebrado |
| cadeia de provedores | ✅ corrigida: parava no erro, não no agente vivo |
| `gh pr comment` | ✅ verificado contra um PR real — passou sem correção |

O caminho de **leitura** do GitHub foi validado contra a API real: 32 checks de
um PR aberto do `cli/cli`, categorias fechando com o total, e as três formas de
erro devolvendo mensagem útil.

O de **escrita** também: o `R` publicou num PR real em 2026-09-05, e o
comentário foi relido pela API — corpo byte a byte igual ao artefato, markdown
preservado, autoria do usuário. Passou sem precisar de correção, ao contrário
do caminho do herdr; o porquê está no `docs/caminho-vivo.md`. O harness é
`TestLivePostReview`, atrás de `DECK_LIVE_PR`.

**Todo caminho de escrita do deck já tocou um sistema real.**

### O que ainda não foi exercitado

- **O TUI dentro de um pane do herdr.** Tudo acima foi dirigido pelo CLI, pelo
  pacote e pelo `Model`; `HERDR_ENV` chegando ao processo, o rodapé mostrando
  `s agente · f pane`, e o fluxo de tecla ponta a ponta seguem por confirmar.
- **A entrega tardia do prompt** (agente que sobe parado numa pergunta) tem
  teste de unidade, mas nunca rodou contra um agente real que ficou bloqueado e
  depois liberou.

---

## Prioridade

Ordenado por ganho sobre complexidade. O item 1 vale mais que o resto somado,
porque reduz risco em vez de adicionar superfície — e é a última peça do
projeto que nunca rodou num sistema real.

### 1. Fechar o que restou do caminho vivo — ganho alto, complexidade baixa

Sobrou **só o TUI dentro de um pane**. O `R` foi fechado — ver acima.

O roteiro item a item está em `docs/teste-no-pane.md`, com o que observar em
cada passo e o que já passou. O portão (`HERDR_ENV` chegando, `s` e `f`) já
passou; o que falta é o ciclo **fechar** — o agente terminar e o deck capturar
o artefato — e o `c` com worktree suja.

O que mais provavelmente ainda quebra: a entrega tardia do prompt quando o
agente destrava, e o `agent read` num agente em tela alternativa — neste caso o
log dizer que não conseguiu ler é o comportamento **correto**, não um bug.

Para repetir o `R` quando fizer sentido (o harness é seguro, mas publica de
verdade):

```sh
DECK_LIVE_PR=https://github.com/voce/repo/pull/1 \
  go test ./internal/ui/ -run TestLivePostReview -v
```

**Nada abaixo desta linha deveria ser construído antes disto.** Empilhar em cima
de um caminho não exercitado é o jeito mais rápido de acumular retrabalho — e o
exercício de agora rendeu cinco bugs em código que tinha teste passando.

### ✅ 2. Avaliação de segurança — feita

[`docs/security.md`](docs/security.md). Uma vulnerabilidade real corrigida
(URL do frontmatter chegando ao `open` do macOS sem validação de esquema),
conteúdo importado passou a ser delimitado e rotulado, e a confirmação do `R`
passou a dizer que o conteúdo vai público. Travessia de caminho verificada e
coberta por teste.

### ✅ 3. Regras de negócio explícitas — feita

[`docs/regras.md`](docs/regras.md), com 16 regras numeradas e um teste nomeado
por regra em `internal/board/regras_test.go`. O exercício revelou dois bugs,
como previsto: arquivar coluna não removia da lista em memória (dava para zerar
o board), e "review não vazio" media bytes em vez de conteúdo (um review em
branco ia para o PR).

### ~~4. Importação de Jira~~ e ~~5. Puxar a sprint~~ — retirados do roadmap

Decisão do dono do projeto: o deck não importa de tracker corporativo. Ficam
registrados aqui como não-objetivos, não como trabalho adiado.

O que cada um teria custado, para o caso de a decisão ser revista: o Jira pedia
um adaptador ao lado de `internal/gh` (fácil) mais uma decisão sobre onde mora
o API token (o difícil — segredo que não pode cair em arquivo versionado) e uma
conversão de ADF para markdown. Puxar a sprint era `NewCardFromSource` em lote,
com `sprint` no `config.md` e deduplicação por URL.

A metade que *empurra* status de volta ao tracker já tinha saído antes, e por um
motivo mais forte que preferência: dá ao tracker co-propriedade do estado do
card e contradiz o princípio de que o markdown é a fonte da verdade — passaria a
existir conflito, ordem de precedência e cursor de sincronia. Isso continua
valendo se o assunto voltar.

O `I` do GitHub segue sendo o único caminho de importação.

### ~~6. OpenRouter / OmniRouter~~ — resolvido sem código

Decidido: o deck **não fala com API de modelo**. OpenRouter, OmniRouter e afins
entram como `opencode` na cadeia de `agent_kind`, com a credencial e a escolha
de modelo morando dentro do agente. Isso mantém o deck sem chave de API, sem
dependência de rede no caminho de execução, e com um único modelo de execução —
o que evita a segunda implementação de backend que este item exigiria. Ver
`CLAUDE.md`, seção "Skills e provedores".

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
- **`agent read` não devolve JSON.** É o único comando do herdr que imprime o
  conteúdo cru. Decodificar envelope ali fazia toda captura falhar antes de
  chegar ao fallback de transcrição.
- **`agent start` pode devolver erro com o agente vivo.** `agent_not_ready`
  quer dizer "subiu e parou numa pergunta". Tratar como falha abandonava um
  agente rodando, sem dono, numa worktree que ninguém mais removia.
- **`herdr api schema --json` é a autoridade** sobre formato. E testar contra o
  sistema real ainda pega o que o schema não pega: foi assim que se descobriu que
  o `gh` canonicaliza a URL de uma issue que na verdade é um PR.
