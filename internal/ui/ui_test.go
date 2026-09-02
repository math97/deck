package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matheusalbuquerque/deck/internal/board"
	"github.com/matheusalbuquerque/deck/internal/gh"
	"github.com/matheusalbuquerque/deck/internal/herdr"
)

// newTestModel monta um board temporário e o modelo já dimensionado.
func newTestModel(t *testing.T) Model {
	t.Helper()
	dir := t.TempDir()
	root, _, err := board.Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	b, err := board.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := New(b)
	m.width, m.height = 160, 40
	return m
}

// ansiSeq casa as sequências de escape que o glamour emite.
var ansiSeq = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// plain devolve o texto sem estilo. O glamour intercala códigos ANSI entre as
// palavras — inclusive nos espaços —, então comparar conteúdo com a saída crua
// falha mesmo quando o texto está lá.
func plain(s string) string { return ansiSeq.ReplaceAllString(s, "") }

// press envia uma sequência de teclas ao modelo.
func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEscape}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

// typeText digita caractere a caractere, como o usuário faria.
func typeText(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	return m
}

func TestNavigateColumns(t *testing.T) {
	m := newTestModel(t)
	if got := m.currentColumn().Key; got != "todo" {
		t.Fatalf("foco inicial deveria ser todo, é %q", got)
	}

	m = press(t, m, "l", "l")
	if got := m.currentColumn().Key; got != "in-progress" {
		t.Errorf("depois de 2x l esperava in-progress, veio %q", got)
	}

	m = press(t, m, "h")
	if got := m.currentColumn().Key; got != "refine" {
		t.Errorf("depois de h esperava refine, veio %q", got)
	}

	// Não pode passar do início.
	m = press(t, m, "h", "h", "h", "h")
	if got := m.currentColumn().Key; got != "todo" {
		t.Errorf("navegação deveria parar em todo, veio %q", got)
	}
}

func TestCreateCardThroughUI(t *testing.T) {
	m := newTestModel(t)

	m = press(t, m, "n")
	if m.mode != modeInput {
		t.Fatal("'n' deveria entrar em modo de entrada")
	}
	m = typeText(t, m, "Corrigir login")
	m = press(t, m, "enter")

	if m.mode != modeNormal {
		t.Error("enter deveria voltar ao modo normal")
	}
	cards := m.b.CardsIn("todo")
	if len(cards) != 1 {
		t.Fatalf("esperava 1 card em todo, veio %d", len(cards))
	}
	if cards[0].Title != "Corrigir login" {
		t.Errorf("título errado: %q", cards[0].Title)
	}
	if m.currentCard() == nil || m.currentCard().Title != "Corrigir login" {
		t.Error("o foco deveria ficar no card recém-criado")
	}
}

func TestEscapeCancelsInput(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "não quero")
	m = press(t, m, "esc")

	if m.mode != modeNormal {
		t.Error("esc deveria voltar ao modo normal")
	}
	if len(m.b.CardsIn("todo")) != 0 {
		t.Error("esc não deveria criar card")
	}
}

func TestMoveCardAcrossColumns(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "tarefa")
	m = press(t, m, "enter")

	m = press(t, m, "L")
	if got := m.currentColumn().Key; got != "refine" {
		t.Fatalf("o foco deveria seguir o card para refine, está em %q", got)
	}
	if len(m.b.CardsIn("refine")) != 1 {
		t.Error("card deveria ter ido para refine")
	}
	if len(m.b.CardsIn("todo")) != 0 {
		t.Error("card não deveria continuar em todo")
	}

	// E o log registrou a transição no disco.
	card := m.b.CardsIn("refine")[0]
	if !strings.Contains(card.Body, "To Do → Refine") {
		t.Errorf("transição não registrada:\n%s", card.Body)
	}

	m = press(t, m, "H")
	if got := m.currentColumn().Key; got != "todo" {
		t.Errorf("H deveria trazer de volta para todo, está em %q", got)
	}
}

func TestNewColumnThroughUI(t *testing.T) {
	m := newTestModel(t)
	before := len(m.columns())

	m = press(t, m, "a")
	m = typeText(t, m, "Bloqueado")
	m = press(t, m, "enter")

	if len(m.columns()) != before+1 {
		t.Fatalf("esperava %d colunas, veio %d", before+1, len(m.columns()))
	}
	col := m.b.Column("bloqueado")
	if col == nil {
		t.Fatal("coluna 'bloqueado' não foi criada")
	}
	if col.Title != "Bloqueado" {
		t.Errorf("título errado: %q", col.Title)
	}
	if m.currentColumn().Key != "bloqueado" {
		t.Error("o foco deveria ir para a coluna nova")
	}
}

func TestRenameColumnKeepsKey(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "card")
	m = press(t, m, "enter")

	m = press(t, m, "r")
	// O campo vem pré-preenchido com o título atual; limpa antes de digitar.
	m.input.SetValue("")
	m = typeText(t, m, "Backlog")
	m = press(t, m, "enter")

	if got := m.b.Column("todo").Title; got != "Backlog" {
		t.Errorf("título não mudou: %q", got)
	}
	// A key preservada é o que impede o card de virar órfão.
	if len(m.b.CardsIn("todo")) != 1 {
		t.Error("renomear não deveria orfanar o card")
	}
}

func TestDeleteOccupiedColumnIsRefused(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "card")
	m = press(t, m, "enter")

	before := len(m.columns())
	m = press(t, m, "x")

	if len(m.columns()) != before {
		t.Error("coluna com card não deveria ser removida")
	}
	if m.status == "" || m.statusOK {
		t.Error("deveria mostrar erro na barra de status")
	}
}

func TestViewRendersAllColumns(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "Corrigir login")
	m = press(t, m, "enter")

	out := m.View()
	for _, want := range []string{"To Do", "Refine", "In Progress", "QA", "Done", "Corrigir login"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() não contém %q", want)
		}
	}
}

