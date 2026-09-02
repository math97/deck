package board

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setup cria um board temporário já inicializado.
func setup(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	root, _, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

func TestInitCreatesDefaultColumns(t *testing.T) {
	b := setup(t)
	if len(b.Columns) != 6 {
		t.Fatalf("esperava 6 colunas, veio %d", len(b.Columns))
	}
	want := []string{"todo", "refine", "in-progress", "code-review", "qa", "done"}
	for i, key := range want {
		if b.Columns[i].Key != key {
			t.Errorf("coluna %d: esperava %q, veio %q", i, key, b.Columns[i].Key)
		}
	}
	if b.Column("todo").HasPrompt() {
		t.Error("To Do não deveria ter prompt")
	}
	if !b.Column("refine").HasPrompt() {
		t.Error("Refine deveria ter prompt")
	}
	if !b.Column("code-review").PostReview {
		t.Error("Code Review deveria estar marcada para publicar no PR")
	}
	if b.Column("qa").PostReview {
		t.Error("só Code Review deveria publicar no PR")
	}
}

func TestInitIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	root, created, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	// 6 colunas + config.md
	if len(created) != 7 {
		t.Fatalf("primeira execução deveria criar 7, criou %d", len(created))
	}

	// Edita uma coluna e roda init de novo: o conteúdo do usuário sobrevive.
	path := filepath.Join(root, "columns", "refine.md")
	os.WriteFile(path, []byte("---\ntitle: Meu Refine\norder: 20\n---\n\nmeu prompt\n"), 0o644)

	_, created2, err := Init(dir)
	if err != nil {
		t.Fatalf("Init 2: %v", err)
	}
	if len(created2) != 0 {
		t.Errorf("segunda execução não deveria criar nada, criou %v", created2)
	}

	b, _ := Load(root)
	if got := b.Column("refine").Title; got != "Meu Refine" {
		t.Errorf("init sobrescreveu edição do usuário: título é %q", got)
	}
	if got := b.Column("refine").Prompt; got != "meu prompt" {
		t.Errorf("init sobrescreveu o prompt: %q", got)
	}
}

