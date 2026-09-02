package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCardPromotesToFolderOnFirstArtifact(t *testing.T) {
	b := setup(t)
	card, err := b.NewCard("Migrar auth", "refine")
	if err != nil {
		t.Fatalf("NewCard: %v", err)
	}

	if card.IsFolder() {
		t.Fatal("card novo deveria nascer como arquivo único")
	}
	simplePath := card.Path

	if _, err := card.WriteArtifact("refine", "# perguntas\n\n- por quê?\n"); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}

	if !card.IsFolder() {
		t.Fatal("card deveria ter sido promovido a diretório")
	}
	if _, err := os.Stat(simplePath); !os.IsNotExist(err) {
		t.Error("o arquivo antigo deveria ter sido movido, não copiado")
	}
	if filepath.Base(card.Path) != CardFileName {
		t.Errorf("card deveria estar em %s, está em %s", CardFileName, card.Path)
	}

	// E o conteúdo do card sobreviveu à promoção.
	b2, _ := Load(b.Root)
	cards := b2.CardsIn("refine")
	if len(cards) != 1 {
		t.Fatalf("esperava 1 card, veio %d", len(cards))
	}
	got := cards[0]
	if got.Title != "Migrar auth" {
		t.Errorf("título perdido na promoção: %q", got.Title)
	}
	if len(got.Artifacts) != 1 {
		t.Fatalf("esperava 1 artefato, veio %d", len(got.Artifacts))
	}
	if got.Artifacts[0].Column != "refine" {
		t.Errorf("artefato deveria ser da coluna refine, é de %q", got.Artifacts[0].Column)
	}
	if got.Artifacts[0].Title != "Refine" {
		t.Errorf("artefato deveria exibir o título da coluna, exibe %q", got.Artifacts[0].Title)
	}
}

func TestArtifactsAccumulateAcrossColumns(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("Esteira", "refine")

	// Simula a passagem pela esteira: cada coluna deixa seu artefato.
	for _, step := range []struct{ col, body string }{
		{"refine", "plano refinado"},
		{"in-progress", "implementado"},
		{"qa", "plano de testes"},
	} {
		if _, err := card.WriteArtifact(step.col, step.body); err != nil {
			t.Fatalf("WriteArtifact(%s): %v", step.col, err)
		}
	}

	b2, _ := Load(b.Root)
	got := b2.CardsIn("refine")[0]
	if len(got.Artifacts) != 3 {
		t.Fatalf("esperava 3 artefatos, veio %d", len(got.Artifacts))
	}
	for _, key := range []string{"refine", "in-progress", "qa"} {
		a := got.Artifact(key)
		if a == nil {
			t.Errorf("artefato %q não foi carregado", key)
			continue
		}
		content, err := a.Read()
		if err != nil {
			t.Errorf("lendo %q: %v", key, err)
		}
		if strings.TrimSpace(content) == "" {
			t.Errorf("artefato %q está vazio", key)
		}
	}
}

func TestWriteArtifactTwiceOverwrites(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("sobrescreve", "refine")

	card.WriteArtifact("refine", "primeira versão")
	card.WriteArtifact("refine", "segunda versão")

	if len(card.Artifacts) != 1 {
		t.Errorf("não deveria duplicar o artefato: %d", len(card.Artifacts))
	}
	content, _ := card.Artifact("refine").Read()
	if !strings.Contains(content, "segunda") {
		t.Errorf("conteúdo não foi sobrescrito: %q", content)
	}
}

func TestFolderCardMovesAndKeepsArtifacts(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("com artefato", "refine")
	card.WriteArtifact("refine", "refinado")

	if err := b.MoveCard(card, "in-progress"); err != nil {
		t.Fatalf("MoveCard: %v", err)
	}

	b2, _ := Load(b.Root)
	cards := b2.CardsIn("in-progress")
	if len(cards) != 1 {
		t.Fatalf("card não chegou em in-progress")
	}
	if len(cards[0].Artifacts) != 1 {
		t.Error("mover o card não deveria perder os artefatos")
	}
	if !strings.Contains(cards[0].Body, "Refine → In Progress") {
		t.Error("transição não registrada no log do card promovido")
	}
}

func TestGitHubFieldsRoundTrip(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("com PR", "qa")

	card.GitHubPR = "https://github.com/org/repo/pull/123"
	card.GitHubIssue = "https://github.com/org/repo/issues/45"
	if err := card.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	b2, _ := Load(b.Root)
	got := b2.CardsIn("qa")[0]
	if got.GitHubPR != "https://github.com/org/repo/pull/123" {
		t.Errorf("github_pr não persistiu: %q", got.GitHubPR)
	}
	if got.GitHubIssue != "https://github.com/org/repo/issues/45" {
		t.Errorf("github_issue não persistiu: %q", got.GitHubIssue)
	}

	// Esvaziar o campo remove a chave em vez de deixar lixo.
	got.GitHubIssue = ""
	got.Save()
	raw, _ := os.ReadFile(got.Path)
	if strings.Contains(string(raw), "github_issue") {
		t.Errorf("chave vazia deveria ter sido removida:\n%s", raw)
	}
}

func TestRenderPromptSubstitutesVariables(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("Corrigir login", "refine")
	card.WriteArtifact("qa", "notas anteriores")

	// Recarrega para que os artefatos tenham título de coluna.
	b2, _ := Load(b.Root)
	card = b2.CardsIn("refine")[0]

	out, err := b2.RenderPrompt(card, b2.Column("refine"), "/tmp/projeto")
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}

	if strings.Contains(out, "{{") {
		t.Errorf("sobrou variável não substituída:\n%s", out)
	}
	if !strings.Contains(out, card.Path) {
		t.Error("prompt deveria conter o caminho do card")
	}
	// O prompt tem que dizer ao agente onde gravar — é isso que monta a esteira.
	if !strings.Contains(out, "refine.md") {
		t.Errorf("prompt deveria apontar o arquivo de saída:\n%s", out)
	}
	if !strings.Contains(out, "## Log") {
		t.Error("prompt deveria proteger a seção Log")
	}
}

func TestRenderPromptListsPriorArtifacts(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("esteira", "in-progress")
	card.WriteArtifact("refine", "o refinamento")

	b2, _ := Load(b.Root)
	card = b2.CardsIn("in-progress")[0]

	out, err := b2.RenderPrompt(card, b2.Column("in-progress"), "/tmp/projeto")
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}
	// É isso que faz a corrente funcionar: a coluna seguinte enxerga a anterior.
	if !strings.Contains(out, "refine.md") {
		t.Errorf("prompt de in-progress deveria citar o artefato de refine:\n%s", out)
	}
}

func TestRenderPromptRefusesColumnWithoutPrompt(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("parado", "todo")

	if _, err := b.RenderPrompt(card, b.Column("todo"), "/tmp"); err == nil {
		t.Error("coluna sem prompt não deveria render nada")
	}
}

func TestFolderWithoutCardFileReportsError(t *testing.T) {
	b := setup(t)
	os.MkdirAll(filepath.Join(b.CardsDir(), "quebrado"), 0o755)

	b2, err := Load(b.Root)
	if err != nil {
		t.Fatalf("Load não deveria falhar: %v", err)
	}
	if len(b2.Errors) == 0 {
		t.Error("pasta sem card.md deveria virar aviso na barra de status")
	}
}
