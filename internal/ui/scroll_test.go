package ui

import "testing"

func TestWindowCardsKeepsSelectionVisible(t *testing.T) {
	// Dez cards de 4 linhas, espaço para 3 deles.
	heights := make([]int, 10)
	for i := range heights {
		heights[i] = 4
	}

	for _, sel := range []int{0, 3, 5, 9} {
		start, end, above, below := windowCards(heights, sel, 12)
		if sel < start || sel >= end {
			t.Errorf("sel=%d ficou fora da janela [%d,%d)", sel, start, end)
		}
		if above != start {
			t.Errorf("sel=%d: above=%d, esperava %d", sel, above, start)
		}
		if below != len(heights)-end {
			t.Errorf("sel=%d: below=%d, esperava %d", sel, below, len(heights)-end)
		}
	}
}

func TestWindowCardsFillsSpaceAtTheEnd(t *testing.T) {
	// Cursor no último card: a janela deve crescer para cima e aproveitar o
	// espaço, em vez de mostrar um card sozinho numa coluna vazia.
	heights := []int{4, 4, 4, 4, 4}
	start, end, _, below := windowCards(heights, 4, 12)
	if end-start != 3 {
		t.Errorf("esperava 3 cards visíveis, veio %d ([%d,%d))", end-start, start, end)
	}
	if below != 0 {
		t.Errorf("nada deveria sobrar abaixo do último, veio %d", below)
	}
}

func TestWindowCardsHandlesEdgeCases(t *testing.T) {
	if s, e, _, _ := windowCards(nil, 0, 10); s != 0 || e != 0 {
		t.Errorf("lista vazia deveria dar janela vazia, veio [%d,%d)", s, e)
	}
	if s, e, _, _ := windowCards([]int{4}, 0, 0); s != 0 || e != 0 {
		t.Errorf("altura zero deveria dar janela vazia, veio [%d,%d)", s, e)
	}
	// Card mais alto que o espaço: mostra assim mesmo, senão a coluna some.
	if s, e, _, _ := windowCards([]int{20}, 0, 5); s != 0 || e != 1 {
		t.Errorf("card maior que a tela deveria aparecer, veio [%d,%d)", s, e)
	}
	// Índice fora de faixa não pode explodir.
	if s, e, _, _ := windowCards([]int{4, 4}, 99, 10); s < 0 || e > 2 {
		t.Errorf("índice fora de faixa gerou janela inválida [%d,%d)", s, e)
	}
}

func TestWindowColumnsKeepsFocusVisible(t *testing.T) {
	for _, focused := range []int{0, 2, 5} {
		start, end := windowColumns(6, focused, 3)
		if focused < start || focused >= end {
			t.Errorf("focused=%d ficou fora de [%d,%d)", focused, start, end)
		}
		if end-start != 3 {
			t.Errorf("focused=%d: esperava 3 colunas, veio %d", focused, end-start)
		}
	}
}

func TestWindowColumnsAllFit(t *testing.T) {
	start, end := windowColumns(4, 0, 10)
	if start != 0 || end != 4 {
		t.Errorf("cabendo tudo, esperava [0,4), veio [%d,%d)", start, end)
	}
}

func TestWindowColumnsDegradesGracefully(t *testing.T) {
	// Terminal estreitíssimo: mostra ao menos uma coluna, a focada.
	start, end := windowColumns(6, 4, 0)
	if end-start != 1 || start != 4 {
		t.Errorf("esperava só a coluna focada, veio [%d,%d)", start, end)
	}
	if s, e := windowColumns(0, 0, 3); s != 0 || e != 0 {
		t.Errorf("sem colunas deveria dar [0,0), veio [%d,%d)", s, e)
	}
}
