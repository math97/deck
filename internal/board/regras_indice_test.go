package board

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// O índice de regras é um documento, e documento apodrece em silêncio: o teste
// muda de nome ou de pacote e a tabela segue apontando para o que não existe
// mais. Foi o que aconteceu — o cabeçalho do manual/regras.md afirmava que todo
// teste morava neste pacote quando cinco já tinham ido para internal/ui.
//
// Este teste transforma esse apodrecimento em falha. Ele não verifica as
// regras: verifica que a tabela continua ligada a testes reais.

// linhaDeRegra casa "| R7 | ... | `TestNome` |" e captura número e teste.
var linhaDeRegra = regexp.MustCompile("^\\|\\s*R(\\d+)\\s*\\|.*\\|\\s*`(Test\\w+)`\\s*\\|")

// linhaSemTeste casa uma linha de regra cuja última coluna não é um teste.
var linhaSemTeste = regexp.MustCompile(`^\|\s*R(\d+)\s*\|`)

// raizDoRepo sobe da pasta do pacote até achar o go.mod.
func raizDoRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		pai := filepath.Dir(dir)
		if pai == dir {
			t.Fatal("go.mod não encontrado acima do pacote")
		}
		dir = pai
	}
}

// testesDoRepo coleta o nome de toda função de teste, em qualquer pacote.
func testesDoRepo(t *testing.T, raiz string) map[string]string {
	t.Helper()
	decl := regexp.MustCompile(`(?m)^func (Test\w+)\(`)
	achados := map[string]string{}

	err := filepath.WalkDir(raiz, func(caminho string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// .git é grande e não tem Go; docs/ pode nem existir num clone.
			if nome := d.Name(); nome == ".git" || nome == "docs" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(caminho, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(caminho)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(raiz, caminho)
		for _, m := range decl.FindAllStringSubmatch(string(raw), -1) {
			achados[m[1]] = rel
		}
		return nil
	})
	if err != nil {
		t.Fatalf("varrendo o repo: %v", err)
	}
	return achados
}

// TestIndiceDeRegrasApontaParaTestesReais é o `verificar:` do manual/regras.md.
//
// Falhou? Não conserte apagando a linha da tabela: ou o teste foi renomeado (e
// a tabela precisa do nome novo), ou a regra ficou sem garantia — e aí ela não
// era regra, era intenção.
func TestIndiceDeRegrasApontaParaTestesReais(t *testing.T) {
	raiz := raizDoRepo(t)
	doc := filepath.Join(raiz, "manual", "regras.md")

	raw, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("manual/regras.md é versionado e deveria existir: %v", err)
	}

	testes := testesDoRepo(t, raiz)
	numeros := map[int]bool{}
	var semTeste []string

	for _, linha := range strings.Split(string(raw), "\n") {
		if m := linhaDeRegra.FindStringSubmatch(linha); m != nil {
			numeros[atoi(t, m[1])] = true
			if _, ok := testes[m[2]]; !ok {
				t.Errorf("R%s cita %s, que não existe em nenhum _test.go", m[1], m[2])
			}
			continue
		}
		// Linha de regra sem teste citado: aceitável só se disser por quê.
		if m := linhaSemTeste.FindStringSubmatch(linha); m != nil && strings.Count(linha, "|") >= 4 {
			numeros[atoi(t, m[1])] = true
			semTeste = append(semTeste, "R"+m[1])
		}
	}

	if len(numeros) == 0 {
		t.Fatal("nenhuma regra encontrada — o formato da tabela mudou?")
	}
	if len(semTeste) > 0 {
		t.Errorf("regras sem teste citado: %s — toda regra precisa de um", strings.Join(semTeste, ", "))
	}

	// Numeração contígua: um buraco quer dizer regra removida sem registro, e
	// o motivo de algo ter deixado de ser proibido é o que se pergunta depois.
	for i := 1; i <= len(numeros); i++ {
		if !numeros[i] {
			t.Errorf("R%d não está na tabela, mas há %d regras — numeração com buraco", i, len(numeros))
		}
	}
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}
