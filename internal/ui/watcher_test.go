package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matheusalbuquerque/deck/internal/board"
)

// waitEvent espera um aviso do observador, com prazo.
func waitEvent(t *testing.T, ch chan struct{}, within time.Duration) bool {
	t.Helper()
	select {
	case <-ch:
		return true
	case <-time.After(within):
		return false
	}
}

func TestWatcherNotifiesOnChange(t *testing.T) {
	dir := t.TempDir()
	root, _, _ := board.Init(dir)

	events := startWatcher(root)
	time.Sleep(50 * time.Millisecond) // deixa o observador assentar

	os.WriteFile(filepath.Join(root, "cards", "x.md"),
		[]byte("---\ncolumn: todo\n---\n"), 0o644)

	if !waitEvent(t, events, 2*time.Second) {
		t.Fatal("criar um card deveria avisar o board")
	}
}

func TestWatcherSurvivesRepeatedEdits(t *testing.T) {
	// Este é o teste que a versão antiga não passava: ela fechava e recriava o
	// fsnotify a cada evento, deixando uma janela cega no meio.
	dir := t.TempDir()
	root, _, _ := board.Init(dir)

	events := startWatcher(root)
	time.Sleep(50 * time.Millisecond)

	cardPath := filepath.Join(root, "cards", "x.md")
	for i := 0; i < 5; i++ {
		os.WriteFile(cardPath, []byte("---\ncolumn: todo\n---\n\nedição\n"), 0o644)

		if !waitEvent(t, events, 2*time.Second) {
			t.Fatalf("edição %d não foi observada — janela cega", i+1)
		}
		// Sem pausa entre uma edição e a próxima: é justamente aqui que a
		// versão antiga perdia eventos.
	}
}

func TestWatcherDebouncesBurst(t *testing.T) {
	dir := t.TempDir()
	root, _, _ := board.Init(dir)

	events := startWatcher(root)
	time.Sleep(50 * time.Millisecond)

	// Um save de editor gera vários eventos seguidos; deve virar um aviso só.
	for i := 0; i < 10; i++ {
		os.WriteFile(filepath.Join(root, "cards", "y.md"),
			[]byte("---\ncolumn: todo\n---\n"), 0o644)
	}

	if !waitEvent(t, events, 2*time.Second) {
		t.Fatal("a rajada deveria gerar ao menos um aviso")
	}
	// O canal tem capacidade 1: no máximo mais um aviso pendente, nunca dez.
	extra := 0
	for waitEvent(t, events, 300*time.Millisecond) {
		extra++
		if extra > 2 {
			t.Fatal("a rajada não foi agrupada — muitos avisos")
		}
	}
}

func TestWatchCmdHandlesNilChannel(t *testing.T) {
	if cmd := watchCmd(nil); cmd != nil {
		t.Error("sem canal, não deveria haver comando de espera")
	}
}
