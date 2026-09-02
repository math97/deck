package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matheusalbuquerque/deck/internal/board"
	"github.com/matheusalbuquerque/deck/internal/herdr"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case fsEventMsg:
		// Alguém editou um arquivo por fora — inclusive um agente.
		m.reload()
		return m, watchCmd(m.root)

	case editorDoneMsg:
		m.reload()
		if msg.err != nil {
			m.setStatus(false, "editor: %v", msg.err)
		}
		return m, nil

	case ghStatesMsg:
		for path, st := range msg {
			m.ghStates[path] = st
		}
		return m, nil

	case ghTickMsg:
		return m, tea.Batch(pollGitHub(m.b.Cards), scheduleGitHubPoll())

	case agentsMsg:
		m.agents = msg
		m.clampCursor()
		return m, nil

	case agentTickMsg:
		return m, tea.Batch(pollAgents(), scheduleAgentPoll())

	case agentStartedMsg:
		return m.agentStarted(msg)

	case clearStatusMsg:
		m.status = ""
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeInput:
			return m.updateInput(msg)
		case modeDetail:
			return m.updateDetail(msg)
		case modeHelp:
			switch msg.String() {
			case "esc", "q", "enter", "?":
				m.mode = modeNormal
			}
			return m, nil
		default:
			return m.updateNormal(msg)
		}
	}
	return m, nil
}

// updateInput trata a captura de texto (novo card, nova coluna, renomear).
func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input.Blur()
		m.input.SetValue("")
		return m, nil

	case "enter":
		value := m.input.Value()
		m.mode = modeNormal
		m.input.Blur()
		m.input.SetValue("")
		return m.commitInput(value)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// commitInput aplica o texto capturado conforme o tipo de entrada.
func (m Model) commitInput(value string) (tea.Model, tea.Cmd) {
	switch m.inputKind {

	case inputNewCard:
		col := m.currentColumn()
		if col == nil {
			return m, nil
		}
		card, err := m.b.NewCard(value, col.Key)
		if err != nil {
			m.setStatus(false, "%v", err)
			return m, clearStatusCmd()
		}
		m.reload()
		// Põe o foco no card recém-criado.
		for i, c := range m.currentCards() {
			if c.Path == card.Path {
				m.cardIdx = i
				break
			}
		}
		m.setStatus(true, "card criado: %s", card.Title)
		return m, clearStatusCmd()

	case inputNewColumn:
		col, err := m.b.NewColumn(value)
		if err != nil {
			m.setStatus(false, "%v", err)
			return m, clearStatusCmd()
		}
		m.reload()
		for i, c := range m.columns() {
			if c.Key == col.Key {
				m.colIdx = i
				break
			}
		}
		m.setStatus(true, "coluna criada: %s", col.Title)
		return m, clearStatusCmd()

	case inputRenameColumn:
		col := m.currentColumn()
		if col == nil {
			return m, nil
		}
		if err := m.b.RenameColumn(col, value); err != nil {
			m.setStatus(false, "%v", err)
			return m, clearStatusCmd()
		}
		m.reload()
		m.setStatus(true, "coluna renomeada")
		return m, clearStatusCmd()
	}
	return m, nil
}

// updateNormal trata a navegação e as ações do board.
func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	case "q", "ctrl+c":
		return m, tea.Quit

	case "?":
		m.mode = modeHelp
		return m, nil

	// --- navegação ---
	case "left", "h":
		m.colIdx--
		m.cardIdx = 0
		m.clampCursor()
		return m, nil

	case "right", "l":
		m.colIdx++
		m.cardIdx = 0
		m.clampCursor()
		return m, nil

	case "up", "k":
		m.cardIdx--
		m.clampCursor()
		return m, nil

	case "down", "j":
		m.cardIdx++
		m.clampCursor()
		return m, nil

	case "g":
		m.cardIdx = 0
		return m, nil

	case "G":
		m.cardIdx = len(m.currentCards()) - 1
		m.clampCursor()
		return m, nil

	// --- mover card entre colunas ---
	case "H":
		return m.moveCard(-1)

	case "L":
		return m.moveCard(+1)

	// --- reordenar card dentro da coluna ---
	case "K":
		return m.shiftCard(-1)

	case "J":
		return m.shiftCard(+1)

	// --- card ---
	case "enter":
		if m.currentCard() != nil {
			m.mode = modeDetail
		}
		return m, nil

	case "n":
		m.inputKind = inputNewCard
		m.mode = modeInput
		m.input.Placeholder = "título do card"
		m.input.Focus()
		return m, nil

	case "e":
		card := m.currentCard()
		if card == nil {
			m.setStatus(false, "nenhum card selecionado")
			return m, clearStatusCmd()
		}
		return m, openEditor(card.Path)

	case "s":
		return m.startAgentForCard()

	case "f":
		return m.focusAgent()

	// --- colunas ---
	case "p":
		col := m.currentColumn()
		if col == nil || col.Path == "" {
			m.setStatus(false, "esta coluna não tem arquivo")
			return m, clearStatusCmd()
		}
		return m, openEditor(col.Path)

	case "a":
		m.inputKind = inputNewColumn
		m.mode = modeInput
		m.input.Placeholder = "título da coluna"
		m.input.Focus()
		return m, nil

	case "r":
		col := m.currentColumn()
		if col == nil {
			return m, nil
		}
		m.inputKind = inputRenameColumn
		m.mode = modeInput
		m.input.Placeholder = "novo título"
		m.input.SetValue(col.Title)
		m.input.CursorEnd()
		m.input.Focus()
		return m, nil

	case "<":
		return m.shiftColumn(-1)

	case ">":
		return m.shiftColumn(+1)

	case "x":
		col := m.currentColumn()
		if col == nil {
			return m, nil
		}
		if err := m.b.DeleteColumn(col); err != nil {
			m.setStatus(false, "%v", err)
			return m, clearStatusCmd()
		}
		m.reload()
		m.setStatus(true, "coluna removida")
		return m, clearStatusCmd()
	}

	return m, nil
}

