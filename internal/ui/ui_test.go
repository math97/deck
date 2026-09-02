package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matheusalbuquerque/deck/internal/board"
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
