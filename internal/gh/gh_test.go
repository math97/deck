package gh

import (
	"errors"
	"testing"
)

func TestBadgePriority(t *testing.T) {
	cases := []struct {
		name  string
		state State
		want  string
	}{
		{"merged ganha de tudo", State{State: "MERGED", ChecksFailing: 3}, "◆ merged"},
		{"fechado", State{State: "CLOSED"}, "✕ fechado"},
		{"CI falhando aparece antes de draft", State{State: "OPEN", IsDraft: true, ChecksTotal: 5, ChecksFailing: 2}, "✗ CI 2/5"},
		{"draft sem falha", State{State: "OPEN", IsDraft: true}, "draft"},
		{"CI rodando", State{State: "OPEN", ChecksTotal: 3, ChecksPending: 1, ChecksPassing: 2}, "● CI 1…"},
		{"mudanças pedidas", State{State: "OPEN", ReviewDecision: "CHANGES_REQUESTED"}, "↩ mudanças"},
		{"aprovado", State{State: "OPEN", ReviewDecision: "APPROVED"}, "✓ aprovado"},
		{"CI verde sem review", State{State: "OPEN", ChecksTotal: 4, ChecksPassing: 4}, "✓ CI ok"},
		{"esperando review", State{State: "OPEN"}, "⏳ review"},
		{"erro vira interrogação", State{Err: errors.New("boom")}, "PR ?"},
	}

	for _, c := range cases {
		if got := c.state.Badge(); got != c.want {
			t.Errorf("%s: Badge() = %q, esperava %q", c.name, got, c.want)
		}
	}
}

func TestHealthy(t *testing.T) {
	cases := []struct {
		name  string
		state State
		want  bool
	}{
		{"aberto e limpo", State{State: "OPEN", ChecksPassing: 2, ChecksTotal: 2}, true},
		{"CI falhando", State{State: "OPEN", ChecksFailing: 1}, false},
		{"fechado", State{State: "CLOSED"}, false},
		{"mudanças pedidas", State{State: "OPEN", ReviewDecision: "CHANGES_REQUESTED"}, false},
		{"erro de consulta", State{Err: errors.New("x")}, false},
	}
	for _, c := range cases {
		if got := c.state.Healthy(); got != c.want {
			t.Errorf("%s: Healthy() = %v, esperava %v", c.name, got, c.want)
		}
	}
}

func TestFetchEmptyURL(t *testing.T) {
	// Não deve tentar executar o gh com url vazia.
	if st := Fetch(nil, "   "); st.Err == nil {
		t.Error("url vazia deveria devolver erro")
	}
}

func TestDetailDescribesState(t *testing.T) {
	s := State{
		Number: 123, State: "OPEN", ReviewDecision: "CHANGES_REQUESTED",
		ChecksTotal: 5, ChecksPassing: 3, ChecksFailing: 1, ChecksPending: 1,
	}
	got := s.Detail()
	for _, want := range []string{"#123", "open", "changes requested", "3 ok", "1 falhando"} {
		if !contains(got, want) {
			t.Errorf("Detail() = %q, deveria conter %q", got, want)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
