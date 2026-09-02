package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/matheusalbuquerque/deck/internal/board"
	"github.com/matheusalbuquerque/deck/internal/herdr"
)

const (
	minColWidth = 18
	maxColWidth = 34
)

func (m Model) View() string {
	if m.width == 0 {
		return "carregando…"
	}

	switch m.mode {
	case modeHelp:
		return m.viewHelp()
	case modeErrors:
		return m.viewErrors()
	case modeDetail:
		return m.viewDetail()
	case modeConfirm:
		return lipgloss.JoinVertical(lipgloss.Left, m.viewBoard(), m.viewConfirm())
	default:
		return lipgloss.JoinVertical(lipgloss.Left, m.viewBoard(), m.viewFooter())
	}
}

// viewBoard desenha as colunas lado a lado.
func (m Model) viewBoard() string {
	cols := m.columns()
	if len(cols) == 0 {
		return styleEmpty.Render("nenhuma coluna — crie uma com 'a'")
	}

	width := m.colWidth(len(cols))
	// Altura útil: total menos rodapé e uma linha de folga.
	avail := m.height - 3
	if avail < 4 {
		avail = 4
	}

	// Nem todas as colunas cabem: mostra a janela que contém a focada.
	perPage := m.width / width
	start, end := windowColumns(len(cols), m.colIdx, perPage)

	rendered := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		rendered = append(rendered, m.viewColumn(cols[i], i == m.colIdx, width, avail))
	}
	board := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)

	// Avisa que há colunas fora da tela, senão elas simplesmente sumiriam.
	if start > 0 || end < len(cols) {
		var marks []string
		if start > 0 {
			marks = append(marks, fmt.Sprintf("‹ %d", start))
		}
		if end < len(cols) {
			marks = append(marks, fmt.Sprintf("%d ›", len(cols)-end))
		}
		board = lipgloss.JoinVertical(lipgloss.Left,
			board, styleColCount.Render(" "+strings.Join(marks, "   ")))
	}
	return board
}

// colWidth reparte a largura disponível entre as colunas, dentro de limites que
// mantêm o título legível.
func (m Model) colWidth(n int) int {
	if n == 0 {
		return minColWidth
	}
	w := m.width/n - 1
	if w < minColWidth {
		w = minColWidth
	}
	if w > maxColWidth {
		w = maxColWidth
	}
	return w
}

