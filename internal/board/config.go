package board

import (
	"os"
	"path/filepath"
	"strings"
)

// ConfigFileName é o arquivo central do board: o que está ligado e o que não
// está. Mesmo formato de tudo o mais aqui — frontmatter YAML, corpo livre para
// anotações suas.
const ConfigFileName = "config.md"

// Toggle é um interruptor de três estados. "auto" é o padrão e significa
// "ligue se as ferramentas estiverem disponíveis" — assim o board funciona sem
// configuração e continua sendo possível desligar algo explicitamente.
type Toggle string

const (
	ToggleAuto Toggle = "auto"
	ToggleOn   Toggle = "on"
	ToggleOff  Toggle = "off"
)

// Enabled resolve o interruptor contra a disponibilidade real da ferramenta.
func (t Toggle) Enabled(available bool) bool {
	switch t {
	case ToggleOn:
		return true
	case ToggleOff:
		return false
	default:
		return available
	}
}

// Config é o que o board tem ligado.
type Config struct {
	GitHub Toggle
	Herdr  Toggle

	// GitHubAutoPost publica o review no PR sem perguntar. Falso por padrão:
	// comentar num PR é uma ação pública e irreversível, e o usuário deve
	// decidir cada uma até dizer o contrário.
	GitHubAutoPost bool

	Path string
	Body string
	doc  *Doc
}

// DefaultConfig é o que vale quando não há config.md.
func DefaultConfig() *Config {
	return &Config{GitHub: ToggleAuto, Herdr: ToggleAuto}
}

// loadConfig lê .deck/config.md. Ausência do arquivo não é erro: o board tem
// que funcionar sem configuração nenhuma.
func loadConfig(root string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.Path = filepath.Join(root, ConfigFileName)

	raw, err := os.ReadFile(cfg.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	doc, err := ParseDoc(raw)
	if err != nil {
		return cfg, err
	}
	cfg.doc = doc
	cfg.Body = doc.Body

	if v := parseToggle(doc.GetString("github")); v != "" {
		cfg.GitHub = v
	}
	if v := parseToggle(doc.GetString("herdr")); v != "" {
		cfg.Herdr = v
	}
	cfg.GitHubAutoPost = parseBool(doc.GetString("github_auto_post"))

	return cfg, nil
}

// parseToggle aceita as formas que uma pessoa escreveria à mão.
func parseToggle(s string) Toggle {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "true", "yes", "sim", "1":
		return ToggleOn
	case "off", "false", "no", "não", "nao", "0":
		return ToggleOff
	case "auto":
		return ToggleAuto
	default:
		return ""
	}
}

func parseBool(s string) bool {
	return parseToggle(s) == ToggleOn
}

// Save grava a configuração preservando o corpo e campos desconhecidos.
func (c *Config) Save() error {
	if c.doc == nil {
		c.doc = &Doc{}
	}
	c.doc.SetString("github", string(c.GitHub))
	c.doc.SetString("herdr", string(c.Herdr))
	if c.GitHubAutoPost {
		c.doc.SetString("github_auto_post", "on")
	} else {
		c.doc.Delete("github_auto_post")
	}

	c.doc.Body = c.Body
	out, err := c.doc.Bytes()
	if err != nil {
		return err
	}
	return writeAtomic(c.Path, out)
}
