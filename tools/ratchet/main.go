// Command ratchet é a catraca do quality gate: mede os achados das ferramentas
// de análise, compara com o baseline versionado e recusa qualquer piora.
//
// A regra é uma só: o estado de hoje é o teto, e o teto só desce.
//
// Por que um programa Go e não `jq` num script: normalizar a saída das
// ferramentas tem regra de verdade — código estável quando existe, contagem por
// arquivo quando não existe — e regra de verdade merece teste. O resto do
// projeto é Go, então a ferramenta também é.
//
// Uso:
//
//	ratchet medir       imprime os achados normalizados
//	ratchet conferir    falha se houver achado novo em relação ao baseline
//	ratchet atualizar   regrava o baseline com o estado atual
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// baselinePath é onde a catraca mora, relativo à raiz do repositório.
const baselinePath = ".ci/baseline.txt"

// achado é uma chave do baseline. Nunca carrega número de linha: senão
// qualquer refatoração mexeria no arquivo e o baseline passaria a mudar por
// motivo que não é qualidade.
type achado struct {
	Ferramenta string
	Regra      string // código estável quando a ferramenta tem um; vazio quando não
	Arquivo    string
}

func (a achado) chave() string {
	return a.Ferramenta + "|" + a.Regra + "|" + a.Arquivo
}

// codigoDeRegra reconhece os códigos estáveis embutidos na mensagem: SA1012 do
// staticcheck, ST1005 do stylecheck, G304 do gosec.
//
// Quem não tem código — errcheck e ineffassign escrevem prosa livre — fica com
// regra vazia de propósito. A alternativa seria usar o texto como chave, mas
// ele cita o identificador ("assignment to m"), então renomear uma variável
// viraria achado novo e reprovaria um PR que não piorou nada. Para esses, a
// contagem por arquivo é que segura.
var codigoDeRegra = regexp.MustCompile(`^([A-Z]{1,2}\d{3,4})\b`)

func regraDe(texto string) string {
	if m := codigoDeRegra.FindStringSubmatch(strings.TrimSpace(texto)); m != nil {
		return m[1]
	}
	return ""
}

// relativo devolve o caminho a partir da raiz do repositório. A saída das
// ferramentas mistura caminho absoluto e relativo, e baseline com o caminho da
// máquina de quem rodou não serve para mais ninguém.
func relativo(raiz, caminho string) string {
	if rel, err := filepath.Rel(raiz, caminho); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(caminho)
}

// medirGolangciLint roda o linter e normaliza a saída.
//
// Saída não vazia com código 1 é o normal: quer dizer que houve achado. Só é
// erro de verdade quando não sai JSON nenhum.
func medirGolangciLint(raiz string) ([]achado, error) {
	cmd := exec.Command("golangci-lint", "run", "--output.json.path=stdout", "./...")
	cmd.Dir = raiz
	out, _ := cmd.Output()

	// O golangci-lint imprime o JSON na primeira linha e um resumo legível
	// depois; decodificar o fluxo inteiro falharia no resumo.
	linha := primeiraLinha(out)
	if len(linha) == 0 {
		return nil, fmt.Errorf("golangci-lint não devolveu JSON — está instalado?")
	}

	var payload struct {
		Issues []struct {
			FromLinter string `json:"FromLinter"`
			Text       string `json:"Text"`
			Pos        struct {
				Filename string `json:"Filename"`
			} `json:"Pos"`
		} `json:"Issues"`
	}
	if err := json.Unmarshal(linha, &payload); err != nil {
		return nil, fmt.Errorf("json do golangci-lint: %w", err)
	}

	achados := make([]achado, 0, len(payload.Issues))
	for _, i := range payload.Issues {
		achados = append(achados, achado{
			Ferramenta: "golangci-lint/" + i.FromLinter,
			Regra:      regraDe(i.Text),
			Arquivo:    relativo(raiz, i.Pos.Filename),
		})
	}
	return achados, nil
}

