package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write cria um SKILL.md num diretório de skills.
func write(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".claude", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseFrontmatterAndBody(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "refinar", `---
name: refinar-tarefa
description: Faz perguntas até a tarefa ficar clara.
version: 1.0
---

# Refinar

Faça uma pergunta por vez.
`)

	got, err := Find(dir, "refinar-tarefa")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	// O nome do frontmatter ganha do nome do diretório.
	if got.Name != "refinar-tarefa" {
		t.Errorf("nome errado: %q", got.Name)
	}
	if !strings.Contains(got.Description, "perguntas") {
		t.Errorf("descrição não lida: %q", got.Description)
	}
	if strings.Contains(got.Body, "version:") {
		t.Errorf("o frontmatter vazou para o corpo:\n%s", got.Body)
	}
	if !strings.HasPrefix(got.Body, "# Refinar") {
		t.Errorf("corpo inesperado:\n%s", got.Body)
	}
}

func TestNameFallsBackToDirectory(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "sem-nome", "# Só corpo\n\ntexto\n")

	got, err := Find(dir, "sem-nome")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.Name != "sem-nome" {
		t.Errorf("sem frontmatter, o nome deveria vir do diretório: %q", got.Name)
	}
}

func TestSkillWithoutBodyIsIgnored(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "vazia", "---\nname: vazia\n---\n\n   \n")

	if _, err := Find(dir, "vazia"); err == nil {
		t.Error("skill sem corpo não serve como prompt e não deveria ser encontrada")
	}
}

func TestFindReportsUnknownSkill(t *testing.T) {
	dir := t.TempDir()
	_, err := Find(dir, "não-existe")
	if err == nil {
		t.Fatal("deveria falhar")
	}
	// A mensagem tem que dizer como descobrir os nomes válidos.
	if !strings.Contains(err.Error(), "deck skills") {
		t.Errorf("erro deveria apontar o caminho: %q", err)
	}
}

func TestListIsSortedAndDeduplicated(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "zebra", "---\nname: zebra\n---\n\ncorpo\n")
	write(t, dir, "alfa", "---\nname: alfa\n---\n\ncorpo\n")

	// ListIn limita a busca ao projeto: List alcançaria as skills reais do
	// usuário e o teste dependeria da máquina.
	got := ListIn([]Root{{Path: filepath.Join(dir, ".claude", "skills"), Source: "projeto"}})
	if len(got) != 2 {
		t.Fatalf("esperava 2 skills, veio %d", len(got))
	}
	if got[0].Name != "alfa" || got[1].Name != "zebra" {
		t.Errorf("lista fora de ordem: %v", []string{got[0].Name, got[1].Name})
	}
}

func TestEmptyNameIsRejected(t *testing.T) {
	if _, err := Find(t.TempDir(), "  "); err == nil {
		t.Error("nome vazio deveria falhar")
	}
}
