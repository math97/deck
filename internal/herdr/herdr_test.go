package herdr

import (
	"regexp"
	"testing"
)

// herdrNameRule é a regra que o herdr aplica a nomes de agente.
var herdrNameRule = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func TestAgentNameAlwaysValid(t *testing.T) {
	cases := []string{
		"corrigir-login",
		"PROJ-42",
		"Migrar auth para OIDC",
		"um-id-absurdamente-longo-que-passa-de-trinta-e-dois-caracteres",
		"acentuação-e-símbolos!!!",
		"123-começa-com-numero",
		"-",
		"",
	}
	for _, in := range cases {
		got := AgentName(in, nil)
		if !herdrNameRule.MatchString(got) {
			t.Errorf("AgentName(%q) = %q, que viola a regra do herdr", in, got)
		}
	}
}

func TestAgentNameAvoidsCollisions(t *testing.T) {
	taken := map[string]bool{"card-login": true, "card-login-2": true}
	got := AgentName("login", taken)

	if taken[got] {
		t.Errorf("AgentName devolveu um nome já em uso: %q", got)
	}
	if !herdrNameRule.MatchString(got) {
		t.Errorf("nome com sufixo violou a regra: %q", got)
	}
}

func TestAgentNameCollisionStaysWithinLimit(t *testing.T) {
	long := "um-id-bem-longo-que-quase-estoura-o-limite-do-herdr"
	base := AgentName(long, nil)

	taken := map[string]bool{base: true}
	got := AgentName(long, taken)

	if len(got) > 32 {
		t.Errorf("nome com sufixo estourou 32 caracteres: %q (%d)", got, len(got))
	}
	if got == base {
		t.Error("deveria ter evitado a colisão")
	}
	if !herdrNameRule.MatchString(got) {
		t.Errorf("nome inválido: %q", got)
	}
}

func TestStatusBadge(t *testing.T) {
	cases := map[Status]string{
		StatusWorking: "● trabalhando",
		StatusBlocked: "◆ te espera",
		StatusDone:    "✓ pronto",
		StatusIdle:    "○ ocioso",
		StatusUnknown: "· ?",
	}
	for st, want := range cases {
		if got := st.Badge(); got != want {
			t.Errorf("%s.Badge() = %q, esperava %q", st, got, want)
		}
	}
}

func TestNeedsAttention(t *testing.T) {
	// Bloqueado e pronto pedem ação do usuário; trabalhando e ocioso, não.
	for st, want := range map[Status]bool{
		StatusBlocked: true,
		StatusDone:    true,
		StatusWorking: false,
		StatusIdle:    false,
		StatusUnknown: false,
	} {
		if got := st.NeedsAttention(); got != want {
			t.Errorf("%s.NeedsAttention() = %v, esperava %v", st, got, want)
		}
	}
}

func TestInsideRequiresEnvVar(t *testing.T) {
	t.Setenv("HERDR_ENV", "")
	if Inside() {
		t.Error("sem HERDR_ENV=1 não deveria se considerar dentro do herdr")
	}
	t.Setenv("HERDR_ENV", "0")
	if Inside() {
		t.Error("HERDR_ENV=0 não deveria contar como dentro")
	}
}

func TestUnknownKindsWithoutHerdr(t *testing.T) {
	// Sem lista de kinds — herdr ausente — não há o que validar, e validação
	// indisponível não pode virar erro.
	if got := UnknownKinds([]string{"qualquer-coisa"}); got != nil && len(KnownKinds()) == 0 {
		t.Errorf("sem herdr, nada deveria ser reportado como desconhecido: %v", got)
	}
}

func TestUnknownKindsIgnoresEmpty(t *testing.T) {
	if got := UnknownKinds(nil); len(got) != 0 {
		t.Errorf("lista vazia não tem desconhecidos: %v", got)
	}
}
