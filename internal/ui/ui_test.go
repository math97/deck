package ui

import (
	"os"
	"path/filepath"
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
	out := m.View()
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
	if !strings.Contains(m.View(), "Critério de aceite") {
		t.Error("aba inicial deveria mostrar o corpo do card")
	}

	m = press(t, m, "tab")
	if !strings.Contains(m.View(), "as perguntas") {
		t.Errorf("segunda aba deveria mostrar o artefato de refine:\n%s", m.View())
	}

	m = press(t, m, "tab")
	if !strings.Contains(m.View(), "o plano") {
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
	m.linkAgent(card, &herdr.Agent{Name: "card-tarefa", PaneID: "w1:p7", Kind: "claude"})

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
