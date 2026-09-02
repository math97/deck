package board

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Doc é um arquivo markdown com frontmatter YAML: a unidade de armazenamento do
// deck. Cards e colunas usam o mesmo formato, então usam o mesmo parser.
type Doc struct {
	Meta yaml.Node // preservado como árvore para não perder campos desconhecidos
	Body string
}

const fence = "---"

// ParseDoc lê um markdown com frontmatter. Um arquivo sem frontmatter é válido:
// vira um Doc de meta vazio, para que um card escrito à mão nunca seja perdido.
func ParseDoc(raw []byte) (*Doc, error) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")

	if !strings.HasPrefix(text, fence+"\n") {
		return &Doc{Body: text}, nil
	}

	rest := text[len(fence)+1:]
	end := strings.Index(rest, "\n"+fence)
	if end < 0 {
		// Cerca de abertura sem fechamento: trata como corpo puro em vez de
		// falhar, senão um arquivo meio-editado derruba o board inteiro.
		return &Doc{Body: text}, nil
	}

	metaText := rest[:end]
	body := rest[end+len(fence)+1:]
	body = strings.TrimPrefix(body, "\n")

	doc := &Doc{Body: body}
	if strings.TrimSpace(metaText) != "" {
		if err := yaml.Unmarshal([]byte(metaText), &doc.Meta); err != nil {
			return nil, fmt.Errorf("frontmatter inválido: %w", err)
		}
		// yaml.Unmarshal devolve um DocumentNode; queremos o mapping interno.
		if doc.Meta.Kind == yaml.DocumentNode && len(doc.Meta.Content) > 0 {
			doc.Meta = *doc.Meta.Content[0]
		}
	}
	return doc, nil
}

// Bytes serializa o Doc de volta para markdown com frontmatter.
func (d *Doc) Bytes() ([]byte, error) {
	var buf bytes.Buffer

	if d.Meta.Kind != 0 && len(d.Meta.Content) > 0 {
		buf.WriteString(fence + "\n")
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&d.Meta); err != nil {
			return nil, fmt.Errorf("serializando frontmatter: %w", err)
		}
		enc.Close()
		buf.WriteString(fence + "\n\n")
	}

	body := strings.TrimLeft(d.Body, "\n")
	buf.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}

// ensureMapping prepara o nó de meta para receber chaves.
func (d *Doc) ensureMapping() {
	if d.Meta.Kind != yaml.MappingNode {
		d.Meta = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
}

// GetString devolve o valor escalar de uma chave do frontmatter.
func (d *Doc) GetString(key string) string {
	if d.Meta.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(d.Meta.Content); i += 2 {
		if d.Meta.Content[i].Value == key {
			return d.Meta.Content[i+1].Value
		}
	}
	return ""
}

// SetString grava uma chave escalar preservando a posição das chaves existentes
// e quaisquer campos que o deck não conheça.
func (d *Doc) SetString(key, value string) {
	d.ensureMapping()
	for i := 0; i+1 < len(d.Meta.Content); i += 2 {
		if d.Meta.Content[i].Value == key {
			d.Meta.Content[i+1].Kind = yaml.ScalarNode
			d.Meta.Content[i+1].Tag = "!!str"
			d.Meta.Content[i+1].Value = value
			d.Meta.Content[i+1].Style = 0
			return
		}
	}
	d.Meta.Content = append(d.Meta.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}

// SetInt grava uma chave numérica.
func (d *Doc) SetInt(key string, value int) {
	d.SetString(key, fmt.Sprintf("%d", value))
	// Reajusta a tag para que o YAML saia sem aspas.
	for i := 0; i+1 < len(d.Meta.Content); i += 2 {
		if d.Meta.Content[i].Value == key {
			d.Meta.Content[i+1].Tag = "!!int"
			return
		}
	}
}

// Delete remove uma chave do frontmatter.
func (d *Doc) Delete(key string) {
	if d.Meta.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(d.Meta.Content); i += 2 {
		if d.Meta.Content[i].Value == key {
			d.Meta.Content = append(d.Meta.Content[:i], d.Meta.Content[i+2:]...)
			return
		}
	}
}
