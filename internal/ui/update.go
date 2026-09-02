package ui

import (
	"context"
	"fmt"
	"strings"
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
		return m, watchCmd(m.fsEvents)

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

	case ghAuthMsg:
		if !msg.ok {
			m.ghEnabled = false
			m.setStatus(false, "gh sem sessão — rode `gh auth login` para ver estado de PR")
			return m, clearStatusCmd()
		}
		return m, nil

	case ghTickMsg:
		return m, tea.Batch(pollGitHub(m.b.Cards), scheduleGitHubPoll())

	case agentsMsg:
		cmds := m.detectFinished(msg)
		m.agents = msg
		m.clampCursor()
		return m, tea.Batch(cmds...)

	case agentTickMsg:
		return m, tea.Batch(pollAgents(), scheduleAgentPoll())

	case agentStartedMsg:
		return m.agentStarted(msg)

	case captureMsg:
		return m.captured(msg)

	case reviewPostedMsg:
		return m.reviewPosted(msg)

	case agentReleasedMsg:
		return m.agentReleased(msg)

	case importedMsg:
		return m.imported(msg)

	case focusFailedMsg:
		m.setStatus(false, "não foi possível focar o agente: %v", msg.err)
		return m, clearStatusCmd()

	case browserFailedMsg:
		m.setStatus(false, "não foi possível abrir o browser: %v", msg.err)
		return m, clearStatusCmd()

	case clearStatusMsg:
		m.status = ""
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeInput:
			return m.updateInput(msg)
		case modeDetail:
			return m.updateDetail(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		case modeHelp, modeErrors:
			m.mode = modeNormal
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

	case inputGitHubPR:
		card := m.currentCard()
		if card == nil {
			return m, nil
		}
		value = strings.TrimSpace(value)
		card.GitHubPR = value
		if err := card.Save(); err != nil {
			m.setStatus(false, "%v", err)
			return m, clearStatusCmd()
		}
		m.reload()
		if value == "" {
			m.setStatus(true, "link do PR removido")
			return m, clearStatusCmd()
		}
		m.setStatus(true, "PR %s ligado ao card", shortPR(value))
		// Consulta o estado do PR agora, sem esperar o próximo ciclo.
		if m.ghEnabled {
			return m, tea.Batch(pollGitHub(m.b.Cards), clearStatusCmd())
		}
		return m, clearStatusCmd()

	case inputImport:
		col := m.currentColumn()
		if col == nil {
			return m, nil
		}
		m.setStatus(true, "buscando no GitHub…")
		return m, importFromGitHub(strings.TrimSpace(value), col.Key)

	case inputFilter:
		m.filter = strings.TrimSpace(value)
		m.cardIdx = 0
		m.clampCursor()
		if m.filter == "" {
			m.setStatus(true, "filtro limpo")
		} else {
			m.setStatus(true, "filtrando por %q — esc limpa", m.filter)
		}
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

	case "!":
		m.mode = modeErrors
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

	case "R":
		return m.askPostReview()

	case "d":
		return m.askArchiveCard()

	case "u":
		card := m.currentCard()
		if card == nil {
			m.setStatus(false, "nenhum card selecionado")
			return m, clearStatusCmd()
		}
		m.inputKind = inputGitHubPR
		m.mode = modeInput
		m.input.Placeholder = "https://github.com/org/repo/pull/123"
		m.input.SetValue(card.GitHubPR)
		m.input.CursorEnd()
		m.input.Focus()
		return m, nil

	case "I":
		if !m.ghEnabled {
			m.setStatus(false, "GitHub desligado — veja github no .deck/config.md")
			return m, clearStatusCmd()
		}
		m.inputKind = inputImport
		m.mode = modeInput
		m.input.Placeholder = "URL da issue ou do PR"
		m.input.SetValue("")
		m.input.Focus()
		return m, nil

	case "/":
		m.inputKind = inputFilter
		m.mode = modeInput
		m.input.Placeholder = "buscar em título, id e corpo"
		m.input.SetValue(m.filter)
		m.input.CursorEnd()
		m.input.Focus()
		return m, nil

	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.clampCursor()
			m.setStatus(true, "filtro limpo")
			return m, clearStatusCmd()
		}
		return m, nil

	case "c":
		return m.askReleaseAgent()

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
		return m.askArchiveColumn()
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

	// O baseline tem que ser tirado antes de o agente rodar: depois, o arquivo
	// que ele gravar já apareceria como pré-existente.
	base := snapshotArtifact(card, col.Key)
	m.pendingBaseline = base

	useWorktree := m.worktreeEnabled()
	where := "subindo agente para %s…"
	if useWorktree {
		where = "criando worktree e subindo agente para %s…"
	}
	m.setStatus(true, where, card.Title)
	return m, startAgent(m.b, card, col, taken, useWorktree)
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
			m.linkAgent(card, msg.agent, msg.kind, msg.workspace, msg.worktree)
		}
		return m, clearStatusCmd()
	}
	if card == nil || msg.agent == nil {
		return m, clearStatusCmd()
	}

	m.linkAgent(card, msg.agent, msg.kind, msg.workspace, msg.worktree)
	if m.pendingBaseline.path == "" {
		m.pendingBaseline = snapshotArtifact(card, card.Column)
	}
	m.baselines[msg.agent.Name] = m.pendingBaseline
	m.pendingBaseline = baseline{}

	if msg.fallback {
		m.setStatus(true, "agente %s rodando com %s (o primeiro provedor recusou)",
			msg.agent.Name, msg.kind)
	} else {
		m.setStatus(true, "agente %s rodando em %s", msg.agent.Name, msg.agent.PaneID)
	}
	return m, tea.Batch(pollAgents(), clearStatusCmd())
}

