package ui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestLivePostReview exercita o R ponta a ponta contra um PR real.
//
// Fica atrás de DECK_LIVE_PR porque publica um comentário público e
// irreversível. Rode assim:
//
//	DECK_LIVE_PR=https://github.com/voce/repo/pull/1 \
//	  go test ./internal/ui/ -run TestLivePostReview -v
func TestLivePostReview(t *testing.T) {
	pr := os.Getenv("DECK_LIVE_PR")
	if pr == "" {
		t.Skip("defina DECK_LIVE_PR para exercitar a publicação real")
	}

	m := newTestModel(t)
	m = press(t, m, "n")
	m = typeText(t, m, "caminho vivo do R")
	m = press(t, m, "enter")
	m = press(t, m, "L", "L", "L")
	if got := m.currentColumn().Key; got != "code-review" {
		t.Fatalf("esperava code-review, está em %q", got)
	}

	card := m.currentCard()
	card.GitHubPR = pr
	if err := card.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	body := "## Review do deck\n\nComentário de teste do caminho de escrita do " +
		"[deck](https://github.com/math97/deck): este texto saiu de um " +
		"artefato markdown do board, publicado com `gh pr comment --body-file`.\n\n" +
		"- [x] artefato lido do disco\n- [x] confirmação passada\n"
	if _, err := card.WriteArtifact("code-review", body); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	m.reload()
	m.ghEnabled = true

	// R pede confirmação, e a confirmação diz para onde vai.
	m = press(t, m, "R")
	if m.mode != modeConfirm {
		t.Fatalf("R deveria pedir confirmação, mode=%v status=%q", m.mode, m.status)
	}
	t.Logf("confirmação: %s | %s", m.confirm.question, m.confirm.detail)

	// s confirma e devolve o comando que publica de verdade.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("confirmar deveria devolver o comando de publicação")
	}

	msg := cmd()
	posted, ok := msg.(reviewPostedMsg)
	if !ok {
		t.Fatalf("esperava reviewPostedMsg, veio %T (%v)", msg, msg)
	}
	if posted.err != nil {
		t.Fatalf("gh pr comment falhou: %v", posted.err)
	}
	t.Logf("comentário publicado em: %s", posted.url)

	// E o resultado tem de voltar ao card.
	next, _ = m.Update(posted)
	m = next.(Model)
	if !m.statusOK {
		t.Fatalf("status deveria ser de sucesso, veio %q", m.status)
	}

	raw, err := os.ReadFile(card.Path)
	if err != nil {
		t.Fatalf("relendo card: %v", err)
	}
	if !strings.Contains(string(raw), "review publicado em") {
		t.Errorf("o card deveria registrar a publicação no log:\n%s", raw)
	}
	t.Logf("log do card:\n%s", string(raw))
}
