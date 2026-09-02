package ui

import (
	"context"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matheusalbuquerque/deck/internal/board"
	"github.com/matheusalbuquerque/deck/internal/herdr"
)

// agentPollInterval é curto porque o estado do agente muda rápido e a consulta
// é local (socket), não rede.
const agentPollInterval = 2 * time.Second

// agentsMsg traz os agentes vivos, indexados pelo nome.
type agentsMsg map[string]herdr.Agent

// agentTickMsg agenda a próxima rodada do poller.
type agentTickMsg struct{}

// agentStartedMsg é o resultado de disparar um agente para um card.
type agentStartedMsg struct {
	cardPath string
	agent    *herdr.Agent
	err      error
}

// pollAgents consulta os agentes vivos na sessão do herdr.
func pollAgents() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		list, err := herdr.AgentList(ctx)
		if err != nil {
			// Sessão pode ter morrido; o board segue funcionando sem badges.
			return agentsMsg{}
		}
		out := make(agentsMsg, len(list))
		for _, a := range list {
			if a.Name != "" {
				out[a.Name] = a
			}
		}
		return out
	}
}

func scheduleAgentPoll() tea.Cmd {
	return tea.Tick(agentPollInterval, func(time.Time) tea.Msg { return agentTickMsg{} })
}

// startAgent dispara um agente para o card, na coluna em que ele está.
//
// Roda inteiro fora da goroutine da UI: `agent start` só retorna quando o herdr
// detecta o agente pronto, o que pode levar dezenas de segundos, e o board não
// pode congelar nesse meio-tempo.
func startAgent(b *board.Board, card *board.Card, col *board.Column, taken map[string]bool) tea.Cmd {
	return func() tea.Msg {
		fail := func(err error) tea.Msg {
			return agentStartedMsg{cardPath: card.Path, err: err}
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fail(err)
		}

		// Renderiza o prompt antes de mexer no layout: se a coluna não tiver
		// prompt, nada é criado.
		prompt, err := b.RenderPrompt(card, col, cwd)
		if err != nil {
			return fail(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		direction := herdr.SplitDirection(ctx, herdr.PaneID())
		pane, err := herdr.PaneSplit(ctx, cwd, direction)
		if err != nil {
			return fail(err)
		}

		kind := col.AgentKind
		if kind == "" {
			kind = "claude"
		}
		name := herdr.AgentName(card.ID, taken)

		agent, err := herdr.AgentStart(ctx, name, kind, pane.PaneID)
		if err != nil {
			// O pane já existe e o nome segue utilizável; devolvemos o erro
			// para o usuário decidir, sem destruir o que foi criado.
			return fail(err)
		}

		if err := herdr.AgentPrompt(ctx, name, prompt); err != nil {
			return agentStartedMsg{cardPath: card.Path, agent: agent, err: err}
		}
		return agentStartedMsg{cardPath: card.Path, agent: agent}
	}
}
