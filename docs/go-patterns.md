# Padrões de Go neste projeto

Regras que valem aqui, com o porquê e um exemplo do próprio código. As fontes
são o [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) e o
[Google Go Style Guide](https://google.github.io/styleguide/go/best-practices.html);
o que está abaixo é o recorte que importa para este código, não a lista inteira.

Quando uma regra abaixo conflitar com o estilo de um arquivo existente, o
arquivo existente ganha — consistência local vale mais que regra global.

---

## Erros

### Nunca descarte um erro em silêncio

O pior defeito de um TUI é a tecla que parece não fazer nada. Se uma ação pode
falhar, o usuário tem que ver.

```go
// ruim — a tecla f não fazia nada e ninguém sabia por quê
_ = herdr.AgentFocus(ctx, name)

// bom
if err := herdr.AgentFocus(ctx, name); err != nil {
    return focusFailedMsg{err: err}
}
```

Descartar é aceitável **com comentário dizendo por quê**:

```go
for _, dir := range []string{root + "/columns", root + "/cards"} {
    // Descartado de propósito: num board recém-criado estes podem não
    // existir ainda, e a raiz já garante que mudanças sejam notadas.
    _ = w.Add(dir)
}
```

### Mensagem de erro em minúscula, sem ponto final

Erros são concatenados; maiúscula e ponto no meio de uma frase ficam feios.

```go
return fmt.Errorf("coluna %q não existe", toKey)     // sim
return fmt.Errorf("Coluna %q não existe.", toKey)    // não
```

### `%w` para embrulhar, `%v` na fronteira

Use `%w` quando quem chama pode querer inspecionar com `errors.Is`/`errors.As`.
Use `%v` ao traduzir erro de processo externo para a linguagem do deck — ali o
erro original é diagnóstico, não algo a inspecionar.

```go
// dentro do deck: preserva a cadeia
return nil, fmt.Errorf("criando worktree: %w", err)

// fronteira com o gh: o stderr vira texto, não erro tipado
return State{Err: fmt.Errorf("gh: %s", firstLine(stderr))}
```

### Acrescente só o que quem chama não sabe

Não escreva `"falhou ao X: %w"` quando o erro já diz o que falhou. A presença do
erro já significa falha.

### Erro antes, caminho feliz sem indentação

```go
col := m.currentColumn()
if col == nil {
    return m, nil
}
if !col.HasPrompt() {
    m.setStatus(false, "a coluna %s não tem prompt", col.Title)
    return m, clearStatusCmd()
}
// daqui para baixo, o caso normal
```

### Valide antes de perguntar

Numa confirmação, cheque as pré-condições **antes** de abrir o diálogo. Perguntar
e depois falhar desperdiça a decisão do usuário.

```go
if m.b.HasCards(col.Key) {
    m.setStatus(false, "%s tem cards — mova-os antes de remover", col.Title)
    return m, clearStatusCmd()
}
// só agora pergunta
m.mode = modeConfirm
```

---

## Nomes e documentação

- **Iniciais consistentes**: `URL`, `ID`, `PR` — nunca `Url`, `Id`.
- **Receiver curto**, uma ou duas letras ligadas ao tipo: `func (c *Card)`,
  `func (b *Board)`, `func (m Model)`. Nunca `self` ou `this`.
- **Sem o nome do pacote no identificador**: `gh.Fetch`, não `gh.FetchGitHubPR`.
- **Todo exportado tem doc comment**, começando pelo nome:
  `// Import busca uma issue ou PR pela URL.`
- **Não exportado não trivial também merece comentário** — sobretudo quando o
  código responde a uma armadilha.

### Comente a decisão, não a mecânica

O código já diz o que faz. O comentário existe para dizer o que quem lê não tem
como deduzir: por que assim, e o que dá errado do outro jeito.

```go
// Estilo explícito, não WithAutoStyle: a detecção automática cai num perfil
// ASCII que deixa "##" e "**" literais na tela, o que é pior do que não
// renderizar.
style := "light"
```

---

## Concorrência e processos externos

### `context.Context` é o primeiro parâmetro, sempre com prazo

Nenhuma chamada externa pode pendurar a UI para sempre.

```go
func Fetch(ctx context.Context, url string) State
func WorktreeCreate(ctx context.Context, cwd, branch, label string) (*WorktreeCreated, error)
```

### Nada caro na abertura

`gh auth status` custa ~300ms porque consulta a API. Chamá-lo em `New()` atrasava
toda abertura do board e fazia a suíte de testes ir de 3s para 20s. Verificação
cara vai para um `tea.Cmd` em segundo plano.

```go
// New(): barato, só LookPath
m.ghEnabled = cfg.GitHub.Enabled(gh.Installed())
// Init(): a sessão é confirmada depois, sem bloquear
cmds = append(cmds, checkGitHubAuth())
```

### Limite a concorrência de subprocessos

Um board com cinquenta cards não pode disparar cinquenta processos `gh`.

```go
sem := make(chan struct{}, 4)
```

### O CLI externo é a autoridade sobre o formato

Não adivinhe JSON de outro programa. `herdr api schema --json` e
`gh ... --json <campos>` dizem a verdade. E teste contra o sistema real quando
puder: foi assim que se descobriu que o `gh` canonicaliza a URL de uma issue que
na verdade é um PR — a detecção precisa usar a URL **devolvida**, não a digitada.

---

## Testes

### Tabela com campos nomeados

```go
cases := []struct {
    name  string
    state State
    want  string
}{
    {"merged ganha de tudo", State{State: "MERGED", ChecksFailing: 3}, "◆ merged"},
    ...
}
```

### `t.Helper()` em todo auxiliar

Sem isso a falha aponta para o auxiliar, não para o teste que quebrou.

```go
func press(t *testing.T, m Model, keys ...string) Model {
    t.Helper()
    ...
}
```

### `t.Fatal` só para pré-condição; `t.Error` para asserção

`Fatal` quando continuar não faz sentido (o setup falhou). `Error` quando dá para
seguir verificando as outras asserções e mostrar tudo o que está errado de uma vez.

### A mensagem de falha tem que dizer o que era esperado

```go
t.Errorf("Slugify(%q) = %q, esperava %q", in, got, want)
```

### Teste o comportamento, não a implementação

```go
// bom: mudar a estrutura interna não quebra este teste
if !strings.Contains(got.Body, "To Do → Refine") {
    t.Errorf("transição não foi registrada no log:\n%s", got.Body)
}
```

### Teste o que você teme, não o que é fácil

Os testes que pagaram a conta aqui são os que exercitam a coisa temida:
frontmatter desconhecido sobrevivendo ao salvar, card órfão não sumindo, cinco
edições seguidas sem o watcher perder nenhuma, cursor e view concordando sobre
qual card está selecionado.

### Nada de rede na suíte

Testes com chamada real ficam atrás de variável de ambiente e são arquivos
temporários, não código versionado:

```go
if os.Getenv("GH_LIVE") == "" {
    t.Skip("")
}
```

---

## Estrutura

### O núcleo não conhece a borda

`internal/board` não importa nada de UI nem de terminal. Isso é o que permite
testá-lo sem cenário e o que mantém o TUI substituível.

### Uma função de ordenação, usada por todos

Se a view ordena de um jeito e o cursor de outro, o usuário seleciona um card e
move outro. `Model.cardsIn` é a única fonte da ordem — filtro e prioridade entram
ali dentro.

### Prefira função pura a estado guardado

`windowCards` e `windowColumns` recebem tamanhos e devolvem a janela visível.
Guardar um offset de rolagem no `Model` criaria um segundo estado capaz de
divergir do cursor.

### Escrita atômica no que é do usuário

Arquivo temporário no mesmo diretório, depois `rename`. Uma falha no meio nunca
deixa um card truncado.

---

## Armadilhas específicas deste projeto

- **`glamour` intercala ANSI entre as palavras**, inclusive nos espaços. Asserção
  sobre texto renderizado precisa passar por `plain()` antes do `strings.Contains`.
- **Não capture o TUI num PTY** para inspecionar: o Bubble Tea segura o stdin e
  trava. Monte o `Model`, fixe `width`/`height` e imprima `m.View()`.
- **Nunca construa ID do herdr.** Pane fechado não reutiliza ID; pane movido de
  workspace ganha ID novo. Leia sempre da resposta.
- **`--force` nunca.** `worktree remove` sem `--force`: com trabalho não
  commitado o herdr recusa, e recusar é o comportamento certo.
