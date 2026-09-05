package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/math97/deck/internal/board"
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

	// O que impede uma rajada de virar dez avisos não é o timer: é o canal.
	// Ele tem capacidade 1 e o envio é não-bloqueante, então um aviso já
	// pendente absorve todos os seguintes — o segundo cai no `default` e é
	// descartado.
	//
	// Essa é a garantia que sustenta a interface: se alguém trocar o canal por
	// um sem buffer, ou por um de capacidade maior, a rajada volta a inundar o
	// board. Por isso a asserção é sobre a capacidade, e não sobre contar
	// avisos num prazo — contagem depende da velocidade da máquina e passaria
	// mesmo com o debounce desligado.
	if cap(events) != 1 {
		t.Fatalf("o canal do observador precisa ter capacidade 1 para absorver rajada, tem %d", cap(events))
	}

	// Um save de editor gera vários eventos seguidos; um aviso tem que chegar.
	for i := 0; i < 10; i++ {
		os.WriteFile(filepath.Join(root, "cards", "y.md"),
			[]byte("---\ncolumn: todo\n---\n"), 0o644)
	}

	if !waitEvent(t, events, 2*time.Second) {
		t.Fatal("a rajada deveria gerar ao menos um aviso")
	}

	// E o board não pode receber um aviso por escrita. O teto é fixo e
	// generoso de propósito: a capacidade 1 já limita o que fica pendente, e
	// numa máquina lenta a rajada pode se espalhar por mais de uma janela
	// legitimamente. Dez seria agrupamento nenhum.
	avisos := 1
	for waitEvent(t, events, 300*time.Millisecond) {
		avisos++
		if avisos > 3 {
			t.Fatalf("a rajada não foi agrupada: %d avisos para 10 escritas", avisos)
		}
	}
}

func TestWatchCmdHandlesNilChannel(t *testing.T) {
	if cmd := watchCmd(nil); cmd != nil {
		t.Error("sem canal, não deveria haver comando de espera")
	}
}
