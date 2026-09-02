// Package herdr controla o herdr, o multiplexer de agentes, pelo CLI dele.
//
// O CLI é a autoridade sobre a sintaxe e devolve JSON no formato
// {"id": ..., "result": {"type": ..., ...}}; erros vão para o stderr como
// {"id": ..., "error": {"code", "message"}} com saída 1.
//
// Todos os identificadores vêm das respostas, nunca são construídos aqui: o
// herdr não reutiliza IDs de panes fechados, e um pane movido de workspace
// ganha um ID novo.
package herdr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Status é o ciclo de vida que o herdr reconhece num agente.
type Status string

const (
	StatusIdle    Status = "idle"
	StatusWorking Status = "working"
	StatusBlocked Status = "blocked"
	StatusDone    Status = "done"
	StatusUnknown Status = "unknown"
)

// Agent é o que o herdr sabe sobre um agente vivo.
type Agent struct {
	Name        string `json:"name"`
	PaneID      string `json:"pane_id"`
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	Kind        string `json:"agent"`
	Status      Status `json:"agent_status"`
	Focused     bool   `json:"focused"`
	Ready       bool   `json:"interactive_ready"`
	CWD         string `json:"cwd"`
}

// Pane é o mínimo que precisamos de um pane.
type Pane struct {
	PaneID      string `json:"pane_id"`
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
}

// envelope é a resposta do CLI.
type envelope struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Inside informa se este processo está rodando dentro de um pane do herdr.
//
// Fora dele o deck não controla nada: o skill do herdr é explícito que uma
// sessão não deve ser dirigida de fora, e o pane focado pode pertencer a outro
// cliente.
func Inside() bool {
	if os.Getenv("HERDR_ENV") != "1" {
		return false
	}
	_, err := exec.LookPath("herdr")
	return err == nil
}

// PaneID devolve o pane em que este processo roda.
func PaneID() string { return os.Getenv("HERDR_PANE_ID") }

// run executa um comando do herdr e decodifica o result.
func run(ctx context.Context, out any, args ...string) error {
	cmd := exec.CommandContext(ctx, "herdr", args...)
	stdout, err := cmd.Output()
	if err != nil {
		// Erro do servidor vem como JSON no stderr; erro de sintaxe, texto.
		if ee, ok := err.(*exec.ExitError); ok {
			var env envelope
			if json.Unmarshal(ee.Stderr, &env) == nil && env.Error != nil {
				return fmt.Errorf("herdr %s: %s", args[0], env.Error.Message)
			}
			if msg := strings.TrimSpace(string(ee.Stderr)); msg != "" {
				return fmt.Errorf("herdr %s: %s", args[0], firstLine(msg))
			}
		}
		return fmt.Errorf("herdr %s: %w", args[0], err)
	}

	var env envelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		return fmt.Errorf("herdr %s: resposta ilegível: %w", args[0], err)
	}
	if env.Error != nil {
		return fmt.Errorf("herdr %s: %s", args[0], env.Error.Message)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Result, out)
}

// AgentList devolve os agentes vivos na sessão.
func AgentList(ctx context.Context) ([]Agent, error) {
	var res struct {
		Agents []Agent `json:"agents"`
	}
	if err := run(ctx, &res, "agent", "list"); err != nil {
		return nil, err
	}
	return res.Agents, nil
}

// PaneSplit cria um pane irmão preservando o diretório de trabalho e sem tirar
// o foco de onde o usuário está.
func PaneSplit(ctx context.Context, cwd, direction string) (*Pane, error) {
	if direction == "" {
		direction = "right"
	}
	var res struct {
		Pane Pane `json:"pane"`
	}
	err := run(ctx, &res, "pane", "split", "--current",
		"--direction", direction, "--cwd", cwd, "--no-focus")
	if err != nil {
		return nil, err
	}
	if res.Pane.PaneID == "" {
		return nil, fmt.Errorf("herdr não devolveu o id do pane novo")
	}
	return &res.Pane, nil
}