func (m Model) viewColumn(col *board.Column, focused bool, width, height int) string {
	cards := m.cardsIn(col.Key)

	// Cabeçalho: título, contagem e limite de WIP quando houver.
	count := fmt.Sprintf(" %d", len(cards))
	if col.WIPLimit > 0 {
		count = fmt.Sprintf(" %d/%d", len(cards), col.WIPLimit)
	}
	titleStyle := styleColTitle
	if focused {
		titleStyle = styleColTitleFocus
	}
	title := truncate(col.Title, width-len(count)-2)
	header := titleStyle.Render(title) + styleColCount.Render(count)

	// Indicador de que a coluna dispara agente.
	if col.HasPrompt() {
		header += styleColCount.Render(" ⚡")
	}

	lines := []string{header, ""}

	if len(cards) == 0 {
		lines = append(lines, styleEmpty.Render("vazio"))
	} else {
		// Renderiza tudo, mede, e mostra só o que cabe — com o card em foco
		// sempre dentro da janela.
		boxes := make([]string, len(cards))
		heights := make([]int, len(cards))
		for i, card := range cards {
			selected := focused && i == m.cardIdx
			boxes[i] = m.viewCard(card, selected, width)
			heights[i] = lipgloss.Height(boxes[i])
		}

		sel := 0
		if focused {
			sel = m.cardIdx
		}
		// -2 do cabeçalho, -1 de folga para o indicador de "há mais".
		avail := height - 3
		start, end, above, below := windowCards(heights, sel, avail)

		if above > 0 {
			lines = append(lines, styleColCount.Render(fmt.Sprintf(" ↑ %d", above)))
		}
		lines = append(lines, boxes[start:end]...)
		if below > 0 {
			lines = append(lines, styleColCount.Render(fmt.Sprintf(" ↓ %d", below)))
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		MaxHeight(height).
		Render(content)
}

func (m Model) viewCard(card *board.Card, selected bool, width int) string {
	style := styleCard
	if selected {
		style = styleCardSelected
	}
	// -4: bordas e padding horizontal da caixa do card.
	inner := width - 4
	if inner < 6 {
		inner = 6
	}

	title := wrap(card.Title, inner, 3)

	// Linha de metadados: idade, marca de artefatos, badge do PR.
	meta := relativeTime(card.Updated)
	if n := len(card.Artifacts); n > 0 {
		meta += fmt.Sprintf(" · %d📄", n)
	}
	line := styleCardMeta.Render(meta)

	if a, ok := m.agentFor(card); ok {
		badge := a.Status.Badge()
		switch a.Status {
		case herdr.StatusBlocked:
			line += " " + styleBadgeBad.Render(badge)
		case herdr.StatusWorking, herdr.StatusDone:
			line += " " + styleBadgeOK.Render(badge)
		default:
			line += " " + styleCardMeta.Render(badge)
		}
	}

	if st, ok := m.ghStates[card.Path]; ok {
		badge := st.Badge()
		if st.Healthy() {
			line += " " + styleBadgeOK.Render(badge)
		} else {
			line += " " + styleBadgeBad.Render(badge)
		}
	} else if card.GitHubPR != "" {
		line += " " + styleCardMeta.Render("PR…")
	}

	return style.Width(width - 2).Render(title + "\n" + line)
}

func (m Model) viewFooter() string {
	// Modo de entrada de texto ocupa o rodapé.
	if m.mode == modeInput {
		label := map[inputKind]string{
			inputNewCard:      "novo card",
			inputNewColumn:    "nova coluna",
			inputRenameColumn: "renomear coluna",
			inputGitHubPR:     "link do PR",
			inputFilter:       "buscar",
			inputImport:       "importar do GitHub",
		}[m.inputKind]
		return stylePrompt.Render(label+":") + " " + m.input.View()
	}

	prefix := ""

	// Agentes esperando resposta viram um contador global. Com a rolagem, o
	// card que precisa de você pode estar fora da tela; o número não some.
	if n := m.countWaiting(); n > 0 {
		plural := "agente espera"
		if n > 1 {
			plural = "agentes esperam"
		}
		prefix += styleBadgeBad.Render(fmt.Sprintf(" ◆ %d %s ", n, plural))
	}

	// O filtro ativo é um prefixo permanente, não uma mensagem: ele convive
	// com o status em vez de competir, senão você esquece que está filtrando e
	// acha que os cards sumiram.
	if m.filter != "" {
		n := 0
		for _, col := range m.columns() {
			n += len(m.cardsIn(col.Key))
		}
		prefix += stylePrompt.Render(fmt.Sprintf(" filtro %q", m.filter)) +
			styleCardMeta.Render(fmt.Sprintf(" %d card(s) ", n))
	}

	// Problemas do board são um chip, não uma mensagem que engole o rodapé:
	// um card órfão é permanente até você arrumá-lo, e não pode custar o guia
	// de teclas pelo resto da sessão. `!` abre a lista completa.
	if n := len(m.b.Errors); n > 0 {
		prefix += styleErrBar.Render(fmt.Sprintf(" ⚠ %d ", n)) +
			styleCardMeta.Render("! ")
	}

	if m.status != "" {
		if m.statusOK {
			return prefix + styleOKBar.Render("✓ "+m.status)
		}
		return prefix + styleErrBar.Render("✗ "+m.status)
	}
	hints := "h/l colunas · j/k cards · H/L mover · n novo · e editar"
	if m.herdrInside {
		hints += " · s agente · f pane"
	}
	if m.ghEnabled {
		hints += " · R review"
	}
	if m.filter != "" {
		hints = "esc limpa o filtro · " + hints
	}
	hints += " · ? ajuda · q sair"
	return prefix + styleStatus.Render(hints)
}

// viewConfirm desenha a pergunta no rodapé. O rótulo deixa explícito que o
// padrão é não: só uma tecla afirmativa segue adiante.
func (m Model) viewConfirm() string {
	q := stylePrompt.Render(m.confirm.question)
	if m.confirm.detail != "" {
		q += styleCardMeta.Render("  " + m.confirm.detail)
	}
	return q + styleStatus.Render("   [s/y confirma · qualquer outra tecla cancela]")
}

// viewDetail mostra o card e seus artefatos em abas.
func (m Model) viewDetail() string {
	card := m.currentCard()
	if card == nil {
		return ""
	}

	width := m.width - 8
	if width > 100 {
		width = 100
	}

	head := styleColTitleFocus.Render(card.Title)

	metaParts := []string{card.ID, "atualizado " + relativeTime(card.Updated)}
	if a, ok := m.agentFor(card); ok {
		metaParts = append(metaParts, "agente "+a.Name+": "+string(a.Status))
	}
	if st, ok := m.ghStates[card.Path]; ok {
		metaParts = append(metaParts, st.Detail())
	}
	meta := styleCardMeta.Render(strings.Join(metaParts, " · "))

	// Barra de abas: o card e um artefato por coluna que já produziu algo.
	arts := m.tabs(card)
	labels := []string{"card"}
	for _, a := range arts {
		t := a.Title
		if t == "" {
			t = a.Column
		}
		labels = append(labels, t)
	}
	var tabBar []string
	for i, l := range labels {
		if i == m.tabIdx {
			tabBar = append(tabBar, styleTabActive.Render(l))
		} else {
			tabBar = append(tabBar, styleTabIdle.Render(l))
		}
	}
	tabs := strings.Join(tabBar, styleCardMeta.Render(" │ "))

	// Corpo da aba ativa, renderizado como markdown.
	raw := strings.TrimSpace(card.Body)
	if m.tabIdx > 0 && m.tabIdx-1 < len(arts) {
		content, err := arts[m.tabIdx-1].Read()
		if err != nil {
			raw = "não foi possível ler o artefato: " + err.Error()
		} else {
			raw = strings.TrimSpace(content)
		}
	} else {
		// Só na aba do card os itens ganham número: são eles que se marcam.
		raw = board.NumberCheckboxes(raw)
	}
	body := renderMarkdown(raw, width-4)

	// Janela rolável: j/k andam, e o rodapé diz onde você está.
	maxLines := m.height - 10
	if maxLines < 5 {
		maxLines = 5
	}
	lines := strings.Split(body, "\n")
	scrollInfo := ""
	if len(lines) > maxLines {
		off := m.detailOffset
		if off > len(lines)-maxLines {
			off = len(lines) - maxLines
		}
		if off < 0 {
			off = 0
		}
		scrollInfo = fmt.Sprintf("  linha %d-%d de %d", off+1, off+maxLines, len(lines))
		lines = lines[off : off+maxLines]
		body = strings.Join(lines, "\n")
	}

	content := strings.Join([]string{head, meta, "", tabs, "", body}, "\n")
	box := styleOverlay.Width(width).Render(content)

	hintText := "tab abas · j/k rola · e edita · o abre o PR · esc fecha"
	if m.tabIdx == 0 && len(card.Checkboxes()) > 0 {
		hintText = "1-9 marca item · " + hintText
	}
	hint := styleStatus.Render(hintText + scrollInfo)
	return lipgloss.JoinVertical(lipgloss.Left, box, hint)
}

// viewErrors lista os problemas encontrados ao carregar o board.
func (m Model) viewErrors() string {
	var lines []string
	for _, e := range m.b.Errors {
		lines = append(lines, styleErrBar.Render("⚠ ")+styleHelpDesc.Render(e))
	}
	if len(lines) == 0 {
		lines = append(lines, styleHelpDesc.Render("nenhum problema"))
	}

	content := styleColTitleFocus.Render("problemas no board") + "\n\n" +
		strings.Join(lines, "\n") + "\n\n" +
		styleHelpDesc.Render("cards sem coluna válida aparecem na coluna ? —\nmova-os com H/L ou corrija o campo column no arquivo.")
	return styleOverlay.Render(content) + "\n" + styleStatus.Render("qualquer tecla fecha")
}

func (m Model) viewHelp() string {
	rows := [][2]string{
		{"h / l", "coluna anterior / próxima"},
		{"j / k", "card abaixo / acima"},
		{"g / G", "primeiro / último card"},
		{"H / L", "mover card para a coluna vizinha"},
		{"J / K", "reordenar card dentro da coluna"},
		{"enter", "abrir o card (abas de artefato)"},
		{"tab", "próxima aba, dentro do card aberto"},
		{"o", "abrir o PR do card no browser"},
		{"n", "novo card na coluna focada"},
		{"d", "arquivar o card (pede confirmação)"},
		{"u", "colar o link do PR no card"},
		{"I", "importar uma issue ou PR do GitHub"},
		{"/", "buscar; esc limpa o filtro"},
		{"e", "editar o card no $EDITOR"},
		{"", ""},
		{"s", "subir um agente com o prompt da coluna"},
		{"f", "pular para o pane do agente"},
		{"R", "publicar o review no PR (pede confirmação)"},
		{"c", "fechar o pane do agente (pede confirmação)"},
		{"", ""},
		{"p", "editar a coluna (título, config e prompt)"},
		{"a", "nova coluna"},
		{"r", "renomear a coluna focada"},
		{"< / >", "reordenar a coluna"},
		{"x", "remover a coluna (só se estiver vazia)"},
		{"", ""},
		{"!", "listar os problemas do board"},
		{"?", "esta ajuda"},
		{"q", "sair"},
	}

	var lines []string
	for _, r := range rows {
		if r[0] == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines,
			styleHelpKey.Render(pad(r[0], 8))+styleHelpDesc.Render(r[1]))
	}

	content := styleColTitleFocus.Render("deck") + "\n\n" + strings.Join(lines, "\n")
	return styleOverlay.Render(content) + "\n" + styleStatus.Render("qualquer tecla fecha")
}

// --- helpers de texto ---

func truncate(s string, max int) string {
	r := []rune(s)
	if max <= 1 || len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// wrap quebra o texto em no máximo maxLines linhas de largura width.
func wrap(s string, width, maxLines int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	cur := ""
	for _, w := range words {
		candidate := w
		if cur != "" {
			candidate = cur + " " + w
		}
		if len([]rune(candidate)) > width && cur != "" {
			lines = append(lines, cur)
			cur = w
			if len(lines) == maxLines {
				break
			}
		} else {
			cur = candidate
		}
	}
	if len(lines) < maxLines && cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == maxLines {
		last := lines[maxLines-1]
		if len([]rune(last)) > width {
			lines[maxLines-1] = truncate(last, width)
		}
	}
	return strings.Join(lines, "\n")
}

func pad(s string, n int) string {
	for len([]rune(s)) < n {
		s += " "
	}
	return s
}

// relativeTime formata a idade de forma curta, para caber no card.
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "agora"
	case d < time.Hour:
		return fmt.Sprintf("%dmin", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return t.Format("02/01")
	}
}