func TestHelpOverlayOpensAndCloses(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "?")
	if m.mode != modeHelp {
		t.Fatal("'?' deveria abrir a ajuda")
	}
	if !strings.Contains(m.View(), "reordenar a coluna") {
		t.Error("ajuda não está sendo renderizada")
	}
	m = press(t, m, "esc")
	if m.mode != modeNormal {
		t.Error("esc deveria fechar a ajuda")
	}
}

func TestDetailOverlayShowsBody(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "Investigar timeout")
	m = press(t, m, "enter")

	m = press(t, m, "enter")
	if m.mode != modeDetail {
		t.Fatal("enter deveria abrir o detalhe do card")
	}
	out := plain(m.View())
	if !strings.Contains(out, "Investigar timeout") {
		t.Error("detalhe deveria mostrar o título")
	}
	if !strings.Contains(out, "Critério de aceite") {
		t.Error("detalhe deveria mostrar o corpo do card")
	}
}

func TestShiftColumnThroughUI(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "l", "l") // in-progress
	m = press(t, m, "<")

	if got := m.columns()[1].Key; got != "in-progress" {
		t.Errorf("in-progress deveria estar na posição 1, lá está %q", got)
	}
	if m.currentColumn().Key != "in-progress" {
		t.Error("o foco deveria acompanhar a coluna movida")
	}
}

func TestReorderCardWithinColumn(t *testing.T) {
	m := newTestModel(t)
	for _, title := range []string{"primeira", "segunda"} {
		m = press(t, m, "n")
		m = typeText(t, m, title)
		m = press(t, m, "enter")
	}

	cards := m.b.CardsIn("todo")
	if len(cards) != 2 {
		t.Fatalf("esperava 2 cards, veio %d", len(cards))
	}
	first := cards[0].Title

	// Foca o primeiro card e desce.
	m.cardIdx = 0
	m = press(t, m, "J")

	after := m.b.CardsIn("todo")
	if after[0].Title == first {
		t.Errorf("a ordem não mudou: ainda começa com %q", first)
	}
	if m.currentCard().Title != first {
		t.Error("o foco deveria seguir o card movido")
	}
}

func TestDetailTabsCycleThroughArtifacts(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "com esteira")
	m = press(t, m, "enter")

	card := m.currentCard()
	if _, err := card.WriteArtifact("refine", "# refinamento\n\nas perguntas"); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	if _, err := card.WriteArtifact("qa", "# testes\n\no plano"); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	m.reload()

	m = press(t, m, "enter") // abre o detalhe
	if m.mode != modeDetail {
		t.Fatal("enter deveria abrir o detalhe")
	}
	// Aba 0 é o card.
	if !strings.Contains(plain(m.View()), "Critério de aceite") {
		t.Error("aba inicial deveria mostrar o corpo do card")
	}

	m = press(t, m, "tab")
	if !strings.Contains(plain(m.View()), "as perguntas") {
		t.Errorf("segunda aba deveria mostrar o artefato de refine:\n%s", plain(m.View()))
	}

	m = press(t, m, "tab")
	if !strings.Contains(plain(m.View()), "o plano") {
		t.Error("terceira aba deveria mostrar o artefato de qa")
	}

	// Circula de volta para o card.
	m = press(t, m, "tab")
	if m.tabIdx != 0 {
		t.Errorf("tab deveria circular de volta ao card, tabIdx=%d", m.tabIdx)
	}

	m = press(t, m, "esc")
	if m.mode != modeNormal || m.tabIdx != 0 {
		t.Error("esc deveria fechar o detalhe e resetar a aba")
	}
}

func TestCardShowsArtifactCount(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "com artefatos")
	m = press(t, m, "enter")

	m.currentCard().WriteArtifact("refine", "x")
	m.currentCard().WriteArtifact("qa", "y")
	m.reload()

	if !strings.Contains(m.View(), "2📄") {
		t.Errorf("o card deveria indicar 2 artefatos:\n%s", m.View())
	}
}

func TestGitHubBadgeRendersOnCard(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "com PR")
	m = press(t, m, "enter")

	card := m.currentCard()
	card.GitHubPR = "https://github.com/org/repo/pull/7"
	card.Save()
	m.reload()

	// Antes do poller responder, mostra o placeholder.
	if !strings.Contains(m.View(), "PR…") {
		t.Errorf("card com PR deveria mostrar placeholder:\n%s", m.View())
	}

	// Com o estado carregado, mostra o badge.
	m.ghStates[m.currentCard().Path] = gh.State{State: "OPEN", ChecksTotal: 3, ChecksPassing: 3}
	if !strings.Contains(m.View(), "CI ok") {
		t.Errorf("card deveria mostrar o badge do PR:\n%s", m.View())
	}
}

func TestOpenPRWithoutLinkWarns(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "sem PR")
	m = press(t, m, "enter")
	m = press(t, m, "enter") // detalhe

	m = press(t, m, "o")
	if m.status == "" || m.statusOK {
		t.Error("deveria avisar que o card não tem github_pr")
	}
}

func TestBlockedCardsRiseToTop(t *testing.T) {
	m := newTestModel(t)
	for _, title := range []string{"primeira", "segunda", "terceira"} {
		m = press(t, m, "n")
		m = typeText(t, m, title)
		m = press(t, m, "enter")
	}

	// A terceira ganha um agente bloqueado.
	cards := m.b.CardsIn("todo")
	target := cards[len(cards)-1]
	target.Agent = &board.AgentRef{Name: "card-terceira", Pane: "w1:p3", Kind: "claude"}
	if err := target.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	m.reload()
	m.agents["card-terceira"] = herdr.Agent{
		Name: "card-terceira", PaneID: "w1:p3", Status: herdr.StatusBlocked,
	}

	got := m.cardsIn("todo")
	if got[0].Title != target.Title {
		t.Errorf("card bloqueado deveria estar no topo, no topo está %q", got[0].Title)
	}
	if len(got) != 3 {
		t.Errorf("reordenar não pode perder card: %d", len(got))
	}
}

