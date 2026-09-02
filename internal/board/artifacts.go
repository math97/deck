package board

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// setOrDelete grava a chave quando há valor e a remove quando não há, para que
// um campo esvaziado não deixe lixo no frontmatter.
func setOrDelete(d *Doc, key, value string) {
	if strings.TrimSpace(value) == "" {
		d.Delete(key)
		return
	}
	d.SetString(key, value)
}

// EnsureDir promove um card de arquivo único para diretório, movendo
// cards/<id>.md para cards/<id>/card.md. Idempotente.
//
// A promoção é automática: acontece na primeira vez que algo precisa gravar um
// artefato, para que um board de tarefas curtas não vire um mar de pastas.
func (c *Card) EnsureDir() error {
	if c.Dir != "" {
		return nil
	}

	dir := strings.TrimSuffix(c.Path, ".md")
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("já existe %s", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	target := filepath.Join(dir, CardFileName)
	if err := os.Rename(c.Path, target); err != nil {
		os.Remove(dir) // desfaz a pasta vazia se o move falhar
		return err
	}

	c.Dir = dir
	c.Path = target
	return nil
}

// ArtifactPath devolve onde o artefato de uma coluna deve morar, promovendo o
// card a diretório se necessário.
func (c *Card) ArtifactPath(columnKey string) (string, error) {
	if err := c.EnsureDir(); err != nil {
		return "", err
	}
	return filepath.Join(c.Dir, columnKey+".md"), nil
}

// WriteArtifact grava a saída de uma coluna no arquivo dela. É o que um agente
// chama ao terminar, e o que você chama ao criar um artefato à mão.
func (c *Card) WriteArtifact(columnKey, content string) (*Artifact, error) {
	path, err := c.ArtifactPath(columnKey)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if err := writeAtomic(path, []byte(content)); err != nil {
		return nil, err
	}

	if a := c.Artifact(columnKey); a != nil {
		return a, nil
	}
	a := &Artifact{Column: columnKey, Title: columnKey, Path: path}
	c.Artifacts = append(c.Artifacts, a)
	return a, nil
}

// Read devolve o conteúdo de um artefato.
func (a *Artifact) Read() (string, error) {
	raw, err := os.ReadFile(a.Path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ArtifactStub cria o arquivo do artefato vazio, com um cabeçalho, para que
// você possa abrir no editor antes de qualquer agente rodar.
func (c *Card) ArtifactStub(col *Column) (*Artifact, error) {
	if a := c.Artifact(col.Key); a != nil {
		return a, nil
	}
	header := fmt.Sprintf("# %s — %s\n\n", col.Title, c.Title)
	return c.WriteArtifact(col.Key, header)
}
