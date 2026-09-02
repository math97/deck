package board

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DirName é o diretório de estado, procurado a partir do cwd para cima.
const DirName = ".deck"

// CardFileName é o markdown do card dentro de um card promovido a diretório.
const CardFileName = "card.md"

// Find sobe a árvore de diretórios procurando .deck, como o git faz com .git.
func Find(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, DirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("nenhum %s encontrado a partir de %s (rode `deck init`)", DirName, start)
		}
		dir = parent
	}
}

// Load lê o board inteiro do disco. Erros de arquivos individuais viram
// entradas em Errors em vez de abortar: um card quebrado não pode esconder os
// outros vinte.
func Load(root string) (*Board, error) {
	b := &Board{Root: root}

	cfg, err := loadConfig(root)
	if err != nil {
		b.addError("config: %v", err)
	}
	b.Config = cfg

	if err := loadColumns(b); err != nil {
		return nil, err
	}
	if err := loadCards(b); err != nil {
		return nil, err
	}
	reconcile(b)
	return b, nil
}

func loadColumns(b *Board) error {
	dir := b.ColumnsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("lendo colunas em %s: %w", dir, err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			b.addError("coluna %s: %v", e.Name(), err)
			continue
		}
		doc, err := ParseDoc(raw)
		if err != nil {
			b.addError("coluna %s: %v", e.Name(), err)
			continue
		}

		key := strings.TrimSuffix(e.Name(), ".md")
		title := doc.GetString("title")
		if title == "" {
			title = key
		}
		b.Columns = append(b.Columns, &Column{
			Key:        key,
			Path:       path,
			Title:      title,
			Order:      atoiOr(doc.GetString("order"), 999),
			AgentKinds: splitList(doc.GetString("agent_kind")),
			Skill:      strings.TrimSpace(doc.GetString("skill")),
			WIPLimit:   atoiOr(doc.GetString("wip_limit"), 0),
			Prompt:     strings.TrimSpace(doc.Body),
			PostReview: parseBool(doc.GetString("post_review")),
			doc:        doc,
		})
	}

	sort.SliceStable(b.Columns, func(i, j int) bool {
		if b.Columns[i].Order != b.Columns[j].Order {
			return b.Columns[i].Order < b.Columns[j].Order
		}
		return b.Columns[i].Key < b.Columns[j].Key
	})

	if len(b.Columns) == 0 {
		b.addError("nenhuma coluna em %s", dir)
	}
	return nil
}

func loadCards(b *Board) error {
	dir := b.CardsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // board sem cards ainda é um board válido
		}
		return fmt.Errorf("lendo cards em %s: %w", dir, err)
	}

	for _, e := range entries {
		var cardPath, cardDir, stem string

		switch {
		case e.IsDir():
			// Card promovido: cards/<id>/card.md, com os artefatos ao lado.
			cardDir = filepath.Join(dir, e.Name())
			cardPath = filepath.Join(cardDir, CardFileName)
			if _, err := os.Stat(cardPath); err != nil {
				b.addError("pasta %s não tem %s", e.Name(), CardFileName)
				continue
			}
			stem = e.Name()

		case strings.HasSuffix(e.Name(), ".md"):
			cardPath = filepath.Join(dir, e.Name())
			stem = strings.TrimSuffix(e.Name(), ".md")

		default:
			continue
		}

		raw, err := os.ReadFile(cardPath)
		if err != nil {
			b.addError("card %s: %v", stem, err)
			continue
		}
		doc, err := ParseDoc(raw)
		if err != nil {
			b.addError("card %s: %v", stem, err)
			continue
		}

		id := doc.GetString("id")
		if id == "" {
			id = stem
		}
		title := doc.GetString("title")
		if title == "" {
			title = stem
		}

		card := &Card{
			ID:          id,
			Path:        cardPath,
			Dir:         cardDir,
			Title:       title,
			Column:      doc.GetString("column"),
			Order:       atoiOr(doc.GetString("order"), 0),
			Created:     parseTimeOr(doc.GetString("created"), time.Time{}),
			Updated:     parseTimeOr(doc.GetString("updated"), time.Time{}),
			GitHubPR:    doc.GetString("github_pr"),
			GitHubIssue: doc.GetString("github_issue"),
			Body:        doc.Body,
			doc:         doc,
		}
		if name := doc.GetString("agent_name"); name != "" {
			card.Agent = &AgentRef{
				Name:      name,
				Pane:      doc.GetString("agent_pane"),
				Kind:      doc.GetString("agent_kind"),
				Workspace: doc.GetString("agent_workspace"),
				Worktree:  doc.GetString("worktree_path"),
			}
		}
		if cardDir != "" {
			loadArtifacts(b, card)
		}
		b.Cards = append(b.Cards, card)
	}
	return nil
}