func TestCursorFollowsDisplayOrder(t *testing.T) {
	m := newTestModel(t)
	for _, title := range []string{"primeira", "segunda"} {
		m = press(t, m, "n")
		m = typeText(t, m, title)
		m = press(t, m, "enter")
	}

	last := m.b.CardsIn("todo")[1]
	last.Agent = &board.AgentRef{Name: "card-x", Pane: "w1:p2"}
	last.Save()
	m.reload()
	m.agents["card-x"] = herdr.Agent{Name: "card-x", Status: herdr.StatusBlocked}

	// O cursor no índice 0 tem que apontar para o mesmo card que a view desenha
	// primeiro, senão selecionar e mover agiriam sobre o card errado.
	m.cardIdx = 0
	if m.currentCard().Title != m.cardsIn("todo")[0].Title {
		t.Error("cursor e ordem de exibição divergiram")
	}
}

func TestAgentBadgeRendersOnCard(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "com agente")
	m = press(t, m, "enter")

	card := m.currentCard()
	card.Agent = &board.AgentRef{Name: "card-com-agente", Pane: "w1:p2"}
	card.Save()
	m.reload()
	m.agents["card-com-agente"] = herdr.Agent{
		Name: "card-com-agente", Status: herdr.StatusWorking,
	}

	if !strings.Contains(m.View(), "trabalhando") {
		t.Errorf("card deveria mostrar o badge do agente:\n%s", m.View())
	}
}

func TestStartAgentOutsideHerdrWarns(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "tarefa")
	m = press(t, m, "enter")
	m = press(t, m, "L") // vai para Refine, que tem prompt

	m.herdrInside = false
	m = press(t, m, "s")

	if m.statusOK || !strings.Contains(m.status, "herdr") {
		t.Errorf("deveria avisar que está fora do herdr, status=%q", m.status)
	}
}

func TestStartAgentRefusedOnColumnWithoutPrompt(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "tarefa")
	m = press(t, m, "enter")

	m.herdrInside = true
	m = press(t, m, "s") // ainda em To Do, que não tem prompt

	if m.statusOK || !strings.Contains(m.status, "prompt") {
		t.Errorf("deveria recusar coluna sem prompt, status=%q", m.status)
	}
}

func TestStartAgentRefusedWhenAlreadyRunning(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "tarefa")
	m = press(t, m, "enter")
	m = press(t, m, "L")

	card := m.currentCard()
	card.Agent = &board.AgentRef{Name: "card-tarefa", Pane: "w1:p2"}
	card.Save()
	m.reload()
	m.agents["card-tarefa"] = herdr.Agent{Name: "card-tarefa", Status: herdr.StatusWorking}
	m.herdrInside = true

	m = press(t, m, "s")
	if m.statusOK || !strings.Contains(m.status, "já tem agente") {
		t.Errorf("deveria recusar disparar em cima de agente vivo, status=%q", m.status)
	}
}

func TestLinkAgentPersistsToCard(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "tarefa")
	m = press(t, m, "enter")

	card := m.currentCard()
	m.linkAgent(card, &herdr.Agent{Name: "card-tarefa", PaneID: "w1:p7", Kind: "claude"}, "", "")

	b2, err := board.Load(m.root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := b2.CardsIn("todo")[0]
	if got.Agent == nil {
		t.Fatal("a ligação com o agente não persistiu no frontmatter")
	}
	if got.Agent.Name != "card-tarefa" || got.Agent.Pane != "w1:p7" {
		t.Errorf("agente gravado errado: %+v", got.Agent)
	}
	if !strings.Contains(got.Body, "card-tarefa") {
		t.Errorf("o log deveria registrar o agente:\n%s", got.Body)
	}
}

func TestFocusAgentWithoutAgentWarns(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "tarefa")
	m = press(t, m, "enter")

	m.herdrInside = true
	m = press(t, m, "f")

	if m.statusOK || !strings.Contains(m.status, "agente") {
		t.Errorf("deveria avisar que não há agente, status=%q", m.status)
	}
}

// cardWithAgent monta um card em Refine já ligado a um agente.
func cardWithAgent(t *testing.T, m Model, title, agentName string) (Model, *board.Card) {
	t.Helper()
	m = press(t, m, "n")
	m = typeText(t, m, title)
	m = press(t, m, "enter")
	m = press(t, m, "L") // → Refine

	card := m.currentCard()
	card.Agent = &board.AgentRef{Name: agentName, Pane: "w1:p2", Kind: "claude"}
	if err := card.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	m.reload()
	return m, m.currentCard()
}

