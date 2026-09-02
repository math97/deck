package board

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const logHeading = "## Log"

// AppendLog registra um evento na seção Log do card, criando a seção se ainda
// não existir. É isso que faz um card parado há duas semanas voltar a fazer
// sentido quando você o reabre.
func (c *Card) AppendLog(format string, args ...any) {
	entry := fmt.Sprintf("- %s · %s", time.Now().Format("2006-01-02 15:04"), fmt.Sprintf(format, args...))

	body := strings.TrimRight(c.Body, "\n")
	if idx := strings.Index(body, logHeading); idx >= 0 {
		body = strings.TrimRight(body, "\n") + "\n" + entry + "\n"
	} else {
		body = body + "\n\n" + logHeading + "\n" + entry + "\n"
	}
	c.Body = strings.TrimLeft(body, "\n")
}

// MoveCard move um card para outra coluna, registra no log e persiste.
func (b *Board) MoveCard(card *Card, toKey string) error {
	if card.Column == toKey {
		return nil
	}
	to := b.Column(toKey)
	if to == nil {
		return fmt.Errorf("coluna %q não existe", toKey)
	}
	if to.WIPLimit > 0 && len(b.CardsIn(toKey)) >= to.WIPLimit {
		return fmt.Errorf("%s está no limite de WIP (%d)", to.Title, to.WIPLimit)
	}

	fromTitle := card.Column
	if from := b.Column(card.Column); from != nil {
		fromTitle = from.Title
	}

	card.Column = toKey
	card.Updated = time.Now()
	card.AppendLog("%s → %s", fromTitle, to.Title)
	return card.Save()
}

// ShiftCard reordena o card dentro da própria coluna. delta -1 sobe, +1 desce.
func (b *Board) ShiftCard(card *Card, delta int) error {
	siblings := b.CardsIn(card.Column)
	idx := -1
	for i, c := range siblings {
		if c.Path == card.Path {
			idx = i
			break
		}
	}
	target := idx + delta
	if idx < 0 || target < 0 || target >= len(siblings) {
		return nil
	}
	siblings[idx], siblings[target] = siblings[target], siblings[idx]

	// Renumera a coluna inteira para manter a ordem estável no disco.
	for i, c := range siblings {
		c.Order = i
		if err := c.Save(); err != nil {
			return err
		}
	}
	return nil
}

// NewCard cria um card na coluna indicada e devolve o card já salvo.
func (b *Board) NewCard(title, columnKey string) (*Card, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("título vazio")
	}
	if b.Column(columnKey) == nil {
		return nil, fmt.Errorf("coluna %q não existe", columnKey)
	}
	if err := os.MkdirAll(b.CardsDir(), 0o755); err != nil {
		return nil, err
	}

	slug := Slugify(title)
	if slug == "" {
		slug = "card"
	}
	id, path := b.uniqueCardPath(slug)

	now := time.Now()
	card := &Card{
		ID:      id,
		Path:    path,
		Title:   title,
		Column:  columnKey,
		Created: now,
		Updated: now,
		Body:    "## Contexto\n\n\n## Critério de aceite\n\n- [ ] \n",
		doc:     &Doc{},
	}
	card.AppendLog("card criado em %s", columnKey)
	if err := card.Save(); err != nil {
		return nil, err
	}
	b.Cards = append(b.Cards, card)
	return card, nil
}

// uniqueCardPath evita colisão quando dois cards têm o mesmo título.
func (b *Board) uniqueCardPath(slug string) (id, path string) {
	id = slug
	path = filepath.Join(b.CardsDir(), id+".md")
	for n := 2; ; n++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return id, path
		}
		id = fmt.Sprintf("%s-%d", slug, n)
		path = filepath.Join(b.CardsDir(), id+".md")
	}
}

// NewColumn cria uma coluna no fim do board.
func (b *Board) NewColumn(title string) (*Column, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("título vazio")
	}
	key := Slugify(title)
	if key == "" {
		return nil, fmt.Errorf("título não produz um nome de arquivo válido")
	}
	if b.Column(key) != nil {
		return nil, fmt.Errorf("já existe uma coluna %q", key)
	}

	order := 0
	for _, c := range b.Columns {
		if c.Key != OrphanColumn && c.Order >= order {
			order = c.Order + 10
		}
	}

	col := &Column{
		Key:   key,
		Path:  filepath.Join(b.ColumnsDir(), key+".md"),
		Title: title,
		Order: order,
		doc:   &Doc{},
	}
	if err := col.Save(); err != nil {
		return nil, err
	}
	b.Columns = append(b.Columns, col)
	return col, nil
}

// RenameColumn troca só o título. A key (e portanto o nome do arquivo e o que
// os cards referenciam) permanece, para não orfanar nada.
func (b *Board) RenameColumn(col *Column, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("título vazio")
	}
	col.Title = title
	return col.Save()
}

// ShiftColumn reordena colunas. delta -1 move para a esquerda, +1 direita.
func (b *Board) ShiftColumn(col *Column, delta int) error {
	var real []*Column
	for _, c := range b.Columns {
		if c.Key != OrphanColumn {
			real = append(real, c)
		}
	}

	idx := -1
	for i, c := range real {
		if c.Key == col.Key {
			idx = i
			break
		}
	}
	target := idx + delta
	if idx < 0 || target < 0 || target >= len(real) {
		return nil
	}
	real[idx], real[target] = real[target], real[idx]

	for i, c := range real {
		c.Order = (i + 1) * 10
		if err := c.Save(); err != nil {
			return err
		}
	}
	return nil
}

// DeleteColumn remove uma coluna vazia. Recusa se houver cards, para que
// remover uma coluna nunca destrua trabalho por acidente.
func (b *Board) DeleteColumn(col *Column) error {
	if col.Key == OrphanColumn {
		return fmt.Errorf("a coluna órfã some sozinha quando os cards forem realocados")
	}
	if b.HasCards(col.Key) {
		return fmt.Errorf("%s tem cards — mova-os antes de remover", col.Title)
	}
	if len(b.Columns) <= 1 {
		return fmt.Errorf("o board precisa de pelo menos uma coluna")
	}
	return os.Remove(col.Path)
}
