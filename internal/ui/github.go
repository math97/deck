package ui

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/math97/deck/internal/board"
	"github.com/math97/deck/internal/gh"
)

// ghStatesMsg traz o estado dos PRs, indexado pelo caminho do card.
type ghStatesMsg map[string]gh.State

// ghPollInterval é conservador de propósito: cada consulta é uma chamada de
// rede, e o estado de um PR não muda de segundo em segundo.
const ghPollInterval = 60 * time.Second

// pollGitHub consulta em paralelo todos os cards que têm PR associado.
func pollGitHub(cards []*board.Card) tea.Cmd {
	var targets []*board.Card
	for _, c := range cards {
		if c.GitHubPR != "" {
			targets = append(targets, c)
		}
	}
	if len(targets) == 0 {
		return nil
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		var (
			mu  sync.Mutex
			wg  sync.WaitGroup
			out = make(ghStatesMsg, len(targets))
		)
		// Limita a concorrência: um board grande não deve disparar cinquenta
		// processos gh de uma vez.
		sem := make(chan struct{}, 4)

		for _, c := range targets {
			wg.Add(1)
			go func(card *board.Card) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				st := gh.Fetch(ctx, card.GitHubPR)
				mu.Lock()
				out[card.Path] = st
				mu.Unlock()
			}(c)
		}
		wg.Wait()
		return out
	}
}

// scheduleGitHubPoll agenda a próxima rodada.
func scheduleGitHubPoll() tea.Cmd {
	return tea.Tick(ghPollInterval, func(time.Time) tea.Msg { return ghTickMsg{} })
}

type ghTickMsg struct{}

// ghAuthMsg traz o resultado da verificação de autenticação.
type ghAuthMsg struct{ ok bool }

// importedMsg traz a issue ou PR buscado no GitHub.
type importedMsg struct {
	item   *gh.Item
	column string
	err    error
}

// importFromGitHub busca a issue ou PR. Fora da goroutine da UI: é rede.
func importFromGitHub(url, column string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		item, err := gh.Import(ctx, url)
		return importedMsg{item: item, column: column, err: err}
	}
}

// checkGitHubAuth confirma a sessão do gh fora do caminho crítico da abertura.
func checkGitHubAuth() tea.Cmd {
	return func() tea.Msg { return ghAuthMsg{ok: gh.Authenticated()} }
}
