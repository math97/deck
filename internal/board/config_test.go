package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToggleResolution(t *testing.T) {
	cases := []struct {
		toggle    Toggle
		available bool
		want      bool
	}{
		{ToggleOn, false, true}, // "on" liga mesmo sem a ferramenta
		{ToggleOn, true, true},
		{ToggleOff, true, false}, // "off" desliga mesmo com a ferramenta
		{ToggleOff, false, false},
		{ToggleAuto, true, true}, // "auto" segue a disponibilidade
		{ToggleAuto, false, false},
	}
	for _, c := range cases {
		if got := c.toggle.Enabled(c.available); got != c.want {
			t.Errorf("%s.Enabled(%v) = %v, esperava %v", c.toggle, c.available, got, c.want)
		}
	}
}

func TestParseToggleAcceptsHumanForms(t *testing.T) {
	for _, on := range []string{"on", "true", "yes", "sim", "1", "ON", " on "} {
		if parseToggle(on) != ToggleOn {
			t.Errorf("%q deveria ser ToggleOn", on)
		}
	}
	for _, off := range []string{"off", "false", "no", "não", "nao", "0"} {
		if parseToggle(off) != ToggleOff {
			t.Errorf("%q deveria ser ToggleOff", off)
		}
	}
	if parseToggle("talvez") != "" {
		t.Error("valor desconhecido deveria cair no padrão, não virar um toggle")
	}
}

func TestInitCreatesConfig(t *testing.T) {
	b := setup(t)
	if b.Config == nil {
		t.Fatal("board deveria ter config")
	}
	if b.Config.GitHub != ToggleAuto || b.Config.Herdr != ToggleAuto {
		t.Errorf("padrão deveria ser auto: github=%s herdr=%s", b.Config.GitHub, b.Config.Herdr)
	}
	if b.Config.GitHubAutoPost {
		t.Error("publicação automática no PR deve nascer desligada")
	}

	raw, err := os.ReadFile(filepath.Join(b.Root, ConfigFileName))
	if err != nil {
		t.Fatalf("config.md não foi criado: %v", err)
	}
	// O arquivo tem que se explicar sozinho.
	if !strings.Contains(string(raw), "github_auto_post") {
		t.Error("o corpo do config deveria explicar github_auto_post")
	}
}

func TestConfigTogglesAreRead(t *testing.T) {
	b := setup(t)
	path := filepath.Join(b.Root, ConfigFileName)
	os.WriteFile(path, []byte("---\ngithub: off\nherdr: on\ngithub_auto_post: sim\n---\n\nnotas\n"), 0o644)

	b2, err := Load(b.Root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if b2.Config.GitHub != ToggleOff {
		t.Errorf("github deveria estar off, está %s", b2.Config.GitHub)
	}
	if b2.Config.Herdr != ToggleOn {
		t.Errorf("herdr deveria estar on, está %s", b2.Config.Herdr)
	}
	if !b2.Config.GitHubAutoPost {
		t.Error("github_auto_post: sim deveria ligar")
	}
	if b2.Config.Body != "notas\n" && !strings.Contains(b2.Config.Body, "notas") {
		t.Errorf("corpo do config perdido: %q", b2.Config.Body)
	}
}

func TestBoardWorksWithoutConfigFile(t *testing.T) {
	b := setup(t)
	os.Remove(filepath.Join(b.Root, ConfigFileName))

	b2, err := Load(b.Root)
	if err != nil {
		t.Fatalf("board sem config não deveria falhar: %v", err)
	}
	if b2.Config.GitHub != ToggleAuto {
		t.Error("sem arquivo, o padrão é auto")
	}
	for _, e := range b2.Errors {
		if strings.Contains(e, "config") {
			t.Errorf("ausência de config não é erro, mas reportou: %q", e)
		}
	}
}

func TestConfigSavePreservesUnknownFields(t *testing.T) {
	b := setup(t)
	path := filepath.Join(b.Root, ConfigFileName)
	os.WriteFile(path, []byte("---\ngithub: on\njira_url: https://x.atlassian.net\n---\n\ncorpo\n"), 0o644)

	b2, _ := Load(b.Root)
	b2.Config.GitHub = ToggleOff
	if err := b2.Config.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "jira_url") {
		t.Errorf("campo desconhecido apagado:\n%s", raw)
	}
	if !strings.Contains(string(raw), "corpo") {
		t.Errorf("corpo apagado:\n%s", raw)
	}
}

func TestCodeReviewColumnHasPrompt(t *testing.T) {
	b := setup(t)
	col := b.Column("code-review")
	if col == nil {
		t.Fatal("coluna code-review não existe")
	}
	if !col.HasPrompt() {
		t.Error("Code Review deveria ter prompt")
	}
	if !col.PostReview {
		t.Error("Code Review deveria ter post_review ligado")
	}

	// post_review sobrevive a um round-trip pelo disco.
	col.WIPLimit = 2
	col.Save()
	b2, _ := Load(b.Root)
	if !b2.Column("code-review").PostReview {
		t.Error("post_review não persistiu ao salvar")
	}
}
