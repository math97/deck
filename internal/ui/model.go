package ui

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"

	"github.com/matheusalbuquerque/deck/internal/board"
)

// mode determina quem consome as teclas.
type mode int

const (
	modeNormal mode = iota
	modeInput       // capturando texto (novo card, nova coluna, renomear)
	modeDetail      // overlay com o corpo do card
	modeHelp
)

// inputKind diz o que fazer com o texto quando o usuário confirma.
type inputKind int

const (
	inputNewCard inputKind = iota
	inputNewColumn
	inputRenameColumn
)

// Model é o estado da aplicação.
type Model struct {
	b    *board.Board
	root string

	colIdx  int
	cardIdx int

	mode      mode
	input     textinput.Model
	inputKind inputKind

	status   string
	statusOK bool
	errMsg   string

	width  int
	height int
}

// Mensagens internas.
type fsEventMsg struct{}
type editorDoneMsg struct{ err error }
type clearStatusMsg struct{}

// New monta o modelo inicial a partir de um board já carregado.
func New(b *board.Board) Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 120

	m := Model{
		b:     b,
		root:  b.Root,
		input: ti,
	}
	m.syncErrors()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(watchCmd(m.root), tea.EnterAltScreen)
}

// --- acesso ao estado focado ---

func (m *Model) columns() []*board.Column { return m.b.Columns }

func (m *Model) currentColumn() *board.Column {
	cols := m.columns()
	if len(cols) == 0 || m.colIdx >= len(cols) {
		return nil
	}
	return cols[m.colIdx]
}

func (m *Model) currentCards() []*board.Card {
	col := m.currentColumn()
	if col == nil {
		return nil
	}
	return m.b.CardsIn(col.Key)
}

func (m *Model) currentCard() *board.Card {
	cards := m.currentCards()
	if len(cards) == 0 || m.cardIdx >= len(cards) {
		return nil
	}
	return cards[m.cardIdx]
}

// clampCursor mantém os índices dentro dos limites após qualquer mudança.
func (m *Model) clampCursor() {
	cols := m.columns()
	if len(cols) == 0 {
		m.colIdx, m.cardIdx = 0, 0
		return
	}
	if m.colIdx >= len(cols) {
		m.colIdx = len(cols) - 1
	}
	if m.colIdx < 0 {
		m.colIdx = 0
	}

	n := len(m.currentCards())
	if m.cardIdx >= n {
		m.cardIdx = n - 1
	}
	if m.cardIdx < 0 {
		m.cardIdx = 0
	}
}

// syncErrors traz os problemas de carregamento para a barra de status.
func (m *Model) syncErrors() {
	m.errMsg = ""
	if len(m.b.Errors) > 0 {
		m.errMsg = m.b.Errors[0]
		if len(m.b.Errors) > 1 {
			m.errMsg += fmt.Sprintf(" (+%d)", len(m.b.Errors)-1)
		}
	}
}

func (m *Model) setStatus(ok bool, format string, args ...any) {
	m.status = fmt.Sprintf(format, args...)
	m.statusOK = ok
}

// reload relê o board do disco preservando, na medida do possível, o foco.
func (m *Model) reload() {
	prevCol := ""
	if c := m.currentColumn(); c != nil {
		prevCol = c.Key
	}
	prevCard := ""
	if c := m.currentCard(); c != nil {
		prevCard = c.Path
	}

	b, err := board.Load(m.root)
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	m.b = b
	m.syncErrors()

	// Reencontra a coluna e o card que estavam em foco.
	for i, c := range m.columns() {
		if c.Key == prevCol {
			m.colIdx = i
			break
		}
	}
	for i, c := range m.currentCards() {
		if c.Path == prevCard {
			m.cardIdx = i
			break
		}
	}
	m.clampCursor()
}

// --- comandos ---

// watchCmd instala o watcher de filesystem e devolve o primeiro evento.
func watchCmd(root string) tea.Cmd {
	return func() tea.Msg {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return nil
		}
		w.Add(root)
		w.Add(root + "/columns")
		w.Add(root + "/cards")

		// Espera um evento e debounce: um save do editor gera vários.
		for range w.Events {
			timer := time.NewTimer(120 * time.Millisecond)
		drain:
			for {
				select {
				case <-w.Events:
					timer.Reset(120 * time.Millisecond)
				case <-timer.C:
					break drain
				}
			}
			w.Close()
			return fsEventMsg{}
		}
		return nil
	}
}

// clearStatusCmd apaga a mensagem de status depois de alguns segundos.
func clearStatusCmd() tea.Cmd {
	return tea.Tick(4*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
}

// openEditor suspende o TUI e abre o arquivo no $EDITOR.
func openEditor(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}