func TestCaptureLogsDeliveredArtifact(t *testing.T) {
	m := newTestModel(t)
	m, card := cardWithAgent(t, m, "entrega ok", "card-entrega")

	next, _ := m.captured(captureMsg{
		cardPath:  card.Path,
		delivered: true,
		artifact:  "/x/refine.md",
		size:      2150,
	})
	m = next.(Model)

	b2, _ := board.Load(m.root)
	got := b2.CardsIn("refine")[0]
	if !strings.Contains(got.Body, "refine.md gravado") {
		t.Errorf("log deveria registrar a entrega:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "2.1 KB") {
		t.Errorf("log deveria trazer o tamanho:\n%s", got.Body)
	}
}

func TestCaptureLogsFallbackTranscript(t *testing.T) {
	m := newTestModel(t)
	m, card := cardWithAgent(t, m, "sem entrega", "card-sem")

	next, _ := m.captured(captureMsg{
		cardPath: card.Path,
		artifact: "/x/refine.md",
		session:  "/x/refine.session.md",
	})
	m = next.(Model)

	b2, _ := board.Load(m.root)
	got := b2.CardsIn("refine")[0]
	if !strings.Contains(got.Body, "SEM gravar refine.md") {
		t.Errorf("log deveria dizer que a entrega não veio:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "refine.session.md") {
		t.Errorf("log deveria apontar a transcrição:\n%s", got.Body)
	}
}

func TestSnapshotDetectsNewArtifact(t *testing.T) {
	m := newTestModel(t)
	m, card := cardWithAgent(t, m, "detecta", "card-detecta")

	// Antes: o artefato não existe.
	base := snapshotArtifact(card, "refine")
	if base.exists {
		t.Fatal("baseline não deveria achar artefato inexistente")
	}

	// O "agente" grava.
	if _, err := card.WriteArtifact("refine", "o refinamento"); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	// Recalcula o path agora que o card virou pasta.
	base.path = card.Dir + "/refine.md"

	cmd := captureAgentResult(card, "card-detecta", base)
	msg := cmd().(captureMsg)
	if !msg.delivered {
		t.Errorf("deveria reconhecer a entrega gravada: %+v", msg)
	}
	if msg.size == 0 {
		t.Error("deveria reportar o tamanho da entrega")
	}
}

func TestSnapshotIgnoresUnchangedArtifact(t *testing.T) {
	m := newTestModel(t)
	m, card := cardWithAgent(t, m, "nao mexeu", "card-nao")

	// Artefato já existia antes de o agente subir.
	card.WriteArtifact("refine", "conteúdo antigo")
	base := snapshotArtifact(card, "refine")
	if !base.exists {
		t.Fatal("baseline deveria ter registrado o artefato existente")
	}

	// O agente termina sem tocar no arquivo. Sem herdr vivo, AgentRead falha —
	// o que importa é que NÃO foi marcado como entrega.
	cmd := captureAgentResult(card, "card-nao", base)
	msg := cmd().(captureMsg)
	if msg.delivered {
		t.Error("artefato intocado não deveria contar como entrega desta rodada")
	}
}

func TestDetectFinishedFiresOnceOnDone(t *testing.T) {
	m := newTestModel(t)
	m, _ = cardWithAgent(t, m, "uma vez", "card-uma")

	// working → done dispara a captura.
	m.agents = agentsMsg{"card-uma": herdr.Agent{Name: "card-uma", Status: herdr.StatusWorking}}
	cmds := m.detectFinished(agentsMsg{
		"card-uma": herdr.Agent{Name: "card-uma", Status: herdr.StatusDone},
	})
	if len(cmds) != 1 {
		t.Fatalf("esperava 1 captura, veio %d", len(cmds))
	}

	// done → done não dispara de novo, senão capturaria a cada 2 segundos.
	m.agents = agentsMsg{"card-uma": herdr.Agent{Name: "card-uma", Status: herdr.StatusDone}}
	cmds = m.detectFinished(agentsMsg{
		"card-uma": herdr.Agent{Name: "card-uma", Status: herdr.StatusDone},
	})
	if len(cmds) != 0 {
		t.Errorf("não deveria recapturar um agente que já estava done: %d", len(cmds))
	}
}

func TestDetectFinishedNotifiesOnBlocked(t *testing.T) {
	m := newTestModel(t)
	m, _ = cardWithAgent(t, m, "bloqueia", "card-bloq")

	m.agents = agentsMsg{"card-bloq": herdr.Agent{Name: "card-bloq", Status: herdr.StatusWorking}}
	cmds := m.detectFinished(agentsMsg{
		"card-bloq": herdr.Agent{Name: "card-bloq", Status: herdr.StatusBlocked},
	})
	if len(cmds) != 1 {
		t.Errorf("bloquear deveria gerar uma notificação, veio %d", len(cmds))
	}
}

func TestHumanSize(t *testing.T) {
	for in, want := range map[int64]string{
		512:             "512 B",
		2150:            "2.1 KB",
		3 * 1024 * 1024: "3.0 MB",
	} {
		if got := humanSize(in); got != want {
			t.Errorf("humanSize(%d) = %q, esperava %q", in, got, want)
		}
	}
}

func TestConfigOffDisablesGitHub(t *testing.T) {
	dir := t.TempDir()
	root, _, _ := board.Init(dir)
	os.WriteFile(filepath.Join(root, board.ConfigFileName),
		[]byte("---\ngithub: off\nherdr: off\n---\n"), 0o644)

	b, _ := board.Load(root)
	m := New(b)
	m.width, m.height = 160, 40

	if m.ghEnabled {
		t.Error("github: off deveria desligar a integração")
	}
	if m.herdrInside {
		t.Error("herdr: off deveria desligar a integração")
	}
	// E o rodapé não deve oferecer o que está desligado.
	out := m.View()
	if strings.Contains(out, "R review") || strings.Contains(out, "s agente") {
		t.Errorf("rodapé não deveria oferecer integração desligada:\n%s", out)
	}
}

func TestPostReviewAsksConfirmation(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "revisar")
	m = press(t, m, "enter")
	// todo → refine → in-progress → code-review
	m = press(t, m, "L", "L", "L")
	if got := m.currentColumn().Key; got != "code-review" {
		t.Fatalf("esperava code-review, está em %q", got)
	}

	card := m.currentCard()
	card.GitHubPR = "https://github.com/org/repo/pull/7"
	card.Save()
	card.WriteArtifact("code-review", "# Review\n\nestá bom para merge.\n")
	m.reload()

	m.ghEnabled = true
	m = press(t, m, "R")

	if m.mode != modeConfirm {
		t.Fatalf("publicar no PR deveria pedir confirmação, mode=%v", m.mode)
	}
	if !strings.Contains(m.View(), "org/repo#7") {
		t.Errorf("a confirmação deveria dizer em qual PR vai publicar:\n%s", m.View())
	}

	// Qualquer tecla que não seja afirmativa cancela.
	m = press(t, m, "n")
	if m.mode != modeNormal || m.statusOK {
		t.Error("tecla não afirmativa deveria cancelar")
	}
}

func TestPostReviewRefusedOutsideReviewColumn(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "tarefa")
	m = press(t, m, "enter")
	m.ghEnabled = true

	m = press(t, m, "R") // ainda em To Do
	if m.statusOK || !strings.Contains(m.status, "não publica review") {
		t.Errorf("deveria recusar fora da coluna de review, status=%q", m.status)
	}
}