// linkAgent grava a ligação card ↔ agente no frontmatter e no log.
func (m *Model) linkAgent(card *board.Card, agent *herdr.Agent, kind, workspace, worktree string) {
	if kind == "" {
		kind = agent.Kind
	}
	card.Agent = &board.AgentRef{
		Name:      agent.Name,
		Pane:      agent.PaneID,
		Kind:      kind,
		Workspace: workspace,
		Worktree:  worktree,
	}
	// O provedor entra no log: dias depois é a única forma de saber com quem o
	// trabalho foi feito, sobretudo quando houve troca por cota.
	if worktree != "" {
		card.AppendLog("agente `%s` (%s) iniciado em %s, worktree %s",
			agent.Name, kind, agent.PaneID, worktree)
	} else {
		card.AppendLog("agente `%s` (%s) iniciado em %s", agent.Name, kind, agent.PaneID)
	}
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
		if err := herdr.AgentFocus(ctx, name); err != nil {
			return focusFailedMsg{err: err}
		}
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

// detectFinished compara o estado anterior com o novo e dispara a captura de
// cada agente que acabou de terminar.
//
// A comparação é feita contra m.agents (o estado da rodada anterior) porque o
// herdr reporta `done` de forma estável: sem isso, capturaríamos de novo a cada
// 2 segundos enquanto o agente seguisse pronto.
func (m *Model) detectFinished(next agentsMsg) []tea.Cmd {
	var cmds []tea.Cmd

	for name, now := range next {
		prev, had := m.agents[name]
		if !had {
			continue
		}

		// Bloqueou agora: avisa, mas não captura — o agente não terminou.
		if now.Status == herdr.StatusBlocked && prev.Status != herdr.StatusBlocked {
			if card := m.cardForAgent(name); card != nil {
				cmds = append(cmds, notify("deck: agente bloqueado", card.Title, "request"))
			}
			continue
		}

		if now.Status != herdr.StatusDone || prev.Status == herdr.StatusDone {
			continue
		}

		card := m.cardForAgent(name)
		if card == nil {
			continue
		}
		base := m.baselines[name]
		if base.path == "" {
			// Agente iniciado antes deste processo do deck: sem baseline, o
			// melhor que dá é considerar entregue se o artefato existir.
			base = snapshotArtifact(card, card.Column)
			base.exists = false
		}
		cmds = append(cmds, captureAgentResult(card, name, base))
	}
	return cmds
}

// cardForAgent encontra o card ligado a um nome de agente.
func (m *Model) cardForAgent(name string) *board.Card {
	for _, c := range m.b.Cards {
		if c.Agent != nil && c.Agent.Name == name {
			return c
		}
	}
	return nil
}

// captured registra no card o desfecho da rodada do agente.
func (m Model) captured(msg captureMsg) (tea.Model, tea.Cmd) {
	var card *board.Card
	for _, c := range m.b.Cards {
		if c.Path == msg.cardPath {
			card = c
			break
		}
	}
	if card == nil {
		return m, nil
	}

	var line, status string
	switch {
	case msg.delivered:
		line = fmt.Sprintf("agente terminou · %s gravado (%s)",
			shortName(msg.artifact), humanSize(msg.size))
		status = card.Title + ": entrega gravada"

	case msg.session != "":
		line = fmt.Sprintf("agente terminou SEM gravar %s · transcrição em %s",
			shortName(msg.artifact), shortName(msg.session))
		status = card.Title + ": sem entrega, transcrição salva"

	default:
		line = "agente terminou sem gravar a entrega"
		if msg.err != nil {
			line += " · " + msg.err.Error()
		}
		status = card.Title + ": agente terminou sem entrega"
	}

	card.AppendLog("%s", line)
	if err := card.Save(); err != nil {
		m.setStatus(false, "salvando card: %v", err)
		return m, clearStatusCmd()
	}

	m.reload()
	m.setStatus(msg.delivered, "%s", status)
	return m, tea.Batch(
		notify("deck: agente terminou", status, "done"),
		clearStatusCmd(),
	)
}

// shortName encurta um caminho para o nome do arquivo, que é o que informa.
func shortName(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}

// humanSize formata bytes de forma curta.
func humanSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

// updateConfirm trata a pergunta sim/não. O padrão é NÃO: qualquer tecla que
// não seja uma aceitação explícita cancela.
func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "s", "Y", "S", "enter":
		action := m.confirm.action
		m.mode = modeNormal
		m.confirm = confirmState{}
		if action != nil {
			return action(m)
		}
		return m, nil
	default:
		m.mode = modeNormal
		m.confirm = confirmState{}
		m.setStatus(false, "cancelado")
		return m, clearStatusCmd()
	}
}

