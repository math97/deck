package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

// agentReleasedMsg é o resultado de fechar o pane de um agente.
type agentReleasedMsg struct {
	cardPath    string
	name        string
	worktree    string // removida com sucesso
	worktreeErr error  // pane fechou, mas a worktree ficou
	err         error
}

// agentStartedMsg é o resultado de disparar um agente para um card.
type agentStartedMsg struct {
	cardPath  string
	agent     *herdr.Agent
	kind      string // provedor que de fato subiu
	fallback  bool   // houve provedor recusado antes deste
	workspace string // preenchido quando o agente ganhou worktree própria
	worktree  string
	pending   string // tarefa que ainda não pôde ser entregue (agente bloqueado)
	err       error
}

// promptSentMsg é a entrega tardia de uma tarefa que esperou o agente liberar.
type promptSentMsg struct {
	name string
	err  error
}

// sendPrompt entrega a tarefa a um agente que já está pronto para recebê-la.
func sendPrompt(name, prompt string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return promptSentMsg{name: name, err: herdr.AgentPrompt(ctx, name, prompt)}
	}
}

// insideGitRepo diz se o diretório está sob controle do git. Sem repositório
// não há worktree possível, e o disparo cai no modo simples em vez de falhar.
func insideGitRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	return cmd.Run() == nil
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
func startAgent(b *board.Board, card *board.Card, col *board.Column, taken map[string]bool, useWorktree bool) tea.Cmd {
	return func() tea.Msg {
		fail := func(err error) tea.Msg {
			return agentStartedMsg{cardPath: card.Path, err: err}
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fail(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		// Onde o agente vai trabalhar. Com worktree, um checkout e um branch
		// só dele; sem, um pane irmão no diretório atual.
		var (
			paneID    string
			workDir   = cwd
			workspace string
			worktree  string
		)

		if useWorktree && insideGitRepo(cwd) {
			wt, err := herdr.WorktreeCreate(ctx, cwd, herdr.BranchName(card.ID), card.Title)
			if err != nil {
				return fail(fmt.Errorf("criando worktree: %w", err))
			}
			paneID = wt.RootPane.PaneID
			workspace = wt.WorkspaceID
			worktree = wt.Worktree.Path
			if worktree != "" {
				workDir = worktree
			}
		} else {
			direction := herdr.SplitDirection(ctx, herdr.PaneID())
			pane, err := herdr.PaneSplit(ctx, cwd, direction)
			if err != nil {
				return fail(err)
			}
			paneID = pane.PaneID
		}

		// O prompt é renderizado com o diretório em que o agente de fato vai
		// trabalhar, senão {{cwd}} apontaria para a árvore errada.
		prompt, err := b.RenderPrompt(card, col, workDir)
		if err != nil {
			return fail(err)
		}

		name := herdr.AgentName(card.ID, taken)

		// Cadeia de provedores: se o primeiro recusar — cota esgotada, binário
		// ausente, sessão expirada — tenta o próximo em vez de desistir. É o
		// que permite continuar trabalhando quando os tokens de um acabam.
		kinds := col.AgentKinds
		if len(kinds) == 0 {
			kinds = []string{"claude"}
		}

		var (
			agent    *herdr.Agent
			used     string
			attempts []string
			err2     error
		)
		for _, kind := range kinds {
			agent, err2 = herdr.AgentStart(ctx, name, kind, paneID)
			// A parada é ter agente, não ausência de erro: um agente que subiu
			// bloqueado numa pergunta vem com os dois. Insistir com o próximo
			// tipo no mesmo pane só produziria agent_pane_busy, e ainda deixaria
			// o primeiro agente vivo e sem dono.
			if agent != nil {
				used = kind
				break
			}
			attempts = append(attempts, fmt.Sprintf("%s: %v", kind, err2))
		}
		if agent == nil {
			// O que foi criado continua de pé e o nome segue utilizável:
			// devolvemos o erro para o usuário decidir, sem destruir nada.
			return fail(fmt.Errorf("nenhum provedor subiu (%s)", strings.Join(attempts, "; ")))
		}

		msg := agentStartedMsg{
			cardPath: card.Path, agent: agent, kind: used,
			fallback:  len(attempts) > 0,
			workspace: workspace, worktree: worktree,
		}
		// Agente parado numa pergunta não aceita tarefa ainda — o herdr recusa
		// com agent_blocked. A tarefa fica pendente e o poller a entrega assim
		// que ele liberar; mandar agora só perderia o prompt.
		if agent.Status == herdr.StatusBlocked || err2 != nil {
			msg.pending = prompt
			return msg
		}
		if err := herdr.AgentPrompt(ctx, name, prompt); err != nil {
			msg.err = err
		}
		return msg
	}
}