func TestPostReviewRefusedWithoutArtifact(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "sem review")
	m = press(t, m, "enter")
	m = press(t, m, "L", "L", "L")

	card := m.currentCard()
	card.GitHubPR = "https://github.com/org/repo/pull/7"
	card.Save()
	m.reload()
	m.ghEnabled = true

	m = press(t, m, "R")
	if m.statusOK || !strings.Contains(m.status, "ainda não há review") {
		t.Errorf("deveria recusar sem artefato, status=%q", m.status)
	}
}

func TestReviewPostedLogsToCard(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "publicado")
	m = press(t, m, "enter")
	card := m.currentCard()

	next, _ := m.reviewPosted(reviewPostedMsg{
		cardPath: card.Path,
		url:      "https://github.com/org/repo/pull/7#issuecomment-1",
	})
	m = next.(Model)

	b2, _ := board.Load(m.root)
	got := b2.CardsIn("todo")[0]
	if !strings.Contains(got.Body, "review publicado") {
		t.Errorf("log deveria registrar a publicação:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "issuecomment-1") {
		t.Errorf("log deveria guardar a URL do comentário:\n%s", got.Body)
	}
}

func TestShortPR(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/org/repo/pull/123":  "org/repo#123",
		"https://github.com/org/repo/pull/123/": "org/repo#123",
		"nao-e-url":                             "nao-e-url",
	} {
		if got := shortPR(in); got != want {
			t.Errorf("shortPR(%q) = %q, esperava %q", in, got, want)
		}
	}
}

func TestArchiveCardAsksAndMovesToArchive(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "para arquivar")
	m = press(t, m, "enter")
	card := m.currentCard()
	card.WriteArtifact("refine", "trabalho feito")
	m.reload()

	m = press(t, m, "d")
	if m.mode != modeConfirm {
		t.Fatal("arquivar deveria pedir confirmação")
	}
	if !strings.Contains(m.View(), "artefato") {
		t.Errorf("a confirmação deveria avisar que há artefatos:\n%s", m.View())
	}

	m = press(t, m, "s")
	if len(m.b.CardsIn("todo")) != 0 {
		t.Error("card deveria ter saído do board")
	}

	// E continua existindo no disco — arquivar não apaga.
	entries, err := os.ReadDir(filepath.Join(m.root, board.ArchiveDirName))
	if err != nil {
		t.Fatalf("pasta de arquivo não criada: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("esperava 1 item arquivado, veio %d", len(entries))
	}
	// Como o card tinha artefato, virou pasta — que foi movida inteira.
	arq := filepath.Join(m.root, board.ArchiveDirName, entries[0].Name())
	if _, err := os.Stat(filepath.Join(arq, "refine.md")); err != nil {
		t.Error("o artefato deveria ter ido junto para o arquivo")
	}
}

func TestArchiveCanBeCancelled(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "fica")
	m = press(t, m, "enter")

	m = press(t, m, "d")
	m = press(t, m, "n") // qualquer tecla não afirmativa cancela

	if len(m.b.CardsIn("todo")) != 1 {
		t.Error("cancelar não deveria arquivar")
	}
}

func TestColumnShowsScrollIndicatorWhenFull(t *testing.T) {
	m := newTestModel(t)
	m.height = 14 // espaço para poucos cards
	for i := 0; i < 8; i++ {
		m = press(t, m, "n")
		m = typeText(t, m, "card")
		m = press(t, m, "enter")
	}

	// Com o cursor no último card, a janela rolou até o fim: há cards acima.
	if out := m.View(); !strings.Contains(out, "↑") {
		t.Errorf("cursor no fim deveria indicar cards acima:\n%s", out)
	}

	// Voltando ao topo, o indicador inverte.
	m = press(t, m, "g")
	out := m.View()
	if !strings.Contains(out, "↓") {
		t.Errorf("cursor no topo deveria indicar cards abaixo:\n%s", out)
	}
	if strings.Contains(out, "↑") {
		t.Errorf("no topo não deveria haver nada acima:\n%s", out)
	}
}

func TestBoardShowsHorizontalIndicatorWhenNarrow(t *testing.T) {
	m := newTestModel(t)
	m.width = 40 // cabe uma ou duas colunas das seis

	out := m.View()
	if !strings.Contains(out, "›") {
		t.Errorf("board estreito deveria indicar colunas fora da tela:\n%s", out)
	}
}

func TestDetailScrolls(t *testing.T) {
	m := newTestModel(t)
	m.height = 20
	m = press(t, m, "n")
	m = typeText(t, m, "longo")
	m = press(t, m, "enter")

	// Corpo maior que a janela.
	card := m.currentCard()
	body := ""
	for i := 0; i < 60; i++ {
		body += "linha de conteúdo\n"
	}
	card.Body = body
	card.Save()
	m.reload()

	m = press(t, m, "enter")
	first := m.View()
	if !strings.Contains(first, "linha 1-") {
		t.Errorf("detalhe longo deveria mostrar a posição:\n%s", first)
	}

	m = press(t, m, "j", "j", "j")
	if m.detailOffset != 3 {
		t.Errorf("j deveria rolar, offset=%d", m.detailOffset)
	}
	if strings.Contains(m.View(), "linha 1-") {
		t.Error("a janela deveria ter andado")
	}

	m = press(t, m, "g")
	if m.detailOffset != 0 {
		t.Error("g deveria voltar ao topo")
	}
	// k no topo não pode ir a negativo.
	m = press(t, m, "k", "k")
	if m.detailOffset != 0 {
		t.Errorf("offset não pode ficar negativo: %d", m.detailOffset)
	}
}