// askArchiveCard tira o card do board, com confirmação.
//
// Arquivar move o card (e os artefatos, quando houver) para .deck/archive/.
// Nada é destruído — mas some da tela, e some da tela sem aviso é o tipo de
// coisa que assusta, então confirma.
func (m Model) askArchiveCard() (tea.Model, tea.Cmd) {
	card := m.currentCard()
	if card == nil {
		m.setStatus(false, "nenhum card selecionado")
		return m, clearStatusCmd()
	}

	detail := card.Title
	if n := len(card.Artifacts); n > 0 {
		detail += fmt.Sprintf(" (+%d artefato(s))", n)
	}

	m.mode = modeConfirm
	m.confirm = confirmState{
		question: "Arquivar este card?",
		detail:   detail,
		action: func(mm Model) (tea.Model, tea.Cmd) {
			if err := mm.b.ArchiveCard(card); err != nil {
				mm.setStatus(false, "%v", err)
				return mm, clearStatusCmd()
			}
			mm.reload()
			mm.setStatus(true, "arquivado em .deck/archive — nada foi apagado")
			return mm, clearStatusCmd()
		},
	}
	return m, nil
}

// askReleaseAgent fecha o pane do agente ligado ao card.
//
// Sem isso, um card que passou por Refine, In Progress e QA deixa panes vivos
// para sempre. Fecha só panes que o próprio deck criou, e com confirmação:
// fechar um pane mata o processo dentro dele.
func (m Model) askReleaseAgent() (tea.Model, tea.Cmd) {
	card := m.currentCard()
	if card == nil || card.Agent == nil {
		m.setStatus(false, "este card não tem agente")
		return m, clearStatusCmd()
	}
	if !m.herdrInside {
		m.setStatus(false, "fora de uma sessão do herdr")
		return m, clearStatusCmd()
	}

	agent := *card.Agent
	warn := ""
	if a, ok := m.agentFor(card); ok && a.Status == herdr.StatusWorking {
		warn = " — ELE AINDA ESTÁ TRABALHANDO"
	}

	question := "Fechar o pane do agente?"
	detail := agent.Name + " · " + agent.Pane
	if agent.Worktree != "" {
		question = "Fechar o pane e remover a worktree?"
		detail += " · " + agent.Worktree + " (recusa se houver trabalho não commitado)"
	}

	m.mode = modeConfirm
	m.confirm = confirmState{
		question: question + warn,
		detail:   detail,
		action: func(mm Model) (tea.Model, tea.Cmd) {
			return mm, releaseAgent(card, agent)
		},
	}
	return m, nil
}

