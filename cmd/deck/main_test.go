package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// initBoard cria um board temporário e devolve o diretório de trabalho.
func initBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	var out bytes.Buffer
	if err := run([]string{"init"}, dir, &out); err != nil {
		t.Fatalf("init: %v", err)
	}
	return dir
}

func TestInitCreatesBoardAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	var out bytes.Buffer
	if err := run([]string{"init"}, dir, &out); err != nil {
		t.Fatalf("init: %v", err)
	}
	got := out.String()
	for _, want := range []string{"board criado", "config.md", "columns/code-review.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("saída do init não menciona %q:\n%s", want, got)
		}
	}

	// Segunda vez não recria nada e diz isso.
	out.Reset()
	if err := run([]string{"init"}, dir, &out); err != nil {
		t.Fatalf("init 2: %v", err)
	}
	if !strings.Contains(out.String(), "já existe") {
		t.Errorf("segundo init deveria avisar que já existe:\n%s", out.String())
	}
}

func TestListShowsColumnsAndCards(t *testing.T) {
	dir := initBoard(t)
	os.WriteFile(filepath.Join(dir, ".deck", "cards", "x.md"),
		[]byte("---\nid: x\ntitle: Uma tarefa\ncolumn: refine\n---\n"), 0o644)

	var out bytes.Buffer
	if err := run([]string{"ls"}, dir, &out); err != nil {
		t.Fatalf("ls: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Refine (1)") {
		t.Errorf("ls deveria contar o card em Refine:\n%s", got)
	}
	if !strings.Contains(got, "Uma tarefa") {
		t.Errorf("ls deveria listar o título:\n%s", got)
	}
	if !strings.Contains(got, "To Do (0)") {
		t.Errorf("ls deveria mostrar colunas vazias:\n%s", got)
	}
}

func TestCommandsFailOutsideBoard(t *testing.T) {
	dir := t.TempDir() // sem .deck
	var out bytes.Buffer

	for _, args := range [][]string{{"ls"}, {"prompt", "x"}} {
		err := run(args, dir, &out)
		if err == nil {
			t.Errorf("%v fora de um board deveria falhar", args)
			continue
		}
		if !strings.Contains(err.Error(), "deck init") {
			t.Errorf("%v: o erro deveria sugerir `deck init`, veio %q", args, err)
		}
	}
}

func TestPromptRendersForCurrentColumn(t *testing.T) {
	dir := initBoard(t)
	os.WriteFile(filepath.Join(dir, ".deck", "cards", "tarefa.md"),
		[]byte("---\nid: tarefa\ntitle: Uma tarefa\ncolumn: refine\n---\n\ncontexto\n"), 0o644)

	var out bytes.Buffer
	if err := run([]string{"prompt", "tarefa"}, dir, &out); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "{{") {
		t.Errorf("sobrou variável não substituída:\n%s", got)
	}
	if !strings.Contains(got, "refine.md") {
		t.Errorf("o prompt deveria apontar o arquivo de saída:\n%s", got)
	}
	if !strings.Contains(got, "refinar") {
		t.Errorf("deveria usar o prompt da coluna Refine:\n%s", got)
	}
}

func TestPromptErrors(t *testing.T) {
	dir := initBoard(t)
	os.WriteFile(filepath.Join(dir, ".deck", "cards", "parado.md"),
		[]byte("---\nid: parado\ntitle: Parado\ncolumn: todo\n---\n"), 0o644)

	var out bytes.Buffer

	// Sem argumento.
	if err := run([]string{"prompt"}, dir, &out); err == nil ||
		!strings.Contains(err.Error(), "uso:") {
		t.Errorf("sem card deveria mostrar o uso, veio %v", err)
	}

	// Card inexistente.
	if err := run([]string{"prompt", "fantasma"}, dir, &out); err == nil ||
		!strings.Contains(err.Error(), "não encontrado") {
		t.Errorf("card inexistente deveria falhar claramente, veio %v", err)
	}

	// Coluna sem prompt: não há o que disparar.
	err := run([]string{"prompt", "parado"}, dir, &out)
	if err == nil || !strings.Contains(err.Error(), "não tem prompt") {
		t.Errorf("coluna sem prompt deveria falhar, veio %v", err)
	}
}

func TestHelpAndUnknownCommand(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer

	if err := run([]string{"help"}, dir, &out); err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(out.String(), "deck init") {
		t.Errorf("help deveria listar os comandos:\n%s", out.String())
	}

	if err := run([]string{"inventado"}, dir, &out); err == nil ||
		!strings.Contains(err.Error(), "desconhecido") {
		t.Errorf("comando inventado deveria falhar, veio %v", err)
	}
}

func TestBoardIsFoundFromSubdirectory(t *testing.T) {
	dir := initBoard(t)
	sub := filepath.Join(dir, "src", "internal")
	os.MkdirAll(sub, 0o755)

	var out bytes.Buffer
	// Como o git com .git, o deck sobe a árvore procurando .deck.
	if err := run([]string{"ls"}, sub, &out); err != nil {
		t.Fatalf("ls a partir de subdiretório: %v", err)
	}
	if !strings.Contains(out.String(), "To Do") {
		t.Errorf("deveria ter encontrado o board acima:\n%s", out.String())
	}
}

func TestNewCreatesCardAndPrintsID(t *testing.T) {
	dir := initBoard(t)

	var out bytes.Buffer
	if err := run([]string{"new", "Corrigir refresh de token"}, dir, &out); err != nil {
		t.Fatalf("new: %v", err)
	}
	// Imprime só o id, para um script encadear.
	if got := strings.TrimSpace(out.String()); got != "corrigir-refresh-de-token" {
		t.Errorf("saída deveria ser só o id, veio %q", got)
	}

	out.Reset()
	run([]string{"ls"}, dir, &out)
	if !strings.Contains(out.String(), "To Do (1)") {
		t.Errorf("card deveria ter caído na primeira coluna:\n%s", out.String())
	}
}

func TestNewRespectsColumnFlag(t *testing.T) {
	dir := initBoard(t)

	var out bytes.Buffer
	if err := run([]string{"new", "Revisar", "--column", "code-review"}, dir, &out); err != nil {
		t.Fatalf("new: %v", err)
	}

	out.Reset()
	run([]string{"ls"}, dir, &out)
	if !strings.Contains(out.String(), "Code Review (1)") {
		t.Errorf("card deveria estar em Code Review:\n%s", out.String())
	}
}

func TestNewErrors(t *testing.T) {
	dir := initBoard(t)
	var out bytes.Buffer

	if err := run([]string{"new"}, dir, &out); err == nil ||
		!strings.Contains(err.Error(), "uso:") {
		t.Errorf("sem título deveria mostrar o uso, veio %v", err)
	}
	if err := run([]string{"new", "x", "--column", "inexistente"}, dir, &out); err == nil ||
		!strings.Contains(err.Error(), "não existe") {
		t.Errorf("coluna inválida deveria falhar, veio %v", err)
	}
	if err := run([]string{"new", "x", "--column"}, dir, &out); err == nil {
		t.Error("--column sem valor deveria falhar")
	}
}