// medirGosec roda o gosec e normaliza a saída. Aqui o `rule_id` sempre existe,
// então a chave fica precisa.
func medirGosec(raiz string) ([]achado, error) {
	cmd := exec.Command("gosec", "-quiet", "-fmt", "json", "./...")
	cmd.Dir = raiz
	out, _ := cmd.Output()
	if len(out) == 0 {
		return nil, fmt.Errorf("gosec não devolveu JSON — está instalado?")
	}

	var payload struct {
		Issues []struct {
			RuleID string `json:"rule_id"`
			File   string `json:"file"`
		} `json:"Issues"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("json do gosec: %w", err)
	}

	achados := make([]achado, 0, len(payload.Issues))
	for _, i := range payload.Issues {
		achados = append(achados, achado{
			Ferramenta: "gosec",
			Regra:      i.RuleID,
			Arquivo:    relativo(raiz, i.File),
		})
	}
	return achados, nil
}

func primeiraLinha(b []byte) []byte {
	if i := strings.IndexByte(string(b), '\n'); i >= 0 {
		return b[:i]
	}
	return b
}

// contar agrupa os achados por chave.
//
// A contagem é o que impede a colisão de virar buraco: onze `errcheck` no mesmo
// arquivo compartilham a chave, e sem contar, um décimo segundo entraria sem
// ninguém notar.
func contar(achados []achado) map[string]int {
	total := map[string]int{}
	for _, a := range achados {
		total[a.chave()]++
	}
	return total
}

// formatar serializa o baseline: "<contagem> <chave>", ordenado por chave.
func formatar(total map[string]int) string {
	chaves := make([]string, 0, len(total))
	for k := range total {
		chaves = append(chaves, k)
	}
	sort.Strings(chaves)

	var b strings.Builder
	for _, k := range chaves {
		fmt.Fprintf(&b, "%d %s\n", total[k], k)
	}
	return b.String()
}

// carregar lê um baseline serializado.
func carregar(texto string) (map[string]int, error) {
	total := map[string]int{}
	s := bufio.NewScanner(strings.NewReader(texto))
	for linha := 1; s.Scan(); linha++ {
		t := strings.TrimSpace(s.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		n, chave, ok := strings.Cut(t, " ")
		if !ok {
			return nil, fmt.Errorf("linha %d malformada: %q", linha, t)
		}
		q, err := strconv.Atoi(n)
		if err != nil {
			return nil, fmt.Errorf("linha %d: contagem inválida %q", linha, n)
		}
		total[chave] = q
	}
	return total, s.Err()
}

// piorou lista o que subiu em relação ao baseline, ordenado.
//
// Só olha para cima: achado que sumiu é melhora, e melhora nunca reprova.
func piorou(base, atual map[string]int) []string {
	var regressoes []string
	for chave, agora := range atual {
		antes := base[chave]
		if agora > antes {
			regressoes = append(regressoes,
				fmt.Sprintf("%s: %d → %d", chave, antes, agora))
		}
	}
	sort.Strings(regressoes)
	return regressoes
}

// melhorou diz se o estado atual é estritamente melhor que o baseline.
func melhorou(base, atual map[string]int) bool {
	if len(piorou(base, atual)) > 0 {
		return false
	}
	for chave, antes := range base {
		if atual[chave] < antes {
			return true
		}
	}
	return false
}

// raizDoRepo sobe até achar o go.mod.
func raizDoRepo() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		pai := filepath.Dir(dir)
		if pai == dir {
			return "", fmt.Errorf("go.mod não encontrado acima de %s", dir)
		}
		dir = pai
	}
}

func medirTudo(raiz string) (map[string]int, error) {
	var todos []achado
	for _, medir := range []func(string) ([]achado, error){medirGolangciLint, medirGosec} {
		achados, err := medir(raiz)
		if err != nil {
			return nil, err
		}
		todos = append(todos, achados...)
	}
	return contar(todos), nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: ratchet medir|conferir|atualizar")
		os.Exit(2)
	}

	raiz, err := raizDoRepo()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	arquivo := filepath.Join(raiz, baselinePath)

	switch os.Args[1] {
	case "medir":
		atual, err := medirTudo(raiz)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Print(formatar(atual))

	case "atualizar":
		atual, err := medirTudo(raiz)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := os.MkdirAll(filepath.Dir(arquivo), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := os.WriteFile(arquivo, []byte(formatar(atual)), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Printf("baseline gravado: %d chaves, %d achados\n", len(atual), somar(atual))

	case "conferir":
		raw, err := os.ReadFile(arquivo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sem baseline em %s — rode `ratchet atualizar` uma vez\n", baselinePath)
			os.Exit(2)
		}
		base, err := carregar(string(raw))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		atual, err := medirTudo(raiz)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		if regressoes := piorou(base, atual); len(regressoes) > 0 {
			fmt.Fprintf(os.Stderr, "a catraca não gira para trás — %d regressão(ões):\n\n", len(regressoes))
			for _, r := range regressoes {
				fmt.Fprintln(os.Stderr, "  "+r)
			}
			fmt.Fprintln(os.Stderr, "\nConserte o achado novo. Se ele for legítimo, diga por quê no corpo do commit")
			fmt.Fprintln(os.Stderr, "e rode `go run ./tools/ratchet atualizar` — o baseline é revisável no diff.")
			os.Exit(1)
		}

		if melhorou(base, atual) {
			fmt.Printf("melhorou: %d achados agora, %d no baseline\n", somar(atual), somar(base))
		} else {
			fmt.Printf("sem regressão: %d achados\n", somar(atual))
		}

	default:
		fmt.Fprintln(os.Stderr, "uso: ratchet medir|conferir|atualizar")
		os.Exit(2)
	}
}

func somar(total map[string]int) int {
	n := 0
	for _, v := range total {
		n += v
	}
	return n
}
