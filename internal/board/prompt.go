package board

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RenderPrompt monta o prompt que será enviado ao agente quando o card entra
// numa coluna.
//
// O que faz a esteira funcionar não é o prompt em si, e sim o fato de o agente
// receber o caminho dos artefatos que as colunas anteriores produziram: o
// refinamento alimenta a implementação, que alimenta o QA.
func (b *Board) RenderPrompt(card *Card, col *Column, cwd string) (string, error) {
	if !col.HasPrompt() {
		return "", fmt.Errorf("a coluna %s não tem prompt", col.Title)
	}

	outPath, err := card.ArtifactPath(col.Key)
	if err != nil {
		return "", err
	}

	vars := map[string]string{
		"card_path":   card.Path,
		"card_dir":    card.Dir,
		"card_id":     card.ID,
		"card_title":  card.Title,
		"column":      col.Key,
		"to_column":   col.Title,
		"output_path": outPath,
		"cwd":         cwd,
		"artifacts":   b.artifactList(card, col.Key),
		"github_pr":   card.GitHubPR,
	}

	out := col.Prompt
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}

	// O rodapé é anexado sempre, com a mecânica que o prompt do usuário não
	// deveria precisar repetir: onde gravar, o que não tocar, e — o mais
	// importante — o que as colunas anteriores já produziram.
	//
	// Listar os artefatos aqui, e não só na variável {{artifacts}}, é
	// deliberado: a esteira precisa funcionar mesmo com um prompt que o
	// usuário escreveu sem conhecer as variáveis.
	var footer strings.Builder
	footer.WriteString("\n\n---\n")

	if prior := b.artifactList(card, col.Key); prior != "(nenhum ainda)" {
		footer.WriteString("Trabalho já feito neste card, leia antes de começar:\n")
		footer.WriteString(prior)
		footer.WriteString("\n\n")
	}

	fmt.Fprintf(&footer,
		"Grave o resultado deste trabalho em %s (markdown).\nNão altere o frontmatter de %s nem a seção ## Log.",
		outPath, card.Path,
	)

	return out + footer.String(), nil
}

// artifactList descreve, em texto, os artefatos que já existem — exceto o da
// própria coluna, que está prestes a ser sobrescrito.
func (b *Board) artifactList(card *Card, skipKey string) string {
	var lines []string
	for _, a := range card.Artifacts {
		if a.Column == skipKey {
			continue
		}
		if _, err := os.Stat(a.Path); err != nil {
			continue
		}
		title := a.Title
		if title == "" {
			title = a.Column
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", title, a.Path))
	}
	sort.Strings(lines)

	if len(lines) == 0 {
		return "(nenhum ainda)"
	}
	return strings.Join(lines, "\n")
}

// RelPath encurta um caminho para exibição, relativo à raiz do board.
func (b *Board) RelPath(path string) string {
	if rel, err := filepath.Rel(filepath.Dir(b.Root), path); err == nil {
		return rel
	}
	return path
}
