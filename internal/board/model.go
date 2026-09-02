package board

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Column é uma coluna do board. O corpo do arquivo é o prompt enviado ao agente
// quando um card entra nesta coluna; uma coluna sem corpo não dispara nada.
type Column struct {
	Key      string // stem do arquivo: .deck/columns/<key>.md
	Path     string
	Title    string
	Order    int
	WIPLimit int
	Prompt   string

	// AgentKinds é a cadeia de provedores a tentar, em ordem. Vem de
	// `agent_kind: claude, codex` — o segundo entra se o primeiro falhar,
	// que é o caso de cota esgotada.
	AgentKinds []string

	// Skill reaproveita uma skill do Claude Code como prompt, para não
	// reescrever do zero o que já está escrito.
	Skill string

	// PostReview marca a coluna cujo artefato é um review destinado ao PR.
	PostReview bool

	doc *Doc
}

// HasPrompt informa se mover um card para cá deve oferecer disparo de agente.
// Uma coluna com skill dispara mesmo sem corpo próprio.
func (c *Column) HasPrompt() bool {
	return strings.TrimSpace(c.Prompt) != "" || strings.TrimSpace(c.Skill) != ""
}

// AgentKind devolve o primeiro provedor da cadeia, ou "claude".
func (c *Column) AgentKind() string {
	if len(c.AgentKinds) > 0 {
		return c.AgentKinds[0]
	}
	return "claude"
}

// splitList quebra "a, b , c" em ["a","b","c"], descartando vazios.
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// AgentRef liga um card ao agente que o herdr está hospedando.
type AgentRef struct {
	Name string
	Pane string
	Kind string

	// Preenchidos quando o agente roda numa worktree própria.
	Workspace string
	Worktree  string
}

// Artifact é a saída que um agente produziu numa coluna: o refinamento, o
// plano, o resultado do QA. O nome do arquivo é a key da coluna que o gerou,
// então a esteira se organiza sozinha, sem configuração.
type Artifact struct {
	Column string // key da coluna que produziu
	Title  string // título da coluna, para exibição
	Path   string
}

// Card é uma tarefa. O frontmatter guarda o estado; o corpo guarda o contexto
// que você quer reler daqui a duas semanas.
//
// Um card mora num arquivo só (cards/<id>.md) até precisar acumular artefatos;
// aí ele é promovido a diretório (cards/<id>/card.md + cards/<id>/<coluna>.md).
type Card struct {
	ID      string
	Path    string // sempre o markdown do card em si
	Dir     string // vazio enquanto o card for de arquivo único
	Title   string
	Column  string
	Order   int
	Created time.Time
	Updated time.Time
	Agent   *AgentRef

	GitHubPR    string
	GitHubIssue string

	Artifacts []*Artifact
	Body      string
	doc       *Doc
}

// IsFolder informa se o card já foi promovido a diretório.
func (c *Card) IsFolder() bool { return c.Dir != "" }

// Artifact devolve o artefato de uma coluna, ou nil.
func (c *Card) Artifact(columnKey string) *Artifact {
	for _, a := range c.Artifacts {
		if a.Column == columnKey {
			return a
		}
	}
	return nil
}

// Board é o estado carregado do disco.
type Board struct {
	Root    string // diretório .deck
	Config  *Config
	Columns []*Column
	Cards   []*Card
	Errors  []string // problemas não fatais, exibidos na barra de status
}

// OrphanColumn recebe cards cuja coluna não existe mais, para que renomear uma
// key nunca faça trabalho sumir da tela.
const OrphanColumn = "?"

// ColumnsDir e CardsDir são os subdiretórios de .deck.
func (b *Board) ColumnsDir() string { return filepath.Join(b.Root, "columns") }
func (b *Board) CardsDir() string   { return filepath.Join(b.Root, "cards") }

// Column busca uma coluna pela key.
func (b *Board) Column(key string) *Column {
	for _, c := range b.Columns {
		if c.Key == key {
			return c
		}
	}
	return nil
}

// CardsIn devolve os cards de uma coluna, já ordenados.
func (b *Board) CardsIn(key string) []*Card {
	var out []*Card
	for _, c := range b.Cards {
		if c.Column == key {
			out = append(out, c)
		}
	}
	sortCards(out)
	return out
}

// sortCards ordena por campo order e desempata pelo mais recente primeiro.
func sortCards(cards []*Card) {
	for i := 1; i < len(cards); i++ {
		for j := i; j > 0; j-- {
			a, b := cards[j-1], cards[j]
			less := a.Order > b.Order || (a.Order == b.Order && a.Updated.Before(b.Updated))
			if !less {
				break
			}
			cards[j-1], cards[j] = cards[j], cards[j-1]
		}
	}
}

// HasCards informa se a coluna está ocupada — usado antes de removê-la.
func (b *Board) HasCards(key string) bool {
	for _, c := range b.Cards {
		if c.Column == key {
			return true
		}
	}
	return false
}

var slugUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify transforma um título em algo seguro para nome de arquivo e para key
// de coluna.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = removeAccents(s)
	s = slugUnsafe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 48 {
		s = s[:48]
		s = strings.Trim(s, "-")
	}
	return s
}

// removeAccents mapeia os acentos do português para ASCII, já que os nomes de
// arquivo são digitados no terminal.
func removeAccents(s string) string {
	repl := strings.NewReplacer(
		"á", "a", "à", "a", "ã", "a", "â", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "õ", "o", "ô", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c", "ñ", "n",
	)
	return repl.Replace(s)
}

// atoiOr converte com valor padrão, para campos opcionais do frontmatter.
func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

// parseTimeOr aceita RFC3339 e a forma curta AAAA-MM-DD.
func parseTimeOr(s string, def time.Time) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return def
}

func (b *Board) addError(format string, args ...any) {
	b.Errors = append(b.Errors, fmt.Sprintf(format, args...))
}