// SplitDirection escolhe entre dividir à direita ou abaixo, seguindo a regra do
// herdr: pane largo divide à direita, pane estreito ou alto divide para baixo.
// Em caso de dúvida devolve "right", que é o padrão seguro.
func SplitDirection(ctx context.Context, paneID string) string {
	if paneID == "" {
		return "right"
	}
	var res struct {
		Area struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"area"`
	}
	if err := run(ctx, &res, "pane", "layout", "--pane", paneID); err != nil {
		return "right"
	}
	// Células de terminal são ~2x mais altas que largas, daí o fator.
	if res.Area.Width >= res.Area.Height*2 {
		return "right"
	}
	return "down"
}

// AgentStart sobe um agente num pane que já existe e está no prompt.
//
// Devolve o agente já pronto para receber input. Se o agente subir bloqueado
// numa pergunta, o herdr responde agent_not_ready mas mantém o nome utilizável.
func AgentStart(ctx context.Context, name, kind, paneID string) (*Agent, error) {
	if kind == "" {
		kind = "claude"
	}
	var res struct {
		Agent Agent `json:"agent"`
	}
	err := run(ctx, &res, "agent", "start", name, "--kind", kind, "--pane", paneID)
	if err != nil {
		return nil, err
	}
	return &res.Agent, nil
}

// AgentPrompt envia texto ao agente. Sem --wait: o board não pode congelar
// esperando um refinamento que dura vinte minutos; quem acompanha é o poller.
func AgentPrompt(ctx context.Context, target, text string) error {
	return run(ctx, nil, "agent", "prompt", target, text)
}

// AgentFocus leva o usuário até o pane do agente.
func AgentFocus(ctx context.Context, target string) error {
	return run(ctx, nil, "agent", "focus", target)
}

// AgentRead devolve a saída recente do agente. recent-unwrapped junta as
// quebras suaves, que é o que serve para transcrição.
func AgentRead(ctx context.Context, target string, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	var res struct {
		Content string `json:"content"`
		Text    string `json:"text"`
	}
	err := run(ctx, &res, "agent", "read", target,
		"--source", "recent-unwrapped", "--lines", fmt.Sprint(lines))
	if err != nil {
		return "", err
	}
	if res.Content != "" {
		return res.Content, nil
	}
	return res.Text, nil
}

var nameUnsafe = regexp.MustCompile(`[^a-z0-9_-]+`)

// AgentName monta um nome válido para o herdr a partir do id do card.
//
// A regra do herdr é [a-z][a-z0-9_-]{0,31} e único entre os agentes vivos —
// daí o truncamento, já que o id de um card pode ter até 48 caracteres.
func AgentName(cardID string, taken map[string]bool) string {
	base := "card-" + nameUnsafe.ReplaceAllString(strings.ToLower(cardID), "-")
	base = strings.Trim(base, "-")
	if base == "" || !isLetter(rune(base[0])) {
		base = "card-" + base
	}
	base = truncateName(base, 32)

	if !taken[base] {
		return base
	}
	// Colisão: acrescenta sufixo numérico cabendo no limite.
	for n := 2; n < 100; n++ {
		suffix := fmt.Sprintf("-%d", n)
		candidate := truncateName(base, 32-len(suffix)) + suffix
		if !taken[candidate] {
			return candidate
		}
	}
	return base
}

func truncateName(s string, max int) string {
	if len(s) > max {
		s = s[:max]
	}
	return strings.TrimRight(s, "-_")
}

func isLetter(r rune) bool { return r >= 'a' && r <= 'z' }

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// Badge resume o status num rótulo curto para o card.
func (s Status) Badge() string {
	switch s {
	case StatusWorking:
		return "● trabalhando"
	case StatusBlocked:
		return "◐ bloqueado"
	case StatusDone:
		return "✓ pronto"
	case StatusIdle:
		return "○ ocioso"
	default:
		return "· ?"
	}
}

// NeedsAttention diz se o card deve subir para o topo da coluna.
func (s Status) NeedsAttention() bool {
	return s == StatusBlocked || s == StatusDone
}

// Notify mostra uma notificação na UI do herdr. Falha em silêncio: um aviso
// que não apareceu nunca deve derrubar o fluxo que o gerou.
func Notify(ctx context.Context, title, body, sound string) {
	if sound == "" {
		sound = "done"
	}
	args := []string{"notification", "show", title, "--sound", sound}
	if body != "" {
		args = append(args, "--body", body)
	}
	_ = run(ctx, nil, args...)
}

// PaneClose fecha um pane e mata o processo dentro dele.
//
// Só deve ser chamado para panes que o próprio deck criou, e com confirmação
// do usuário: o skill do herdr é explícito que não se fecha o que não se criou.
func PaneClose(ctx context.Context, paneID string) error {
	if paneID == "" {
		return fmt.Errorf("pane vazio")
	}
	return run(ctx, nil, "pane", "close", paneID)
}