func TestNewCardAndMove(t *testing.T) {
	b := setup(t)

	card, err := b.NewCard("Corrigir refresh de token", "todo")
	if err != nil {
		t.Fatalf("NewCard: %v", err)
	}
	if card.ID != "corrigir-refresh-de-token" {
		t.Errorf("id inesperado: %q", card.ID)
	}

	if err := b.MoveCard(card, "refine"); err != nil {
		t.Fatalf("MoveCard: %v", err)
	}

	// Recarrega do disco: o estado tem que ter persistido de verdade.
	b2, err := Load(b.Root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cards := b2.CardsIn("refine")
	if len(cards) != 1 {
		t.Fatalf("esperava 1 card em refine, veio %d", len(cards))
	}
	got := cards[0]
	if got.Title != "Corrigir refresh de token" {
		t.Errorf("título perdido: %q", got.Title)
	}
	if !strings.Contains(got.Body, "To Do → Refine") {
		t.Errorf("transição não foi registrada no log:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "## Critério de aceite") {
		t.Errorf("corpo do template perdido:\n%s", got.Body)
	}
}

func TestMoveRespectsWIPLimit(t *testing.T) {
	b := setup(t)
	col := b.Column("in-progress")
	col.WIPLimit = 1
	if err := col.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, _ = Load(b.Root)

	a, _ := b.NewCard("primeira", "todo")
	c, _ := b.NewCard("segunda", "todo")

	if err := b.MoveCard(a, "in-progress"); err != nil {
		t.Fatalf("primeiro move deveria passar: %v", err)
	}
	if err := b.MoveCard(c, "in-progress"); err == nil {
		t.Error("segundo move deveria ser recusado pelo limite de WIP")
	}
	if c.Column != "todo" {
		t.Errorf("card recusado não deveria ter mudado de coluna, está em %q", c.Column)
	}
}

func TestUnknownFrontmatterFieldsSurvive(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("teste", "todo")

	// Simula um campo que o deck não conhece, escrito à mão ou por um agente.
	raw, _ := os.ReadFile(card.Path)
	edited := strings.Replace(string(raw), "id:", "jira: PROJ-42\nid:", 1)
	os.WriteFile(card.Path, []byte(edited), 0o644)

	b2, _ := Load(b.Root)
	reloaded := b2.CardsIn("todo")[0]
	if err := b2.MoveCard(reloaded, "refine"); err != nil {
		t.Fatalf("MoveCard: %v", err)
	}

	final, _ := os.ReadFile(reloaded.Path)
	if !strings.Contains(string(final), "jira: PROJ-42") {
		t.Errorf("campo desconhecido foi apagado ao salvar:\n%s", final)
	}
}

func TestOrphanCardsAreNotLost(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("órfão", "refine")

	// Remove a coluna por fora, como se o usuário tivesse deletado o arquivo.
	os.Remove(b.Column("refine").Path)

	b2, err := Load(b.Root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	orphans := b2.CardsIn(OrphanColumn)
	if len(orphans) != 1 {
		t.Fatalf("esperava 1 card órfão, veio %d", len(orphans))
	}
	if orphans[0].Title != "órfão" {
		t.Errorf("card errado: %q", orphans[0].Title)
	}
	if len(b2.Errors) == 0 {
		t.Error("board deveria reportar o problema na barra de status")
	}
	_ = card
}

func TestArchiveColumnRefusesWhenOccupied(t *testing.T) {
	b := setup(t)
	b.NewCard("ocupa", "qa")

	if err := b.ArchiveColumn(b.Column("qa")); err == nil {
		t.Error("deveria recusar remover coluna com cards")
	}
	if _, err := os.Stat(b.Column("qa").Path); err != nil {
		t.Error("arquivo da coluna não deveria ter sido removido")
	}
}

func TestShiftColumnReorders(t *testing.T) {
	b := setup(t)
	if err := b.ShiftColumn(b.Column("qa"), -1); err != nil {
		t.Fatalf("ShiftColumn: %v", err)
	}

	b2, _ := Load(b.Root)
	want := []string{"todo", "refine", "in-progress", "qa", "code-review", "done"}
	for i, key := range want {
		if b2.Columns[i].Key != key {
			t.Errorf("posição %d: esperava %q, veio %q", i, key, b2.Columns[i].Key)
		}
	}
}

func TestParseDocWithoutFrontmatter(t *testing.T) {
	doc, err := ParseDoc([]byte("# só um markdown\n\ntexto\n"))
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if !strings.HasPrefix(doc.Body, "# só um markdown") {
		t.Errorf("corpo inesperado: %q", doc.Body)
	}
}

func TestParseDocUnterminatedFrontmatter(t *testing.T) {
	// Arquivo salvo no meio da edição não pode derrubar o board.
	doc, err := ParseDoc([]byte("---\ntitle: meio\n"))
	if err != nil {
		t.Fatalf("ParseDoc não deveria falhar: %v", err)
	}
	if doc.Body == "" {
		t.Error("conteúdo deveria ter sido preservado como corpo")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Corrigir Refresh de Token": "corrigir-refresh-de-token",
		"Ação  com   espaços":       "acao-com-espacos",
		"  --trim--  ":              "trim",
		"Configuração (v2)":         "configuracao-v2",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, esperava %q", in, got, want)
		}
	}
}

func TestColumnAgentKindChain(t *testing.T) {
	b := setup(t)
	path := filepath.Join(b.Root, "columns", "refine.md")
	os.WriteFile(path, []byte("---\ntitle: Refine\norder: 20\nagent_kind: claude, codex , gemini\n---\n\nprompt\n"), 0o644)

	b2, _ := Load(b.Root)
	col := b2.Column("refine")
	want := []string{"claude", "codex", "gemini"}
	if len(col.AgentKinds) != 3 {
		t.Fatalf("esperava 3 provedores, veio %v", col.AgentKinds)
	}
	for i, k := range want {
		if col.AgentKinds[i] != k {
			t.Errorf("provedor %d: esperava %q, veio %q", i, k, col.AgentKinds[i])
		}
	}
	if col.AgentKind() != "claude" {
		t.Errorf("o primeiro da cadeia deveria ser o padrão: %q", col.AgentKind())
	}

	// A cadeia sobrevive a um round-trip pelo disco.
	col.Save()
	b3, _ := Load(b.Root)
	if len(b3.Column("refine").AgentKinds) != 3 {
		t.Error("a cadeia não persistiu ao salvar")
	}
}

func TestColumnWithoutAgentKindDefaultsToClaude(t *testing.T) {
	b := setup(t)
	if got := b.Column("todo").AgentKind(); got != "claude" {
		t.Errorf("padrão deveria ser claude, veio %q", got)
	}
}

func TestSkillColumnHasPromptWithoutBody(t *testing.T) {
	b := setup(t)
	path := filepath.Join(b.Root, "columns", "refine.md")
	os.WriteFile(path, []byte("---\ntitle: Refine\norder: 20\nskill: minha-skill\n---\n"), 0o644)

	b2, _ := Load(b.Root)
	col := b2.Column("refine")
	if col.Skill != "minha-skill" {
		t.Errorf("skill não lida: %q", col.Skill)
	}
	// Sem corpo, mas com skill: a coluna dispara agente.
	if !col.HasPrompt() {
		t.Error("coluna com skill deveria contar como tendo prompt")
	}
}

func TestRenderPromptUsesSkill(t *testing.T) {
	b := setup(t)
	path := filepath.Join(b.Root, "columns", "refine.md")
	os.WriteFile(path, []byte("---\ntitle: Refine\norder: 20\nskill: fake\n---\n\ninstrução extra do board\n"), 0o644)

	// Injeta um resolvedor de mentira: o pacote board não conhece o disco.
	old := Skills
	Skills = func(dir, name string) (string, error) {
		if name != "fake" {
			return "", fmt.Errorf("skill %q não encontrada", name)
		}
		return "CORPO DA SKILL", nil
	}
	defer func() { Skills = old }()

	b2, _ := Load(b.Root)
	card, _ := b2.NewCard("x", "refine")

	out, err := b2.RenderPrompt(card, b2.Column("refine"), "/tmp")
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}
	if !strings.Contains(out, "CORPO DA SKILL") {
		t.Errorf("a skill deveria virar o prompt:\n%s", out)
	}
	// O texto da coluna entra depois, como complemento.
	if !strings.Contains(out, "instrução extra do board") {
		t.Errorf("o texto da coluna deveria ser preservado:\n%s", out)
	}
	if strings.Index(out, "CORPO DA SKILL") > strings.Index(out, "instrução extra") {
		t.Error("a skill deveria vir antes do complemento da coluna")
	}
}

func TestRenderPromptReportsMissingSkill(t *testing.T) {
	b := setup(t)
	path := filepath.Join(b.Root, "columns", "refine.md")
	os.WriteFile(path, []byte("---\ntitle: Refine\norder: 20\nskill: fantasma\n---\n"), 0o644)

	old := Skills
	Skills = func(dir, name string) (string, error) {
		return "", fmt.Errorf("skill %q não encontrada", name)
	}
	defer func() { Skills = old }()

	b2, _ := Load(b.Root)
	card, _ := b2.NewCard("x", "refine")

	if _, err := b2.RenderPrompt(card, b2.Column("refine"), "/tmp"); err == nil {
		t.Error("skill inexistente deveria falhar claramente")
	}
}