func TestFilterNarrowsCards(t *testing.T) {
	m := newTestModel(t)
	for _, title := range []string{"Corrigir login", "Migrar auth", "Ajustar rodapé"} {
		m = press(t, m, "n")
		m = typeText(t, m, title)
		m = press(t, m, "enter")
	}

	m = press(t, m, "/")
	m = typeText(t, m, "auth")
	m = press(t, m, "enter")

	got := m.cardsIn("todo")
	if len(got) != 1 || got[0].Title != "Migrar auth" {
		t.Errorf("filtro deveria deixar só 'Migrar auth', veio %d cards", len(got))
	}
	if !strings.Contains(m.View(), "filtro") {
		t.Errorf("o filtro ativo deveria aparecer no rodapé:\n%s", m.View())
	}

	m = press(t, m, "esc")
	if m.filter != "" {
		t.Error("esc deveria limpar o filtro")
	}
	if len(m.cardsIn("todo")) != 3 {
		t.Error("limpar o filtro deveria trazer todos de volta")
	}
}

func TestFilterSearchesBody(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "Tarefa opaca")
	m = press(t, m, "enter")

	card := m.currentCard()
	card.Body = "## Contexto\n\nO token de refresh expira cedo demais.\n"
	card.Save()
	m.reload()

	m.filter = "refresh"
	if len(m.cardsIn("todo")) != 1 {
		t.Error("a busca deveria encontrar pelo corpo, não só pelo título")
	}
}

func TestFilterKeepsCursorConsistent(t *testing.T) {
	m := newTestModel(t)
	for _, title := range []string{"aaa", "bbb", "ccc"} {
		m = press(t, m, "n")
		m = typeText(t, m, title)
		m = press(t, m, "enter")
	}
	m.cardIdx = 2

	m.filter = "bbb"
	m.clampCursor()
	if c := m.currentCard(); c == nil || c.Title != "bbb" {
		t.Errorf("cursor deveria cair no único card visível, veio %v", c)
	}
}

func TestSetGitHubPRThroughUI(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "com PR")
	m = press(t, m, "enter")

	m = press(t, m, "u")
	if m.mode != modeInput {
		t.Fatal("'u' deveria abrir a entrada do link")
	}
	m = typeText(t, m, "https://github.com/org/repo/pull/42")
	m = press(t, m, "enter")

	b2, _ := board.Load(m.root)
	got := b2.CardsIn("todo")[0]
	if got.GitHubPR != "https://github.com/org/repo/pull/42" {
		t.Errorf("link não persistiu: %q", got.GitHubPR)
	}
	if !strings.Contains(m.status, "org/repo#42") {
		t.Errorf("status deveria confirmar o PR, veio %q", m.status)
	}
}

func TestReleaseAgentAsksAndWarnsWhenWorking(t *testing.T) {
	m := newTestModel(t)
	m, card := cardWithAgent(t, m, "ocupado", "card-ocupado")
	m.herdrInside = true
	m.agents["card-ocupado"] = herdr.Agent{Name: "card-ocupado", Status: herdr.StatusWorking}

	m = press(t, m, "c")
	if m.mode != modeConfirm {
		t.Fatal("fechar o pane deveria pedir confirmação")
	}
	if !strings.Contains(m.View(), "AINDA ESTÁ TRABALHANDO") {
		t.Errorf("deveria avisar que o agente está trabalhando:\n%s", m.View())
	}
	_ = card
}

func TestAgentReleasedClearsCard(t *testing.T) {
	m := newTestModel(t)
	m, card := cardWithAgent(t, m, "libera", "card-libera")
	m.agents["card-libera"] = herdr.Agent{Name: "card-libera"}

	next, _ := m.agentReleased(agentReleasedMsg{cardPath: card.Path, name: "card-libera"})
	m = next.(Model)

	b2, _ := board.Load(m.root)
	got := b2.CardsIn("refine")[0]
	if got.Agent != nil {
		t.Error("a referência ao agente deveria ter sido removida do card")
	}
	if !strings.Contains(got.Body, "liberado") {
		t.Errorf("o log deveria registrar a liberação:\n%s", got.Body)
	}
	if _, ok := m.agents["card-libera"]; ok {
		t.Error("agente deveria ter saído do mapa em memória")
	}
}

func TestReleaseAgentWithoutAgentWarns(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "sem agente")
	m = press(t, m, "enter")
	m.herdrInside = true

	m = press(t, m, "c")
	if m.statusOK || !strings.Contains(m.status, "não tem agente") {
		t.Errorf("deveria avisar que não há agente, status=%q", m.status)
	}
}

func TestToggleCheckboxThroughUI(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "com criterios")
	m = press(t, m, "enter")

	card := m.currentCard()
	card.Body = "## Critério de aceite\n\n- [ ] primeiro\n- [ ] segundo\n"
	card.Save()
	m.reload()

	m = press(t, m, "enter") // abre o detalhe
	if !strings.Contains(plain(m.View()), "1-9 marca item") {
		t.Errorf("o rodapé deveria oferecer as teclas de item:\n%s", plain(m.View()))
	}

	m = press(t, m, "2")
	b2, _ := board.Load(m.root)
	got := b2.CardsIn("todo")[0]
	if !strings.Contains(got.Body, "- [ ] primeiro") {
		t.Errorf("item 1 não deveria ter mudado:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "- [x] segundo") {
		t.Errorf("item 2 deveria ter sido marcado:\n%s", got.Body)
	}
	if !strings.Contains(m.status, "marcado") {
		t.Errorf("status deveria confirmar, veio %q", m.status)
	}

	// Apertar de novo desmarca.
	m = press(t, m, "2")
	b3, _ := board.Load(m.root)
	if !strings.Contains(b3.CardsIn("todo")[0].Body, "- [ ] segundo") {
		t.Error("apertar de novo deveria desmarcar")
	}
}

