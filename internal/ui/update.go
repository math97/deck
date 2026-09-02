package ui

import (
	tea "github.com/charmbracelet/bubbletea"
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
