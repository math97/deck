package ui

import (
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matheusalbuquerque/deck/internal/board"
)

// tabs devolve as abas do detalhe: o card em si, mais um artefato por coluna
// que já produziu algo. A ordem segue a das colunas, para que a esteira apareça
// na tela na mesma sequência em que aconteceu.
func (m Model) tabs(card *board.Card) []*board.Artifact {
	if card == nil {
		return nil
	}
	var out []*board.Artifact
	for _, col := range m.columns() {
		if a := card.Artifact(col.Key); a != nil {
			out = append(out, a)
		}
	}
	// Artefatos de colunas que não existem mais entram no fim, para não sumir.
	for _, a := range card.Artifacts {
		if m.b.Column(a.Column) == nil {
			out = append(out, a)
		}
	}
	return out
}

// updateDetail trata as teclas com o card aberto.
func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	card := m.currentCard()
	if card == nil {
		m.mode = modeNormal
		return m, nil
	}
	n := len(m.tabs(card)) + 1 // +1 pela aba do próprio card

	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		m.tabIdx = 0
		m.detailOffset = 0
		return m, nil

	case "tab", "right", "l":
		m.tabIdx = (m.tabIdx + 1) % n
		m.detailOffset = 0 // trocar de aba volta ao topo
		return m, nil

	case "shift+tab", "left", "h":
		m.tabIdx = (m.tabIdx - 1 + n) % n
		m.detailOffset = 0
		return m, nil

	case "j", "down":
		m.detailOffset++
		return m, nil

	case "k", "up":
		if m.detailOffset > 0 {
			m.detailOffset--
		}
		return m, nil

	case "g":
		m.detailOffset = 0
		return m, nil

	case "e":
		// Edita o que está na aba ativa: o card ou o artefato.
		return m, openEditor(m.activePath(card))

	case "o":
		return m.openPR(card)

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		return m.toggleCheckbox(card, int(msg.String()[0]-'0'))
	}
	return m, nil
}

// toggleCheckbox marca ou desmarca um critério de aceite direto no TUI,
// gravando no markdown do card.
func (m Model) toggleCheckbox(card *board.Card, n int) (tea.Model, tea.Cmd) {
	if m.tabIdx != 0 {
		m.setStatus(false, "os itens ficam na aba do card")
		return m, clearStatusCmd()
	}
	box, err := card.ToggleCheckbox(n)
	if err != nil {
		m.setStatus(false, "%v", err)
		return m, clearStatusCmd()
	}

	mark := "desmarcado"
	if box.Checked {
		mark = "marcado"
	}
	m.reload()
	m.setStatus(true, "%s: %s", mark, box.Text)
	return m, clearStatusCmd()
}

// activePath devolve o arquivo por trás da aba ativa.
func (m Model) activePath(card *board.Card) string {
	tabs := m.tabs(card)
	if m.tabIdx == 0 || m.tabIdx-1 >= len(tabs) {
		return card.Path
	}
	return tabs[m.tabIdx-1].Path
}

// openPR abre o pull request associado no browser.
func (m Model) openPR(card *board.Card) (tea.Model, tea.Cmd) {
	url := card.GitHubPR
	if url == "" {
		url = card.GitHubIssue
	}
	if url == "" {
		m.setStatus(false, "card sem github_pr no frontmatter")
		return m, clearStatusCmd()
	}
	return m, tea.Batch(openBrowser(url), clearStatusCmd())
}

// focusFailedMsg e browserFailedMsg levam à barra de status falhas que antes
// eram engolidas: sem elas, apertar a tecla parecia não fazer nada.
type focusFailedMsg struct{ err error }
type browserFailedMsg struct{ err error }

// openBrowser abre uma URL no navegador padrão do sistema.
func openBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		if err := cmd.Start(); err != nil {
			return browserFailedMsg{err: err}
		}
		return nil
	}
}
