// Package gh lê estado de PRs pelo CLI do GitHub.
//
// Usa o binário `gh` em vez da API direta de propósito: herda a autenticação
// que o usuário já tem no terminal, funciona com GitHub Enterprise sem
// configuração extra, e não guarda credencial nenhuma.
package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// State resume o que importa de um PR para caber num card.
type State struct {
	Number         int
	Title          string
	State          string // OPEN, MERGED, CLOSED
	IsDraft        bool
	ReviewDecision string // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED
	ChecksTotal    int
	ChecksPassing  int
	ChecksFailing  int
	ChecksPending  int
	Err            error // falha ao consultar; o card mostra estado degradado
}

// Installed diz apenas se o binário existe. É instantâneo, e é o que se usa
// para decidir a configuração inicial.
func Installed() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// Authenticated confirma que há sessão válida.
//
// Custa cerca de 300ms porque o gh consulta a API, então NUNCA deve ser
// chamado na abertura do board: rode-o em segundo plano e desligue os badges
// se falhar. Bloquear aqui atrasava a abertura do deck em todo lançamento.
func Authenticated() bool {
	if !Installed() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "gh", "auth", "status").Run() == nil
}

// prPayload espelha o JSON que o gh devolve.
type prPayload struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	State          string `json:"state"`
	IsDraft        bool   `json:"isDraft"`
	ReviewDecision string `json:"reviewDecision"`
	StatusCheck    []struct {
		State      string `json:"state"`      // check runs: SUCCESS/FAILURE/PENDING
		Conclusion string `json:"conclusion"` // status contexts
		Status     string `json:"status"`
	} `json:"statusCheckRollup"`
}

// Fetch consulta um PR pela URL.
func Fetch(ctx context.Context, url string) State {
	url = strings.TrimSpace(url)
	if url == "" {
		return State{Err: fmt.Errorf("url vazia")}
	}

	cmd := exec.CommandContext(ctx, "gh", "pr", "view", url,
		"--json", "number,title,state,isDraft,reviewDecision,statusCheckRollup")
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		if stderr == "" {
			stderr = err.Error()
		}
		return State{Err: fmt.Errorf("gh: %s", firstLine(stderr))}
	}

	var p prPayload
	if err := json.Unmarshal(out, &p); err != nil {
		return State{Err: fmt.Errorf("gh: json inválido: %w", err)}
	}

	s := State{
		Number:         p.Number,
		Title:          p.Title,
		State:          p.State,
		IsDraft:        p.IsDraft,
		ReviewDecision: p.ReviewDecision,
	}
	for _, c := range p.StatusCheck {
		s.ChecksTotal++
		switch strings.ToUpper(firstNonEmpty(c.Conclusion, c.State)) {
		case "SUCCESS", "NEUTRAL", "SKIPPED":
			s.ChecksPassing++
		case "FAILURE", "ERROR", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED":
			s.ChecksFailing++
		default:
			s.ChecksPending++
		}
	}
	return s
}

// Badge resume o estado num rótulo curto, do tamanho que cabe num card.
func (s State) Badge() string {
	switch {
	case s.Err != nil:
		return "PR ?"
	case s.State == "MERGED":
		return "◆ merged"
	case s.State == "CLOSED":
		return "✕ fechado"
	case s.ChecksFailing > 0:
		return fmt.Sprintf("✗ CI %d/%d", s.ChecksFailing, s.ChecksTotal)
	case s.IsDraft:
		return "draft"
	case s.ChecksPending > 0:
		return fmt.Sprintf("● CI %d…", s.ChecksPending)
	case s.ReviewDecision == "CHANGES_REQUESTED":
		return "↩ mudanças"
	case s.ReviewDecision == "APPROVED":
		return "✓ aprovado"
	case s.ChecksTotal > 0 && s.ChecksPassing == s.ChecksTotal:
		return "✓ CI ok"
	default:
		return "⏳ review"
	}
}

// Healthy diz se o badge deve aparecer em cor de alerta.
func (s State) Healthy() bool {
	return s.Err == nil && s.ChecksFailing == 0 && s.State != "CLOSED" &&
		s.ReviewDecision != "CHANGES_REQUESTED"
}

// Detail é a descrição longa, para o painel do card.
func (s State) Detail() string {
	if s.Err != nil {
		return "não foi possível consultar: " + s.Err.Error()
	}
	var parts []string
	parts = append(parts, fmt.Sprintf("PR #%d · %s", s.Number, strings.ToLower(s.State)))
	if s.IsDraft {
		parts = append(parts, "draft")
	}
	if s.ReviewDecision != "" {
		parts = append(parts, "review: "+strings.ToLower(strings.ReplaceAll(s.ReviewDecision, "_", " ")))
	}
	if s.ChecksTotal > 0 {
		parts = append(parts, fmt.Sprintf("checks: %d ok, %d falhando, %d rodando",
			s.ChecksPassing, s.ChecksFailing, s.ChecksPending))
	}
	return strings.Join(parts, " · ")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// PostComment publica um comentário num pull request, lendo o corpo de um
// arquivo. Devolve a URL do comentário criado quando o gh a reporta.
//
// É uma ação pública e que não dá para desfazer direito: quem chama tem de ter
// obtido confirmação antes.
func PostComment(ctx context.Context, prURL, bodyFile string) (string, error) {
	if strings.TrimSpace(prURL) == "" {
		return "", fmt.Errorf("card sem github_pr")
	}
	if _, err := os.Stat(bodyFile); err != nil {
		return "", fmt.Errorf("nada para publicar: %w", err)
	}

	cmd := exec.CommandContext(ctx, "gh", "pr", "comment", prURL, "--body-file", bodyFile)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		if stderr == "" {
			stderr = err.Error()
		}
		return "", fmt.Errorf("gh pr comment: %s", firstLine(stderr))
	}
	// O gh imprime a URL do comentário; se mudar, o vazio não atrapalha.
	return strings.TrimSpace(string(out)), nil
}
