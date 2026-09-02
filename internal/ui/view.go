package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/matheusalbuquerque/deck/internal/board"
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
	case modeDetail:
		return m.viewDetail()
	}

	body := m.viewBoard()
	return lipgloss.JoinVertical(lipgloss.Left, body, m.viewFooter())
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

	rendered := make([]string, 0, len(cols))
	for i, col := range cols {
		rendered = append(rendered, m.viewColumn(col, i == m.colIdx, width, avail))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
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
	cards := m.b.CardsIn(col.Key)

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
	}
	for i, card := range cards {
		selected := focused && i == m.cardIdx
		lines = append(lines, m.viewCard(card, selected, width))
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
	meta := styleCardMeta.Render(relativeTime(card.Updated))

	return style.Width(width - 2).Render(title + "\n" + meta)
}

func (m Model) viewFooter() string {
	// Modo de entrada de texto ocupa o rodapé.
	if m.mode == modeInput {
		label := map[inputKind]string{
			inputNewCard:      "novo card",
			inputNewColumn:    "nova coluna",
			inputRenameColumn: "renomear coluna",
		}[m.inputKind]
		return stylePrompt.Render(label+":") + " " + m.input.View()
	}

	if m.errMsg != "" {
		return styleErrBar.Render("⚠ " + m.errMsg)
	}
	if m.status != "" {
		if m.statusOK {
			return styleOKBar.Render("✓ " + m.status)
		}
		return styleErrBar.Render("✗ " + m.status)
	}
	return styleStatus.Render("h/l colunas · j/k cards · H/L mover · n novo · e editar · p prompt · ? ajuda · q sair")
}

// viewDetail mostra o card inteiro. A renderização com glamour entra na fase 2.
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
	meta := styleCardMeta.Render(fmt.Sprintf(
		"%s · atualizado %s · %s",
		card.ID, relativeTime(card.Updated), card.Column,
	))

	content := strings.Join([]string{head, meta, "", strings.TrimSpace(card.Body)}, "\n")
	box := styleOverlay.Width(width).Render(content)

	hint := styleStatus.Render("esc fecha · e edita no $EDITOR")
	return lipgloss.JoinVertical(lipgloss.Left, box, hint)
}

func (m Model) viewHelp() string {
	rows := [][2]string{
		{"h / l", "coluna anterior / próxima"},
		{"j / k", "card abaixo / acima"},
		{"g / G", "primeiro / último card"},
		{"H / L", "mover card para a coluna vizinha"},
		{"J / K", "reordenar card dentro da coluna"},
		{"enter", "abrir o card"},
		{"n", "novo card na coluna focada"},
		{"e", "editar o card no $EDITOR"},
		{"", ""},
		{"p", "editar a coluna (título, config e prompt)"},
		{"a", "nova coluna"},
		{"r", "renomear a coluna focada"},
		{"< / >", "reordenar a coluna"},
		{"x", "remover a coluna (só se estiver vazia)"},
		{"", ""},
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
