package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matheusalbuquerque/deck/internal/board"
	"github.com/matheusalbuquerque/deck/internal/ui"
)

const usage = `deck — board kanban no terminal, definido em markdown

uso:
  deck            abre o board (procura .deck a partir do diretório atual)
  deck init       cria .deck com as colunas padrão
  deck ls         lista os cards por coluna, sem abrir o TUI
  deck prompt <card>
                  imprime o prompt da coluna atual do card, já renderizado
                  (útil para mandar a um agente: deck prompt x | claude -p)
  deck help       esta mensagem

o board vive em .deck/
  columns/<key>.md   uma coluna: frontmatter é a config, o corpo é o prompt
  cards/<id>.md      um card: frontmatter é o estado, o corpo é o contexto
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "deck:", err)
		os.Exit(1)
	}
}

func run() error {
	cmd := ""
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "":
		return runBoard()
	case "init":
		return runInit()
	case "ls":
		return runList()
	case "prompt":
		return runPrompt(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	default:
		return fmt.Errorf("comando desconhecido %q (veja `deck help`)", cmd)
	}
}

func runInit() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	root, created, err := board.Init(cwd)
	if err != nil {
		return err
	}

	rel, _ := filepath.Rel(cwd, root)
	if len(created) == 0 {
		fmt.Printf("%s já existe — nada a criar\n", rel)
		return nil
	}
	fmt.Printf("board criado em %s\n", rel)
	for _, c := range created {
		fmt.Printf("  %s\n", c)
	}
	fmt.Println("\nrode `deck` para abrir.")
	return nil
}

// load encontra e carrega o board a partir do diretório atual.
func load() (*board.Board, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := board.Find(cwd)
	if err != nil {
		return nil, err
	}
	return board.Load(root)
}

func runBoard() error {
	b, err := load()
	if err != nil {
		return err
	}
	p := tea.NewProgram(ui.New(b), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// runList imprime o board em texto puro — útil para conferir de dentro de um
// script ou de um agente, sem subir o TUI.
func runList() error {
	b, err := load()
	if err != nil {
		return err
	}

	for _, col := range b.Columns {
		cards := b.CardsIn(col.Key)
		fmt.Printf("\n%s (%d)\n", col.Title, len(cards))
		if len(cards) == 0 {
			fmt.Println("  —")
			continue
		}
		for _, c := range cards {
			fmt.Printf("  %s  %s\n", c.ID, c.Title)
		}
	}

	for _, e := range b.Errors {
		fmt.Fprintf(os.Stderr, "\n⚠ %s\n", e)
	}
	return nil
}

// runPrompt renderiza o prompt da coluna em que o card está, com as variáveis
// substituídas. É a ponte entre o board e um agente antes de a integração com
// o herdr existir: `deck prompt <card> | claude -p`.
func runPrompt(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("uso: deck prompt <card-id>")
	}
	id := args[0]

	b, err := load()
	if err != nil {
		return err
	}

	var card *board.Card
	for _, c := range b.Cards {
		if c.ID == id {
			card = c
			break
		}
	}
	if card == nil {
		return fmt.Errorf("card %q não encontrado (veja `deck ls`)", id)
	}

	col := b.Column(card.Column)
	if col == nil {
		return fmt.Errorf("card %q está numa coluna inexistente", id)
	}
	if !col.HasPrompt() {
		return fmt.Errorf("a coluna %s não tem prompt — nada a disparar aqui", col.Title)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	out, err := b.RenderPrompt(card, col, cwd)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}
