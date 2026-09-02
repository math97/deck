package board

import (
	"strings"
	"testing"
)

func TestCheckboxesAreFound(t *testing.T) {
	c := &Card{Body: `## Contexto

texto qualquer, com um - hífen no meio

## Critério de aceite

- [ ] primeiro
- [x] segundo
* [X] terceiro com asterisco
  + [ ] quarto indentado

- não é checkbox
`}
	boxes := c.Checkboxes()
	if len(boxes) != 4 {
		t.Fatalf("esperava 4 itens, veio %d", len(boxes))
	}
	if boxes[0].Text != "primeiro" || boxes[0].Checked {
		t.Errorf("item 1 errado: %+v", boxes[0])
	}
	if !boxes[1].Checked {
		t.Error("item 2 deveria estar marcado")
	}
	if !boxes[2].Checked {
		t.Error("X maiúsculo deveria contar como marcado")
	}
	if boxes[3].Text != "quarto indentado" {
		t.Errorf("item indentado não foi lido: %+v", boxes[3])
	}
	for i, b := range boxes {
		if b.Index != i+1 {
			t.Errorf("índice deveria ser 1-based e em ordem: %+v", b)
		}
	}
}

func TestToggleCheckboxFlipsAndPersists(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("com criterios", "todo")
	card.Body = "## Critério de aceite\n\n- [ ] um\n- [x] dois\n"
	card.Save()

	box, err := card.ToggleCheckbox(1)
	if err != nil {
		t.Fatalf("Toggle: %v", err)
	}
	if !box.Checked {
		t.Error("item 1 deveria ter ficado marcado")
	}

	if _, err := card.ToggleCheckbox(2); err != nil {
		t.Fatalf("Toggle 2: %v", err)
	}

	b2, _ := Load(b.Root)
	got := b2.CardsIn("todo")[0]
	if !strings.Contains(got.Body, "- [x] um") {
		t.Errorf("item 1 não persistiu marcado:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "- [ ] dois") {
		t.Errorf("item 2 não persistiu desmarcado:\n%s", got.Body)
	}
}

func TestToggleCheckboxPreservesLineShape(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("formatos", "todo")
	// Indentação e marcador diferentes têm que sobreviver ao toggle.
	card.Body = "  + [ ] indentado com mais\n* [ ] com asterisco\n"
	card.Save()

	card.ToggleCheckbox(1)
	card.ToggleCheckbox(2)

	b2, _ := Load(b.Root)
	got := b2.CardsIn("todo")[0].Body
	if !strings.Contains(got, "  + [x] indentado com mais") {
		t.Errorf("indentação ou marcador alterados:\n%q", got)
	}
	if !strings.Contains(got, "* [x] com asterisco") {
		t.Errorf("marcador asterisco alterado:\n%q", got)
	}
}

func TestToggleCheckboxOutOfRange(t *testing.T) {
	b := setup(t)
	card, _ := b.NewCard("poucos", "todo")
	card.Body = "- [ ] único\n"
	card.Save()

	for _, n := range []int{0, 2, -1, 99} {
		if _, err := card.ToggleCheckbox(n); err == nil {
			t.Errorf("item %d não existe, deveria falhar", n)
		}
	}
}

func TestNumberCheckboxes(t *testing.T) {
	body := "## Critério\n\n- [ ] um\n- [x] dois\n\ntexto\n"
	got := NumberCheckboxes(body)

	if !strings.Contains(got, "- [ ] **1.** um") {
		t.Errorf("item 1 não foi numerado:\n%s", got)
	}
	if !strings.Contains(got, "- [x] **2.** dois") {
		t.Errorf("item 2 não foi numerado:\n%s", got)
	}
	if !strings.Contains(got, "texto") {
		t.Error("o resto do corpo deveria ficar intacto")
	}
	// Numerar é só para exibição — não altera o que está em disco.
	if strings.Contains(body, "**1.**") {
		t.Error("NumberCheckboxes não pode alterar a entrada")
	}
}

func TestNumberCheckboxesStopsAtNine(t *testing.T) {
	body := ""
	for i := 0; i < 12; i++ {
		body += "- [ ] item\n"
	}
	got := NumberCheckboxes(body)
	if strings.Contains(got, "**10.**") {
		t.Error("não deveria numerar além do 9, que é o limite das teclas")
	}
	if !strings.Contains(got, "**9.**") {
		t.Error("deveria numerar até o 9")
	}
}
