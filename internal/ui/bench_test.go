package ui

import (
	"fmt"
	"testing"

	"github.com/matheusalbuquerque/deck/internal/board"
)

// benchModel monta um board com n cards espalhados pelas colunas.
func benchModel(b *testing.B, n int) Model {
	b.Helper()
	dir := b.TempDir()
	root, _, _ := board.Init(dir)
	bd, _ := board.Load(root)
	cols := []string{"todo", "refine", "in-progress", "code-review", "qa", "done"}
	for i := 0; i < n; i++ {
		bd.NewCard(fmt.Sprintf("Tarefa número %d com título razoavelmente longo", i),
			cols[i%len(cols)])
	}
	bd, _ = board.Load(root)
	m := New(bd)
	m.width, m.height = 160, 45
	return m
}

func BenchmarkViewBoard50(b *testing.B)  { benchView(b, 50) }
func BenchmarkViewBoard200(b *testing.B) { benchView(b, 200) }

func benchView(b *testing.B, n int) {
	m := benchModel(b, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkViewDetail(b *testing.B) {
	m := benchModel(b, 20)
	card := m.currentCard()
	body := "## Contexto\n\n"
	for i := 0; i < 60; i++ {
		body += fmt.Sprintf("- item **%d** com `codigo` e texto\n", i)
	}
	card.Body = body
	card.Save()
	m.reload()
	m.mode = modeDetail

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}
