package board

import (
	"fmt"
	"regexp"
	"strings"
)

// taskLine casa uma linha de checklist markdown: "- [ ] texto" ou "- [x] texto",
// aceitando indentação, marcador - * +, e X maiúsculo.
var taskLine = regexp.MustCompile(`^(\s*[-*+]\s+\[)([ xX])(\]\s*)(.*)$`)

// Checkbox é um item de checklist no corpo do card.
type Checkbox struct {
	Index   int // 1-based, na ordem em que aparece
	Line    int // índice da linha no corpo
	Checked bool
	Text    string
}

// Checkboxes lista os itens de checklist do corpo do card, em ordem.
//
// Não olha em que seção estão: se você criou uma checklist fora de "Critério de
// aceite", ela conta também. Impor uma seção seria impor um formato, e o corpo
// do card é seu.
func (c *Card) Checkboxes() []Checkbox {
	var out []Checkbox
	for i, line := range strings.Split(c.Body, "\n") {
		m := taskLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out = append(out, Checkbox{
			Index:   len(out) + 1,
			Line:    i,
			Checked: m[2] != " ",
			Text:    strings.TrimSpace(m[4]),
		})
	}
	return out
}

// ToggleCheckbox inverte o n-ésimo item (1-based) e persiste o card.
//
// Reescreve só o caractere de dentro dos colchetes: o resto da linha —
// indentação, marcador, texto — fica exatamente como estava.
func (c *Card) ToggleCheckbox(n int) (Checkbox, error) {
	boxes := c.Checkboxes()
	if n < 1 || n > len(boxes) {
		return Checkbox{}, fmt.Errorf("não há item %d nesta checklist", n)
	}
	box := boxes[n-1]

	lines := strings.Split(c.Body, "\n")
	m := taskLine.FindStringSubmatch(lines[box.Line])
	if m == nil {
		return Checkbox{}, fmt.Errorf("linha %d deixou de ser um item", box.Line)
	}

	mark := "x"
	if box.Checked {
		mark = " "
	}
	lines[box.Line] = m[1] + mark + m[3] + m[4]
	c.Body = strings.Join(lines, "\n")

	box.Checked = !box.Checked
	return box, c.Save()
}

// NumberCheckboxes devolve o corpo com cada item de checklist numerado, para
// que o usuário saiba qual tecla aperta. O corpo em disco não é tocado.
func NumberCheckboxes(body string) string {
	n := 0
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		m := taskLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n++
		if n > 9 {
			continue // só os nove primeiros têm tecla
		}
		lines[i] = fmt.Sprintf("%s%s%s**%d.** %s", m[1], m[2], m[3], n, m[4])
	}
	return strings.Join(lines, "\n")
}
