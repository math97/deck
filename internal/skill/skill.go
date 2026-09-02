// Package skill localiza skills do Claude Code para reaproveitá-las como
// prompt de coluna.
//
// Uma skill é um SKILL.md com frontmatter (name, description) e corpo markdown.
// O deck inlina o corpo no prompt em vez de mandar `/nome`: assim a coluna
// funciona com qualquer agente — codex, gemini, o que for — e não depende de o
// provedor entender slash commands.
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileName é o arquivo que define uma skill.
const FileName = "SKILL.md"

// Skill é uma skill encontrada em disco.
type Skill struct {
	Name        string // do frontmatter, ou o nome do diretório
	Description string
	Path        string
	Source      string // "projeto", "usuário" ou o nome do plugin
	Body        string
}

// Root é um diretório onde se procuram skills, com o rótulo que explica de
// onde a skill veio.
type Root struct {
	Path   string
	Source string
}

// Roots devolve os diretórios de busca, do mais específico ao mais geral: o
// projeto ganha do usuário, que ganha dos plugins.
func Roots(projectDir string) []Root {
	var roots []Root
	if projectDir != "" {
		roots = append(roots, Root{
			Path:   filepath.Join(projectDir, ".claude", "skills"),
			Source: "projeto",
		})
	}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots,
			Root{filepath.Join(home, ".claude", "skills"), "usuário"},
			Root{filepath.Join(home, ".claude", "plugins", "cache"), "plugin"},
		)
	}
	return roots
}

// List devolve todas as skills encontradas, ordenadas por nome.
func List(projectDir string) []Skill { return ListIn(Roots(projectDir)) }

// ListIn varre diretórios específicos. Existe separado de List para que o
// chamador — inclusive o teste — possa limitar a busca em vez de sempre
// alcançar o home do usuário.
func ListIn(roots []Root) []Skill {
	seen := map[string]bool{}
	var out []Skill

	for _, root := range roots {
		source := root.Source
		filepath.WalkDir(root.Path, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() != FileName {
				return nil
			}
			s, err := parse(path, source)
			if err != nil || seen[s.Name] {
				return nil
			}
			seen[s.Name] = true
			out = append(out, s)
			return nil
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Find localiza uma skill pelo nome.
func Find(projectDir, name string) (*Skill, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("nome de skill vazio")
	}

	for _, s := range List(projectDir) {
		if s.Name == name {
			found := s
			return &found, nil
		}
	}
	return nil, fmt.Errorf("skill %q não encontrada (veja `deck skills`)", name)
}

// parse lê um SKILL.md.
func parse(path, source string) (Skill, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}

	s := Skill{
		Path:   path,
		Source: source,
		Name:   filepath.Base(filepath.Dir(path)),
	}

	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	body := text

	// O frontmatter aqui é raso — só chaves escalares —, então uma varredura
	// linha a linha basta e evita depender do parser do board, que é do lado
	// do modelo e não deveria ser importado por um utilitário de leitura.
	if strings.HasPrefix(text, "---\n") {
		if end := strings.Index(text[4:], "\n---"); end >= 0 {
			meta := text[4 : 4+end]
			body = strings.TrimPrefix(text[4+end+4:], "\n")
			for _, line := range strings.Split(meta, "\n") {
				key, value, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				value = strings.Trim(strings.TrimSpace(value), `"'`)
				switch strings.TrimSpace(key) {
				case "name":
					if value != "" {
						s.Name = value
					}
				case "description":
					s.Description = value
				}
			}
		}
	}

	s.Body = strings.TrimSpace(body)
	if s.Body == "" {
		return Skill{}, fmt.Errorf("skill sem corpo: %s", path)
	}
	return s, nil
}