func TestToggleCheckboxOnArtifactTabIsRefused(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "com artefato")
	m = press(t, m, "enter")

	card := m.currentCard()
	card.Body = "- [ ] um\n"
	card.Save()
	card.WriteArtifact("refine", "# outro\n\n- [ ] item do artefato\n")
	m.reload()

	m = press(t, m, "enter")
	m = press(t, m, "tab") // vai para a aba do artefato
	m = press(t, m, "1")

	if m.statusOK {
		t.Error("marcar item fora da aba do card deveria ser recusado")
	}
	b2, _ := board.Load(m.root)
	if !strings.Contains(b2.CardsIn("todo")[0].Body, "- [ ] um") {
		t.Error("o item do card não deveria ter sido tocado")
	}
}

func TestDetailRendersMarkdown(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "markdown")
	m = press(t, m, "enter")

	card := m.currentCard()
	card.Body = "## Seção\n\ntexto **forte** e `codigo`\n"
	card.Save()
	m.reload()

	m = press(t, m, "enter")
	out := m.View()

	// O glamour aplica estilo: os asteriscos e crases somem do texto visível.
	stripped := plain(out)
	if strings.Contains(stripped, "**forte**") {
		t.Errorf("o negrito deveria ter sido renderizado, não literal:\n%s", stripped)
	}
	if !strings.Contains(stripped, "forte") {
		t.Errorf("o texto deveria continuar lá:\n%s", stripped)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Error("o detalhe deveria sair estilizado")
	}
}

func TestArchiveColumnAsksAndPreservesFile(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "l", "l", "l") // code-review, que tem prompt

	if m.currentColumn().Key != "code-review" {
		t.Fatalf("esperava code-review, está em %q", m.currentColumn().Key)
	}
	before := len(m.columns())

	m = press(t, m, "x")
	if m.mode != modeConfirm {
		t.Fatal("remover coluna deveria pedir confirmação")
	}
	if !strings.Contains(plain(m.View()), "tem prompt escrito") {
		t.Errorf("a confirmação deveria avisar que há prompt:\n%s", plain(m.View()))
	}

	// Cancelar mantém tudo.
	m = press(t, m, "n")
	if len(m.columns()) != before {
		t.Fatal("cancelar não deveria remover a coluna")
	}

	// Confirmar arquiva, sem apagar.
	m = press(t, m, "x")
	m = press(t, m, "s")
	if len(m.columns()) != before-1 {
		t.Errorf("coluna deveria ter saído do board: %d", len(m.columns()))
	}

	arq := filepath.Join(m.root, board.ArchiveDirName, "columns", "code-review.md")
	raw, err := os.ReadFile(arq)
	if err != nil {
		t.Fatalf("a coluna deveria ter sido arquivada, não apagada: %v", err)
	}
	if !strings.Contains(string(raw), "Revise o código") {
		t.Error("o prompt deveria ter sobrevivido ao arquivamento")
	}
}

func TestArchiveColumnRefusesWithCardsBeforeAsking(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "ocupa")
	m = press(t, m, "enter")

	m = press(t, m, "x")
	if m.mode == modeConfirm {
		t.Error("não deveria nem perguntar se a coluna tem cards")
	}
	if m.statusOK || !strings.Contains(m.status, "mova-os") {
		t.Errorf("deveria explicar por que recusou, veio %q", m.status)
	}
}

func TestWaitingCounterAppearsInFooter(t *testing.T) {
	m := newTestModel(t)
	m, _ = cardWithAgent(t, m, "espera", "card-espera")
	m.agents["card-espera"] = herdr.Agent{Name: "card-espera", Status: herdr.StatusBlocked}

	out := plain(m.View())
	if !strings.Contains(out, "1 agente espera") {
		t.Errorf("o rodapé deveria contar quem espera resposta:\n%s", out)
	}
	if !strings.Contains(out, "te espera") {
		t.Errorf("o card deveria dizer que o agente espera você:\n%s", out)
	}

	// Plural.
	m.agents["outro"] = herdr.Agent{Name: "outro", Status: herdr.StatusBlocked}
	if !strings.Contains(plain(m.View()), "2 agentes esperam") {
		t.Error("deveria pluralizar")
	}

	// Quem está trabalhando não conta.
	m.agents["outro"] = herdr.Agent{Name: "outro", Status: herdr.StatusWorking}
	if !strings.Contains(plain(m.View()), "1 agente espera") {
		t.Error("só bloqueados deveriam contar")
	}
}

func TestBoardErrorsDoNotEatTheHints(t *testing.T) {
	m := newTestModel(t)
	// Card apontando para coluna inexistente: erro permanente até ser corrigido.
	os.WriteFile(filepath.Join(m.root, "cards", "orfao.md"),
		[]byte("---\nid: orfao\ncolumn: fantasma\n---\n"), 0o644)
	m.reload()

	if len(m.b.Errors) == 0 {
		t.Fatal("o board deveria ter registrado o problema")
	}

	out := plain(m.View())
	if !strings.Contains(out, "⚠ 1") {
		t.Errorf("deveria haver um chip de problema:\n%s", out)
	}
	// O ponto do conserto: as teclas continuam visíveis.
	if !strings.Contains(out, "? ajuda") || !strings.Contains(out, "q sair") {
		t.Errorf("o guia de teclas não pode ser engolido pelo erro:\n%s", out)
	}

	// E a lista completa fica a uma tecla.
	m = press(t, m, "!")
	if m.mode != modeErrors {
		t.Fatal("'!' deveria abrir a lista de problemas")
	}
	det := plain(m.View())
	if !strings.Contains(det, "coluna inexistente") {
		t.Errorf("a lista deveria explicar o problema:\n%s", det)
	}
	if !strings.Contains(det, "coluna ?") {
		t.Errorf("deveria dizer onde os cards foram parar:\n%s", det)
	}
}

