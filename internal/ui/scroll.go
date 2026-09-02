package ui

// Este arquivo resolve a pergunta "o que cabe na tela".
//
// As duas funções são puras: recebem tamanhos e o índice em foco, devolvem a
// janela visível. Nada de estado de rolagem no Model — o que está em foco já
// determina o que precisa aparecer, e um offset guardado só teria como se
// dessincronizar do cursor.

// windowCards escolhe quais cards de uma coluna cabem na altura disponível,
// garantindo que o selecionado esteja entre eles.
//
// heights são as alturas já renderizadas de cada card, em linhas. Devolve o
// intervalo [start, end) e quantos ficaram fora de cada lado.
func windowCards(heights []int, sel, avail int) (start, end, above, below int) {
	n := len(heights)
	if n == 0 || avail <= 0 {
		return 0, 0, 0, 0
	}
	if sel < 0 {
		sel = 0
	}
	if sel >= n {
		sel = n - 1
	}

	// Começa pelo selecionado e cresce para cima até estourar: assim o cursor
	// fica visível mesmo quando é o último de uma coluna longa.
	used := heights[sel]
	start, end = sel, sel+1

	for start > 0 {
		h := heights[start-1]
		if used+h > avail {
			break
		}
		used += h
		start--
	}
	for end < n {
		h := heights[end]
		if used+h > avail {
			break
		}
		used += h
		end++
	}
	// Se sobrou espaço só para cima (cursor no fim), aproveita.
	for start > 0 {
		h := heights[start-1]
		if used+h > avail {
			break
		}
		used += h
		start--
	}

	return start, end, start, n - end
}

// windowColumns escolhe quais colunas cabem na largura, garantindo que a
// focada esteja visível. Rola o mínimo necessário, para o board não dar saltos
// quando você anda uma coluna.
func windowColumns(total, focused, perPage int) (start, end int) {
	if total == 0 {
		return 0, 0
	}
	if perPage < 1 {
		perPage = 1
	}
	if perPage >= total {
		return 0, total
	}

	start = 0
	if focused >= perPage {
		start = focused - perPage + 1
	}
	if start > total-perPage {
		start = total - perPage
	}
	if start < 0 {
		start = 0
	}
	return start, start + perPage
}