// releaseAgent fecha o pane e desliga a referência no card.
func releaseAgent(card *board.Card, agent board.AgentRef) tea.Cmd {
	cardPath := card.Path
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		if err := herdr.PaneClose(ctx, agent.Pane); err != nil {
			return agentReleasedMsg{cardPath: cardPath, name: agent.Name, err: err}
		}

		// A worktree sai depois do pane, e sem --force: com trabalho não
		// commitado o herdr recusa, e recusar é o comportamento certo.
		msg := agentReleasedMsg{cardPath: cardPath, name: agent.Name}
		if agent.Workspace != "" {
			if err := herdr.WorktreeRemove(ctx, agent.Workspace); err != nil {
				msg.worktreeErr = err
			} else {
				msg.worktree = agent.Worktree
			}
		}
		return msg
	}
}

// agentReleased limpa a ligação no card depois de fechar o pane.
func (m Model) agentReleased(msg agentReleasedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setStatus(false, "%v", msg.err)
		return m, clearStatusCmd()
	}

	for _, c := range m.b.Cards {
		if c.Path == msg.cardPath {
			c.Agent = nil
			c.AppendLog("agente `%s` liberado", msg.name)
			if err := c.Save(); err != nil {
				m.setStatus(false, "salvando card: %v", err)
				return m, clearStatusCmd()
			}
			break
		}
	}
	delete(m.agents, msg.name)
	delete(m.baselines, msg.name)

	m.reload()
	switch {
	case msg.worktreeErr != nil:
		m.setStatus(false, "pane fechado, mas a worktree ficou: %v", msg.worktreeErr)
	case msg.worktree != "":
		m.setStatus(true, "agente %s liberado e worktree removida", msg.name)
	default:
		m.setStatus(true, "agente %s liberado", msg.name)
	}
	return m, clearStatusCmd()
}

// worktreeEnabled resolve o interruptor de worktree da config.
func (m *Model) worktreeEnabled() bool {
	if m.b.Config == nil {
		return true
	}
	// Em "auto" a decisão final é do startAgent, que checa se há repositório
	// git; aqui só respeitamos um "off" explícito.
	return m.b.Config.Worktree != board.ToggleOff
}

// askArchiveColumn tira a coluna do board, com confirmação.
//
// A coluna carrega o prompt — o trabalho de escrever como o agente se comporta
// naquela etapa. Perder isso por um toque de tecla era o pior estrago que o
// deck permitia; agora pergunta, e arquiva em vez de apagar.
func (m Model) askArchiveColumn() (tea.Model, tea.Cmd) {
	col := m.currentColumn()
	if col == nil {
		return m, nil
	}
	// Valida antes de perguntar: não faz sentido confirmar algo que vai falhar.
	if m.b.HasCards(col.Key) {
		m.setStatus(false, "%s tem cards — mova-os antes de remover", col.Title)
		return m, clearStatusCmd()
	}

	detail := col.Title
	if col.HasPrompt() {
		detail += " — tem prompt escrito"
	}

	m.mode = modeConfirm
	m.confirm = confirmState{
		question: "Arquivar esta coluna?",
		detail:   detail,
		action: func(mm Model) (tea.Model, tea.Cmd) {
			if err := mm.b.ArchiveColumn(col); err != nil {
				mm.setStatus(false, "%v", err)
				return mm, clearStatusCmd()
			}
			mm.reload()
			mm.setStatus(true, "arquivada em .deck/archive/columns — `deck init` recria as padrão")
			return mm, clearStatusCmd()
		},
	}
	return m, nil
}

// imported cria o card a partir do que veio do GitHub.
func (m Model) imported(msg importedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setStatus(false, "%v", msg.err)
		return m, clearStatusCmd()
	}

	card, err := m.b.NewCardFromSource(
		msg.item.Title, msg.column, msg.item.URL, msg.item.Body, msg.item.IsPR)
	if err != nil {
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

	kind := "issue"
	if msg.item.IsPR {
		kind = "PR"
	}
	m.setStatus(true, "%s #%d importado: %s", kind, msg.item.Number, card.Title)
	return m, tea.Batch(pollGitHub(m.b.Cards), clearStatusCmd())
}
