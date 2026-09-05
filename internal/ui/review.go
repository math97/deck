package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/math97/deck/internal/board"
	"github.com/math97/deck/internal/gh"
)

// reviewPostedMsg é o resultado de publicar um review no PR.
type reviewPostedMsg struct {
	cardPath string
	url      string
	err      error
}

// reviewArtifact devolve o artefato de review do card, se a coluna atual for
// uma coluna de review e o arquivo existir com conteúdo.
func (m *Model) reviewArtifact(card *board.Card) (*board.Artifact, *board.Column, error) {
	if card == nil {
		return nil, nil, fmt.Errorf("nenhum card selecionado")
	}
	col := m.b.Column(card.Column)
	if col == nil || !col.PostReview {
		return nil, nil, fmt.Errorf("a coluna %s não publica review no PR", m.columnTitle())
	}
	a := card.Artifact(col.Key)
	if a == nil {
		return nil, nil, fmt.Errorf("ainda não há review — rode o agente com s")
	}
	// Vazio é medido pelo conteúdo, não pelo tamanho: WriteArtifact garante a
	// quebra de linha final, então um review sem nada escrito tem 1 byte e
	// passaria por uma checagem de tamanho — direto para um PR público.
	raw, err := os.ReadFile(a.Path)
	if err != nil || strings.TrimSpace(string(raw)) == "" {
		return nil, nil, fmt.Errorf("o review está vazio")
	}
	return a, col, nil
}

// postReview publica o artefato de review como comentário no PR.
func postReview(card *board.Card, artifactPath string) tea.Cmd {
	cardPath, prURL := card.Path, card.GitHubPR

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		url, err := gh.PostComment(ctx, prURL, artifactPath)
		return reviewPostedMsg{cardPath: cardPath, url: url, err: err}
	}
}

// askPostReview valida e pede confirmação antes de publicar.
//
// Comentar num PR é público e visível para o time inteiro; não é o tipo de
// coisa que deve acontecer por um toque de tecla sem aviso. Com
// github_auto_post ligado no config.md, o usuário abre mão desta confirmação.
func (m Model) askPostReview() (tea.Model, tea.Cmd) {
	if !m.ghEnabled {
		m.setStatus(false, "GitHub desligado — veja github no .deck/config.md")
		return m, clearStatusCmd()
	}

	card := m.currentCard()
	artifact, _, err := m.reviewArtifact(card)
	if err != nil {
		m.setStatus(false, "%v", err)
		return m, clearStatusCmd()
	}
	if card.GitHubPR == "" {
		m.setStatus(false, "card sem github_pr no frontmatter")
		return m, clearStatusCmd()
	}

	if m.b.Config != nil && m.b.Config.GitHubAutoPost {
		m.setStatus(true, "publicando review…")
		return m, postReview(card, artifact.Path)
	}

	m.mode = modeConfirm
	m.confirm = confirmState{
		question: fmt.Sprintf("Publicar o review no PR? (%s)", shortPR(card.GitHubPR)),
		// O aviso é sobre o que sai daqui, não sobre o que entra: o artefato
		// vai inteiro para um PR que pode ser público e não dá para despublicar.
		// Quem escreveu o review foi um agente, então nem sempre foi lido.
		detail: fmt.Sprintf("%s · %s · vai inteiro e público, sem desfazer",
			card.Title, shortName(artifact.Path)),
		action: func(mm Model) (tea.Model, tea.Cmd) {
			mm.setStatus(true, "publicando review…")
			return mm, postReview(card, artifact.Path)
		},
	}
	return m, nil
}

// reviewPosted registra a publicação no card.
func (m Model) reviewPosted(msg reviewPostedMsg) (tea.Model, tea.Cmd) {
	var card *board.Card
	for _, c := range m.b.Cards {
		if c.Path == msg.cardPath {
			card = c
			break
		}
	}
	if msg.err != nil {
		m.setStatus(false, "%v", msg.err)
		return m, clearStatusCmd()
	}
	if card == nil {
		return m, clearStatusCmd()
	}

	where := msg.url
	if where == "" {
		where = card.GitHubPR
	}
	card.AppendLog("review publicado em %s", where)
	if err := card.Save(); err != nil {
		m.setStatus(false, "salvando card: %v", err)
		return m, clearStatusCmd()
	}

	m.reload()
	m.setStatus(true, "review publicado no PR")
	return m, tea.Batch(
		notify("deck: review publicado", card.Title, "done"),
		clearStatusCmd(),
	)
}

// shortPR reduz a URL do PR a "org/repo#123", que é como se fala dele.
func shortPR(url string) string {
	parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
	if len(parts) < 5 {
		return url
	}
	n := len(parts)
	return fmt.Sprintf("%s/%s#%s", parts[n-4], parts[n-3], parts[n-1])
}
