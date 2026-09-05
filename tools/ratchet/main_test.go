package main

import (
	"strings"
	"testing"
)

// A lógica testada aqui é a que decide se um PR passa. Se ela errar para o lado
// frouxo, o gate existe mas não segura nada — e ninguém percebe, porque o CI
// fica verde.

func TestRegraDeReconheceCodigoEstavel(t *testing.T) {
	casos := map[string]string{
		"SA1012: do not pass a nil Context":             "SA1012",
		"G304 (CWE-22): Potential file inclusion":       "G304",
		"ST1005: error strings should not be uppercase": "ST1005",

		// Sem código: errcheck e ineffassign escrevem prosa livre. Devolver
		// vazio é o certo — usar o texto como chave faria renomear variável
		// virar achado novo.
		"Error return value of `fmt.Fprint` is not checked": "",
		"ineffectual assignment to m":                       "",
	}
	for texto, quer := range casos {
		if got := regraDe(texto); got != quer {
			t.Errorf("regraDe(%q) = %q, queria %q", texto, got, quer)
		}
	}
}

func TestContarAgrupaColisaoEmVezDeEsconder(t *testing.T) {
	// Onze errcheck no mesmo arquivo compartilham a chave. Sem contagem, um
	// décimo segundo entraria sem ninguém notar — que é o buraco que este
	// desenho fecha.
	a := achado{Ferramenta: "golangci-lint/errcheck", Arquivo: "internal/ui/ui_test.go"}
	total := contar([]achado{a, a, a})

	if got := total[a.chave()]; got != 3 {
		t.Fatalf("contagem = %d, queria 3", got)
	}
}

func TestPiorouSoOlhaParaCima(t *testing.T) {
	base := map[string]int{"gosec|G304|a.go": 2, "gosec|G104|b.go": 1}

	// Achado que sumiu é melhora, e melhora nunca reprova.
	if r := piorou(base, map[string]int{"gosec|G304|a.go": 1}); len(r) != 0 {
		t.Errorf("melhora não deveria reprovar, veio %v", r)
	}
	// Achado que subiu, reprova.
	if r := piorou(base, map[string]int{"gosec|G304|a.go": 3}); len(r) != 1 {
		t.Errorf("regressão deveria reprovar, veio %v", r)
	}
	// Chave nova é regressão mesmo com o total igual: é o caso de trocar um
	// achado por outro, que uma contagem global deixaria passar.
	trocado := map[string]int{"gosec|G304|a.go": 2, "gosec|G401|c.go": 1}
	r := piorou(base, trocado)
	if len(r) != 1 || !strings.Contains(r[0], "G401") {
		t.Errorf("achado novo em chave nova deveria reprovar, veio %v", r)
	}
}

func TestMelhorouExigeQuedaSemRegressao(t *testing.T) {
	base := map[string]int{"x|A|a.go": 2, "x|B|b.go": 2}

	if !melhorou(base, map[string]int{"x|A|a.go": 1, "x|B|b.go": 2}) {
		t.Error("queda sem regressão é melhora")
	}
	if melhorou(base, base) {
		t.Error("estado igual não é melhora")
	}
	// Cair de um lado e subir do outro não é melhora: é regressão disfarçada.
	if melhorou(base, map[string]int{"x|A|a.go": 1, "x|B|b.go": 3}) {
		t.Error("queda com regressão não pode contar como melhora")
	}
}

func TestBaselineSobreviveARoundTrip(t *testing.T) {
	original := map[string]int{"gosec|G204|internal/gh/gh.go": 2, "golangci-lint/errcheck||cmd/deck/main.go": 7}

	volta, err := carregar(formatar(original))
	if err != nil {
		t.Fatalf("carregar: %v", err)
	}
	if len(volta) != len(original) {
		t.Fatalf("round-trip mudou o tamanho: %d → %d", len(original), len(volta))
	}
	for k, v := range original {
		if volta[k] != v {
			t.Errorf("chave %q: %d → %d", k, v, volta[k])
		}
	}
}

func TestCarregarRecusaLinhaMalformada(t *testing.T) {
	// Baseline corrompido tem que falhar alto. Se `carregar` engolisse a linha,
	// o gate passaria a comparar contra um baseline menor do que o real — ou
	// seja, ficaria mais frouxo exatamente quando algo já está errado.
	if _, err := carregar("naoehnumero gosec|G304|a.go\n"); err == nil {
		t.Error("contagem inválida deveria falhar")
	}
	if _, err := carregar("semchave\n"); err == nil {
		t.Error("linha sem chave deveria falhar")
	}
	// Comentário e linha vazia são ignorados de propósito.
	total, err := carregar("# nota\n\n3 gosec|G304|a.go\n")
	if err != nil || total["gosec|G304|a.go"] != 3 {
		t.Errorf("comentário e vazio deveriam ser ignorados: %v, %v", total, err)
	}
}

func TestRelativoNuncaVazaCaminhoDaMaquina(t *testing.T) {
	// Baseline com o caminho de quem rodou não serve para mais ninguém, e
	// falharia no CI.
	if got := relativo("/repo", "/repo/internal/ui/view.go"); got != "internal/ui/view.go" {
		t.Errorf("relativo = %q", got)
	}
	if got := relativo("/repo", "internal/ui/view.go"); !strings.HasPrefix(got, "internal/") {
		t.Errorf("caminho já relativo deveria passar limpo, veio %q", got)
	}
}
