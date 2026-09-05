package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/math97/deck/internal/board"
	"github.com/math97/deck/internal/herdr"
)

// baseline é o estado do artefato no instante em que o agente subiu. É o que
// permite distinguir "o agente gravou a entrega" de "o arquivo já estava lá".
type baseline struct {
	path    string
	exists  bool
	size    int64
	modTime time.Time
}

// captureMsg é o resultado de capturar o fim de um agente.
type captureMsg struct {
	cardPath  string
	delivered bool   // o agente gravou o artefato esperado
	artifact  string // caminho da entrega
	session   string // caminho da transcrição, quando houve fallback
	size      int64
	err       error
}

// snapshotArtifact registra o estado do artefato antes de o agente trabalhar.
func snapshotArtifact(card *board.Card, columnKey string) baseline {
	if card.Dir == "" {
		return baseline{}
	}
	path := card.Dir + "/" + columnKey + ".md"
	b := baseline{path: path}
	if info, err := os.Stat(path); err == nil {
		b.exists = true
		b.size = info.Size()
		b.modTime = info.ModTime()
	}
	return b
}

// captureAgentResult roda quando um agente termina.
//
// A entrega é o artefato que o próprio agente gravou — o deck nunca sobrescreve
// isso com transcrição de terminal. A transcrição só é salva, num arquivo
// separado, quando a entrega não veio: agente travado, interrompido, ou que
// simplesmente ignorou a instrução.
func captureAgentResult(card *board.Card, agentName string, base baseline) tea.Cmd {
	cardPath := card.Path

	return func() tea.Msg {
		msg := captureMsg{cardPath: cardPath, artifact: base.path}

		if base.path != "" {
			if info, err := os.Stat(base.path); err == nil {
				changed := !base.exists ||
					info.Size() != base.size ||
					info.ModTime().After(base.modTime)
				if changed {
					msg.delivered = true
					msg.size = info.Size()
					return msg
				}
			}
		}

		// Sem entrega: salva a transcrição para que a rodada não se perca.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		text, err := herdr.AgentRead(ctx, agentName, 400)
		if err != nil {
			msg.err = err
			return msg
		}
		if strings.TrimSpace(text) == "" {
			// Alternate screen: o que saiu da tela não entra no scrollback do
			// herdr, então não há o que recuperar.
			msg.err = fmt.Errorf("nada legível no pane (agente em tela alternativa?)")
			return msg
		}

		sessionPath := strings.TrimSuffix(base.path, ".md") + ".session.md"
		header := fmt.Sprintf(
			"# Transcrição — %s\n\nO agente `%s` terminou sem gravar a entrega esperada.\nEste arquivo é a saída bruta do pane, salva para nada se perder.\n\n---\n\n",
			time.Now().Format("2006-01-02 15:04"), agentName,
		)
		if err := os.WriteFile(sessionPath, []byte(header+text+"\n"), 0o644); err != nil {
			msg.err = err
			return msg
		}
		msg.session = sessionPath
		return msg
	}
}

// notify avisa na UI do herdr, para o usuário não precisar estar olhando o board.
func notify(title, body, sound string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		herdr.Notify(ctx, title, body, sound)
		return nil
	}
}
