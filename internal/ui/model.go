package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"

	"github.com/matheusalbuquerque/deck/internal/board"
	"github.com/matheusalbuquerque/deck/internal/gh"
	"github.com/matheusalbuquerque/deck/internal/herdr"
)

// mode determina quem consome as teclas.
type mode int

const (
	modeNormal mode = iota
	modeInput       // capturando texto (novo card, nova coluna, renomear)
	modeDetail      // overlay com o corpo do card
	modeHelp
	modeConfirm // pergunta sim/não antes de uma ação irreversível
)

// confirmState é a pergunta pendente e o que fazer quando aceita.
type confirmState struct {
	question string
	detail   string
	action   func(Model) (tea.Model, tea.Cmd)
}

// inputKind diz o que fazer com o texto quando o usuário confirma.
type inputKind int

const (
	inputNewCard inputKind = iota
	inputNewColumn
	inputRenameColumn
	inputGitHubPR
	inputFilter
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

	// Estado do GitHub por caminho de card, preenchido pelo poller.
	ghStates  map[string]gh.State
	ghEnabled bool

	// Aba ativa no detalhe do card: 0 é o card, 1+ são os artefatos.
	tabIdx int

	// Agentes vivos no herdr, por nome, e se estamos dentro de uma sessão.
	agents      map[string]herdr.Agent
	herdrInside bool

	// Estado do artefato quando cada agente subiu, por nome de agente. É o que
	// permite dizer se a entrega foi de fato gravada.
	baselines map[string]baseline

	// Baseline do disparo em voo, até o herdr confirmar o nome do agente.
	pendingBaseline baseline

	confirm confirmState

	// Rolagem do corpo no detalhe do card.
	detailOffset int

	// Filtro de busca. Vazio mostra tudo.
	filter string
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
		b:         b,
		root:      b.Root,
		input:     ti,
		ghStates:  map[string]gh.State{},
		agents:    map[string]herdr.Agent{},
		baselines: map[string]baseline{},
	}

	// A config decide o que está ligado; "auto" cai na disponibilidade real.
	cfg := b.Config
	if cfg == nil {
		cfg = board.DefaultConfig()
	}
	m.ghEnabled = cfg.GitHub.Enabled(gh.Available())
	m.herdrInside = cfg.Herdr.Enabled(herdr.Inside())
	m.syncErrors()
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{watchCmd(m.root), tea.EnterAltScreen}
	if m.ghEnabled {
		cmds = append(cmds, pollGitHub(m.b.Cards), scheduleGitHubPoll())
	}
	if m.herdrInside {
		cmds = append(cmds, pollAgents(), scheduleAgentPoll())
	}
	return tea.Batch(cmds...)
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
	return m.cardsIn(col.Key)
}

// cardsIn devolve os cards de uma coluna na ordem em que serão desenhados.
//
// Cards cujo agente está bloqueado ou terminou sobem para o topo: é o que
// precisa de você. A view e o cursor usam esta mesma função, senão o card
// selecionado deixaria de ser o card desenhado.
func (m *Model) cardsIn(key string) []*board.Card {
	cards := m.b.CardsIn(key)

	if m.filter != "" {
		needle := strings.ToLower(m.filter)
		var kept []*board.Card
		for _, c := range cards {
			if matchesCard(c, needle) {
				kept = append(kept, c)
			}
		}
		cards = kept
	}

	if len(m.agents) == 0 {
		return cards
	}

	var urgent, rest []*board.Card
	for _, c := range cards {
		if c.Agent == nil {
			rest = append(rest, c)
			continue
		}
		if a, ok := m.agents[c.Agent.Name]; ok && a.Status.NeedsAttention() {
			urgent = append(urgent, c)
		} else {
			rest = append(rest, c)
		}
	}
	if len(urgent) == 0 {
		return cards
	}
	return append(urgent, rest...)
}

// matchesCard procura o termo no título, no id e no corpo do card. O corpo
// entra de propósito: quase sempre você lembra de um detalhe do contexto, não
// do título exato que deu à tarefa.
func matchesCard(c *board.Card, needle string) bool {
	return strings.Contains(strings.ToLower(c.Title), needle) ||
		strings.Contains(strings.ToLower(c.ID), needle) ||
		strings.Contains(strings.ToLower(c.Body), needle)
}

// agentFor devolve o agente vivo ligado ao card, se houver.
func (m *Model) agentFor(card *board.Card) (herdr.Agent, bool) {
	if card == nil || card.Agent == nil {
		return herdr.Agent{}, false
	}
	a, ok := m.agents[card.Agent.Name]
	return a, ok
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