// moveCard leva o card selecionado para a coluna vizinha.
func (m Model) moveCard(delta int) (tea.Model, tea.Cmd) {
	card := m.currentCard()
	if card == nil {
		return m, nil
	}
	cols := m.columns()
	target := m.colIdx + delta
	if target < 0 || target >= len(cols) {
		return m, nil
	}

	if err := m.b.MoveCard(card, cols[target].Key); err != nil {
		m.setStatus(false, "%v", err)
		return m, clearStatusCmd()
	}

	m.colIdx = target
	m.reload()
	// Segue o card para a coluna de destino.
	for i, c := range m.currentCards() {
		if c.Path == card.Path {
			m.cardIdx = i
			break
		}
	}
	m.setStatus(true, "→ %s", cols[target].Title)
	return m, clearStatusCmd()
}

// shiftCard reordena o card dentro da coluna.
func (m Model) shiftCard(delta int) (tea.Model, tea.Cmd) {
	card := m.currentCard()
	if card == nil {
		return m, nil
	}
	if err := m.b.ShiftCard(card, delta); err != nil {
		m.setStatus(false, "%v", err)
		return m, clearStatusCmd()
	}
	m.reload()
	for i, c := range m.currentCards() {
		if c.Path == card.Path {
			m.cardIdx = i
			break
		}
	}
	return m, nil
}

// shiftColumn reordena a coluna focada.
func (m Model) shiftColumn(delta int) (tea.Model, tea.Cmd) {
	col := m.currentColumn()
	if col == nil {
		return m, nil
	}
	if err := m.b.ShiftColumn(col, delta); err != nil {
		m.setStatus(false, "%v", err)
		return m, clearStatusCmd()
	}
	m.reload()
	for i, c := range m.columns() {
		if c.Key == col.Key {
			m.colIdx = i
			break
		}
	}
	return m, nil
}

// startAgentForCard dispara um agente na coluna em que o card está.
func (m Model) startAgentForCard() (tea.Model, tea.Cmd) {
	if !m.herdrInside {
		m.setStatus(false, "fora de uma sessão do herdr — abra o deck dentro de um pane")
		return m, clearStatusCmd()
	}

	card := m.currentCard()
	if card == nil {
		m.setStatus(false, "nenhum card selecionado")
		return m, clearStatusCmd()
	}
	col := m.currentColumn()
	if col == nil || !col.HasPrompt() {
		m.setStatus(false, "a coluna %s não tem prompt", m.columnTitle())
		return m, clearStatusCmd()
	}
	if a, ok := m.agentFor(card); ok && a.Status != herdr.StatusDone {
		m.setStatus(false, "%s já tem agente (%s) — use f para ir até ele", card.Title, a.Status)
		return m, clearStatusCmd()
	}

	taken := make(map[string]bool, len(m.agents))
	for name := range m.agents {
		taken[name] = true
	}

	m.setStatus(true, "subindo agente para %s…", card.Title)
	return m, startAgent(m.b, card, col, taken)
}

// agentStarted registra no card o agente que subiu.
func (m Model) agentStarted(msg agentStartedMsg) (tea.Model, tea.Cmd) {
	var card *board.Card
	for _, c := range m.b.Cards {
		if c.Path == msg.cardPath {
			card = c
			break
		}
	}

	if msg.err != nil {
		m.setStatus(false, "%v", msg.err)
		// Mesmo com erro no prompt, o agente pode ter subido: registra o que há.
		if card != nil && msg.agent != nil {
			m.linkAgent(card, msg.agent)
		}
		return m, clearStatusCmd()
	}
	if card == nil || msg.agent == nil {
		return m, clearStatusCmd()
	}

	m.linkAgent(card, msg.agent)
	m.setStatus(true, "agente %s rodando em %s", msg.agent.Name, msg.agent.PaneID)
	return m, tea.Batch(pollAgents(), clearStatusCmd())
}

// linkAgent grava a ligação card ↔ agente no frontmatter e no log.
func (m *Model) linkAgent(card *board.Card, agent *herdr.Agent) {
	card.Agent = &board.AgentRef{
		Name: agent.Name,
		Pane: agent.PaneID,
		Kind: agent.Kind,
	}
	card.AppendLog("agente `%s` iniciado em %s", agent.Name, agent.PaneID)
	if err := card.Save(); err != nil {
		m.setStatus(false, "salvando card: %v", err)
		return
	}
	m.agents[agent.Name] = *agent
}

// focusAgent leva o usuário ao pane do agente do card.
func (m Model) focusAgent() (tea.Model, tea.Cmd) {
	card := m.currentCard()
	if card == nil || card.Agent == nil {
		m.setStatus(false, "este card não tem agente")
		return m, clearStatusCmd()
	}
	if !m.herdrInside {
		m.setStatus(false, "fora de uma sessão do herdr")
		return m, clearStatusCmd()
	}
	if _, ok := m.agentFor(card); !ok {
		m.setStatus(false, "o agente %s não está mais vivo", card.Agent.Name)
		return m, clearStatusCmd()
	}

	name := card.Agent.Name
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = herdr.AgentFocus(ctx, name)
		return nil
	}
}

// columnTitle é um atalho seguro para o título da coluna focada.
func (m *Model) columnTitle() string {
	if col := m.currentColumn(); col != nil {
		return col.Title
	}
	return "?"
}