func TestLinkAgentRecordsWorktree(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "com worktree")
	m = press(t, m, "enter")

	card := m.currentCard()
	m.linkAgent(card,
		&herdr.Agent{Name: "card-wt", PaneID: "w2:p1", Kind: "claude"},
		"w2", "/tmp/repo-deck-card-wt")

	b2, _ := board.Load(m.root)
	got := b2.CardsIn("todo")[0]
	if got.Agent == nil || got.Agent.Workspace != "w2" {
		t.Errorf("workspace não persistiu: %+v", got.Agent)
	}
	if got.Agent.Worktree != "/tmp/repo-deck-card-wt" {
		t.Errorf("worktree não persistiu: %+v", got.Agent)
	}
	if !strings.Contains(got.Body, "worktree /tmp/repo-deck-card-wt") {
		t.Errorf("o log deveria citar a worktree:\n%s", got.Body)
	}
}

func TestReleaseMentionsWorktreeInConfirmation(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "com wt")
	m = press(t, m, "enter")

	card := m.currentCard()
	card.Agent = &board.AgentRef{
		Name: "card-wt", Pane: "w2:p1", Workspace: "w2", Worktree: "/tmp/wt",
	}
	card.Save()
	m.reload()
	m.herdrInside = true

	m = press(t, m, "c")
	out := plain(m.View())
	if !strings.Contains(out, "remover a worktree") {
		t.Errorf("a confirmação deveria avisar sobre a worktree:\n%s", out)
	}
	if !strings.Contains(out, "não commitado") {
		t.Errorf("deveria avisar que recusa com trabalho pendente:\n%s", out)
	}
}

func TestReleaseReportsWorktreeLeftBehind(t *testing.T) {
	m := newTestModel(t)
	m, card := cardWithAgent(t, m, "wt suja", "card-suja")

	next, _ := m.agentReleased(agentReleasedMsg{
		cardPath:    card.Path,
		name:        "card-suja",
		worktreeErr: errWorktreeDirty,
	})
	m = next.(Model)

	// O pane fechou; a worktree ficou. O usuário precisa saber disso.
	if m.statusOK || !strings.Contains(m.status, "worktree ficou") {
		t.Errorf("deveria avisar que a worktree sobrou, veio %q", m.status)
	}
}

func TestWorktreeToggleRespectsConfig(t *testing.T) {
	m := newTestModel(t)
	if !m.worktreeEnabled() {
		t.Error("padrão deveria usar worktree")
	}
	m.b.Config.Worktree = board.ToggleOff
	if m.worktreeEnabled() {
		t.Error("worktree: off deveria desligar")
	}
}

func TestImportRefusedWithGitHubOff(t *testing.T) {
	m := newTestModel(t)
	m.ghEnabled = false
	m = press(t, m, "I")
	if m.statusOK || !strings.Contains(m.status, "GitHub desligado") {
		t.Errorf("deveria avisar que o GitHub está desligado, veio %q", m.status)
	}
}

func TestImportedCreatesCardFromIssue(t *testing.T) {
	m := newTestModel(t)
	m.ghEnabled = true

	next, _ := m.imported(importedMsg{
		column: "todo",
		item: &gh.Item{
			Number: 42,
			Title:  "Corrigir refresh de token",
			Body:   "O token expira cedo demais.",
			URL:    "https://github.com/org/repo/issues/42",
		},
	})
	m = next.(Model)

	cards := m.b.CardsIn("todo")
	if len(cards) != 1 {
		t.Fatalf("esperava 1 card, veio %d", len(cards))
	}
	got := cards[0]
	if got.Title != "Corrigir refresh de token" {
		t.Errorf("título errado: %q", got.Title)
	}
	if got.GitHubIssue != "https://github.com/org/repo/issues/42" {
		t.Errorf("issue não foi ligada: %q", got.GitHubIssue)
	}
	if got.GitHubPR != "" {
		t.Error("issue não deveria virar github_pr")
	}
	if !strings.Contains(got.Body, "Descrição original") {
		t.Errorf("o texto do GitHub deveria entrar em seção própria:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "O token expira cedo demais") {
		t.Errorf("corpo original perdido:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "Critério de aceite") {
		t.Error("o card importado deveria ter a seção de critérios")
	}
}

func TestImportedPRSetsGitHubPR(t *testing.T) {
	m := newTestModel(t)
	next, _ := m.imported(importedMsg{
		column: "todo",
		item: &gh.Item{
			Number: 7, Title: "Um PR", IsPR: true,
			URL: "https://github.com/org/repo/pull/7",
		},
	})
	m = next.(Model)

	got := m.b.CardsIn("todo")[0]
	if got.GitHubPR != "https://github.com/org/repo/pull/7" {
		t.Errorf("PR não foi ligado: %q", got.GitHubPR)
	}
	if got.GitHubIssue != "" {
		t.Error("PR não deveria virar github_issue")
	}
}

func TestImportErrorIsReported(t *testing.T) {
	m := newTestModel(t)
	next, _ := m.imported(importedMsg{err: errImportFailed})
	m = next.(Model)
	if m.statusOK || m.status == "" {
		t.Error("erro de importação deveria aparecer na barra")
	}
	if len(m.b.CardsIn("todo")) != 0 {
		t.Error("erro não deveria criar card")
	}
}
