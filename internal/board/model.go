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
	Key       string // stem do arquivo: .deck/columns/<key>.md
	Path      string
	Title     string
	Order     int
	AgentKind string
	WIPLimit  int
	Prompt    string
	doc       *Doc
}

// HasPrompt informa se mover um card para cá deve oferecer disparo de agente.
func (c *Column) HasPrompt() bool { return strings.TrimSpace(c.Prompt) != "" }

// AgentRef liga um card ao agente que o herdr está hospedando.
type AgentRef struct {
	Name string
	Pane string
	Kind string
}

// Card é uma tarefa. O frontmatter guarda o estado; o corpo guarda o contexto
// que você quer reler daqui a duas semanas.
type Card struct {
	ID      string
	Path    string
	Title   string
	Column  string
	Order   int
	Created time.Time
	Updated time.Time
	Agent   *AgentRef
	Body    string
	doc     *Doc
}

// Board é o estado carregado do disco.
type Board struct {
	Root    string // diretório .deck
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