// loadArtifacts lista os markdowns ao lado do card. O stem de cada arquivo é a
// key da coluna que o produziu.
func loadArtifacts(b *Board, card *Card) {
	entries, err := os.ReadDir(card.Dir)
	if err != nil {
		b.addError("artefatos de %s: %v", card.ID, err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == CardFileName {
			continue
		}
		key := strings.TrimSuffix(e.Name(), ".md")
		card.Artifacts = append(card.Artifacts, &Artifact{
			Column: key,
			Path:   filepath.Join(card.Dir, e.Name()),
		})
	}
}

// reconcile move para a coluna órfã todo card que aponta para uma key que não
// existe mais, e registra o problema.
func reconcile(b *Board) {
	valid := map[string]bool{}
	for _, c := range b.Columns {
		valid[c.Key] = true
	}

	// Dá aos artefatos o título da coluna que os produziu, para exibição.
	titles := map[string]string{}
	for _, c := range b.Columns {
		titles[c.Key] = c.Title
	}

	orphans := 0
	for _, card := range b.Cards {
		for _, a := range card.Artifacts {
			if t, ok := titles[a.Column]; ok {
				a.Title = t
			} else {
				// Artefato de uma coluna que não existe mais continua legível.
				a.Title = a.Column
			}
		}
		if card.Column == "" || !valid[card.Column] {
			orphans++
			card.Column = OrphanColumn
		}
	}
	if orphans > 0 {
		b.Columns = append(b.Columns, &Column{
			Key:   OrphanColumn,
			Title: "? sem coluna",
			Order: 100000,
		})
		b.addError("%d card(s) apontam para uma coluna inexistente", orphans)
	}
}

// Save grava um card no disco, sincronizando o frontmatter com a struct.
func (c *Card) Save() error {
	if c.doc == nil {
		c.doc = &Doc{}
	}
	c.doc.SetString("id", c.ID)
	c.doc.SetString("title", c.Title)
	c.doc.SetString("column", c.Column)
	c.doc.SetInt("order", c.Order)
	if !c.Created.IsZero() {
		c.doc.SetString("created", c.Created.Format(time.RFC3339))
	}
	c.doc.SetString("updated", c.Updated.Format(time.RFC3339))

	setOrDelete(c.doc, "github_pr", c.GitHubPR)
	setOrDelete(c.doc, "github_issue", c.GitHubIssue)

	if c.Agent != nil {
		c.doc.SetString("agent_name", c.Agent.Name)
		c.doc.SetString("agent_pane", c.Agent.Pane)
		c.doc.SetString("agent_kind", c.Agent.Kind)
		setOrDelete(c.doc, "agent_workspace", c.Agent.Workspace)
		setOrDelete(c.doc, "worktree_path", c.Agent.Worktree)
	} else {
		for _, k := range []string{"agent_name", "agent_pane", "agent_kind",
			"agent_workspace", "worktree_path"} {
			c.doc.Delete(k)
		}
	}

	c.doc.Body = c.Body
	out, err := c.doc.Bytes()
	if err != nil {
		return err
	}
	return writeAtomic(c.Path, out)
}

// Save grava uma coluna no disco.
func (c *Column) Save() error {
	if c.doc == nil {
		c.doc = &Doc{}
	}
	c.doc.SetString("title", c.Title)
	c.doc.SetInt("order", c.Order)
	if len(c.AgentKinds) > 0 {
		c.doc.SetString("agent_kind", strings.Join(c.AgentKinds, ", "))
	}
	setOrDelete(c.doc, "skill", c.Skill)
	if c.WIPLimit > 0 {
		c.doc.SetInt("wip_limit", c.WIPLimit)
	} else {
		c.doc.Delete("wip_limit")
	}
	if c.PostReview {
		c.doc.SetString("post_review", "on")
	} else {
		c.doc.Delete("post_review")
	}

	c.doc.Body = c.Prompt
	out, err := c.doc.Bytes()
	if err != nil {
		return err
	}
	return writeAtomic(c.Path, out)
}

// writeAtomic escreve via arquivo temporário + rename, para que uma falha no
// meio da gravação não deixe um card truncado no disco.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".deck-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
