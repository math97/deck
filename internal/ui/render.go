package ui

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// O renderizador é caro de construir e depende só da largura, então guardamos
// um por largura. Sem isso, cada frame do detalhe reconstruiria o tema inteiro.
var (
	renderMu    sync.Mutex
	renderCache = map[int]*glamour.TermRenderer{}
)

// renderMarkdown formata markdown para o terminal, respeitando o tema claro ou
// escuro. Se algo der errado, devolve o texto cru: um card sempre tem que ser
// legível, mesmo que sem enfeite.
func renderMarkdown(md string, width int) string {
	if width < 20 {
		width = 20
	}

	renderMu.Lock()
	r, ok := renderCache[width]
	if !ok {
		// Estilo explícito, não WithAutoStyle: a detecção automática cai num
		// perfil ASCII que deixa "##" e "**" literais na tela, o que é pior do
		// que não renderizar. Quem decide claro ou escuro é o lipgloss, que já
		// faz essa detecção para o resto do board.
		style := "light"
		if lipgloss.HasDarkBackground() {
			style = "dark"
		}
		var err error
		r, err = glamour.NewTermRenderer(
			glamour.WithStandardStyle(style),
			glamour.WithWordWrap(width),
			glamour.WithEmoji(),
		)
		if err != nil {
			renderMu.Unlock()
			return md
		}
		renderCache[width] = r
	}
	renderMu.Unlock()

	out, err := r.Render(md)
	if err != nil {
		return md
	}

	// O glamour emolduora com linhas em branco; num overlay apertado elas são
	// espaço desperdiçado.
	return strings.Trim(out, "\n")
}
