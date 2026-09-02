package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Os testes deste arquivo são nomeados pela regra que garantem, e não pela
// função que exercitam. Ver docs/regras.md: cada regra de lá tem um teste aqui,
// e uma regra sem teste é uma regra que ninguém decidiu de propósito.

func boardDeTeste(t *testing.T) *Board {
	t.Helper()
	root, _, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

// R1: um card só entra numa coluna que existe.
func TestRegraCardSoEntraEmColunaExistente(t *testing.T) {
	b := boardDeTeste(t)
	card, _ := b.NewCard("tarefa", "todo")
	if err := b.MoveCard(card, "coluna-que-nao-existe"); err == nil {
		t.Error("mover para coluna inexistente deveria falhar")
	}
	if card.Column != "todo" {
		t.Errorf("o card não deveria ter saído do lugar: %s", card.Column)
	}
}

// R2: o limite de WIP vale em qualquer direção, inclusive voltando.
func TestRegraLimiteDeWIPValeEmQualquerDirecao(t *testing.T) {
	b := boardDeTeste(t)
	col := b.Column("refine")
	if col == nil {
		t.Fatal("board padrão sem refine")
	}
	col.WIPLimit = 1
	if err := col.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	primeiro, _ := b.NewCard("primeiro", "todo")
	segundo, _ := b.NewCard("segundo", "todo")
	if err := b.MoveCard(primeiro, "refine"); err != nil {
		t.Fatalf("primeiro deveria caber: %v", err)
	}
	if err := b.MoveCard(segundo, "refine"); err == nil {
		t.Error("segundo deveria esbarrar no limite de WIP")
	}

	// E voltando: o limite é da coluna de destino, não do sentido do movimento.
	if err := b.MoveCard(primeiro, "in-progress"); err != nil {
		t.Fatalf("sair de refine: %v", err)
	}
	if err := b.MoveCard(segundo, "refine"); err != nil {
		t.Errorf("com a vaga aberta, deveria caber: %v", err)
	}
	if err := b.MoveCard(primeiro, "refine"); err == nil {
		t.Error("voltar para uma coluna cheia também esbarra no limite")
	}
}

// R3: o board não tem sentido único — um card volta de qualquer coluna para
// qualquer outra. Não há esteira obrigatória.
func TestRegraCardVoltaParaQualquerColuna(t *testing.T) {
	b := boardDeTeste(t)
	card, _ := b.NewCard("tarefa", "todo")
	for _, key := range []string{"in-progress", "done", "refine", "todo"} {
		if err := b.MoveCard(card, key); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	if card.Column != "todo" {
		t.Errorf("terminou em %s", card.Column)
	}
}

// R4: todo movimento fica registrado no log do card. É o que permite reconstruir
// o caminho depois, já que o board não guarda histórico em outro lugar.
func TestRegraTodoMovimentoEntraNoLog(t *testing.T) {
	b := boardDeTeste(t)
	card, _ := b.NewCard("tarefa", "todo")
	if err := b.MoveCard(card, "refine"); err != nil {
		t.Fatalf("MoveCard: %v", err)
	}
	if !strings.Contains(card.Body, "→ Refine") {
		t.Errorf("o movimento deveria estar no log:\n%s", card.Body)
	}
}

// R5: uma coluna só é arquivada vazia, e nunca a última.
func TestRegraColunaSoArquivaVaziaENuncaAUltima(t *testing.T) {
	b := boardDeTeste(t)
	card, _ := b.NewCard("tarefa", "todo")

	if err := b.ArchiveColumn(b.Column("todo")); err == nil {
		t.Error("coluna com card não deveria ser arquivada")
	}
	if err := b.MoveCard(card, "done"); err != nil {
		t.Fatalf("MoveCard: %v", err)
	}
	if err := b.ArchiveColumn(b.Column("todo")); err != nil {
		t.Errorf("coluna vazia deveria arquivar: %v", err)
	}

	// Esvazia tudo e tenta chegar a zero coluna.
	b2 := boardDeTeste(t)
	cols := append([]*Column(nil), b2.Columns...)
	for i, c := range cols {
		err := b2.ArchiveColumn(c)
		if i < len(cols)-1 && err != nil {
			t.Fatalf("arquivando %s: %v", c.Key, err)
		}
		if i == len(cols)-1 && err == nil {
			t.Error("a última coluna não deveria poder ser arquivada")
		}
	}
}

// R6: arquivar um card o move, nunca o apaga.
func TestRegraArquivarPreservaOCard(t *testing.T) {
	b := boardDeTeste(t)
	card, _ := b.NewCard("tarefa que importa", "todo")
	if err := b.ArchiveCard(card); err != nil {
		t.Fatalf("ArchiveCard: %v", err)
	}
	if _, err := os.Stat(card.Path); err == nil {
		t.Error("o arquivo original deveria ter saído do board")
	}
	entries, err := os.ReadDir(b.ArchiveDir())
	if err != nil || len(entries) == 0 {
		t.Fatalf("o card deveria estar no arquivo: %v", err)
	}
}

// R7: um card apontando para coluna inexistente vai para `?` e continua visível.
func TestRegraCardOrfaoNuncaSome(t *testing.T) {
	b := boardDeTeste(t)
	os.WriteFile(filepath.Join(b.Root, "cards", "orfao.md"),
		[]byte("---\ntitle: órfão\ncolumn: sumida\n---\n\ncorpo\n"), 0o644)

	b2, err := Load(b.Root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(b2.CardsIn(OrphanColumn)) != 1 {
		t.Error("o card órfão deveria aparecer na coluna ?")
	}
	if len(b2.Errors) == 0 {
		t.Error("a barra deveria avisar sobre o órfão")
	}
}

// R8: campo desconhecido no frontmatter sobrevive ao salvar. O board é dono do
// arquivo, não do conteúdo dele.
func TestRegraCampoDesconhecidoSobrevive(t *testing.T) {
	b := boardDeTeste(t)
	path := filepath.Join(b.Root, "cards", "com-extra.md")
	os.WriteFile(path, []byte(
		"---\ntitle: com extra\ncolumn: todo\njira: PROJ-42\nassignee: alguem\n---\n\ncorpo\n"), 0o644)

	b2, _ := Load(b.Root)
	var card *Card
	for _, c := range b2.Cards {
		if c.Title == "com extra" {
			card = c
		}
	}
	if card == nil {
		t.Fatal("card não carregou")
	}
	if err := b2.MoveCard(card, "refine"); err != nil {
		t.Fatalf("MoveCard: %v", err)
	}
	raw, _ := os.ReadFile(path)
	for _, campo := range []string{"jira: PROJ-42", "assignee: alguem"} {
		if !strings.Contains(string(raw), campo) {
			t.Errorf("%q se perdeu ao salvar:\n%s", campo, raw)
		}
	}
}

// R9: uma coluna só dispara agente se tiver prompt — próprio ou de skill.
func TestRegraColunaSemPromptNaoDisparaAgente(t *testing.T) {
	b := boardDeTeste(t)
	if b.Column("todo").HasPrompt() {
		t.Error("To Do não tem prompt e não deveria disparar agente")
	}
	if !b.Column("refine").HasPrompt() {
		t.Error("Refine tem prompt e deveria disparar")
	}
}

// R10: o nome do artefato é a key da coluna que o produziu. Uma regra, zero
// configuração — e é o que faz a esteira se encontrar depois.
func TestRegraArtefatoLevaAKeyDaColuna(t *testing.T) {
	b := boardDeTeste(t)
	card, _ := b.NewCard("tarefa", "refine")
	path, err := card.ArtifactPath("refine")
	if err != nil {
		t.Fatalf("ArtifactPath: %v", err)
	}
	if filepath.Base(path) != "refine.md" {
		t.Errorf("artefato = %s, queria refine.md", filepath.Base(path))
	}
}

// R11: dois cards nunca dividem branch. O branch sai do id do card, que é o
// nome do arquivo dele — único por construção, não por verificação.
func TestRegraCadaCardTemBranchProprio(t *testing.T) {
	b := boardDeTeste(t)
	primeiro, _ := b.NewCard("mesma coisa", "todo")
	segundo, _ := b.NewCard("mesma coisa", "todo")
	if primeiro.ID == segundo.ID {
		t.Fatalf("dois cards com o mesmo id: %s", primeiro.ID)
	}
}
