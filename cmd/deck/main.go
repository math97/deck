package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/math97/deck/internal/board"
	"github.com/math97/deck/internal/skill"
	"github.com/math97/deck/internal/ui"
)

const usage = `deck — board kanban no terminal, definido em markdown

uso:
  deck            abre o board (procura .deck a partir do diretório atual)
  deck init       cria .deck com as colunas padrão
  deck ls         lista os cards por coluna, sem abrir o TUI
  deck new "<título>" [--column <key>]
                  cria um card (padrão: a primeira coluna do board)
  deck skills     lista as skills disponíveis para usar numa coluna
  deck prompt <card>
                  imprime o prompt da coluna atual do card, já renderizado
                  (útil para mandar a um agente: deck prompt x | claude -p)
  deck help       esta mensagem

o board vive em .deck/
  columns/<key>.md   uma coluna: frontmatter é a config, o corpo é o prompt
  cards/<id>.md      um card: frontmatter é o estado, o corpo é o contexto
`

func main() {
	// Liga o resolvedor de skills: o pacote board não conhece o sistema de
	// arquivos de skills, só sabe pedir um corpo por nome.
	board.Skills = func(projectDir, name string) (string, error) {
		sk, err := skill.Find(projectDir, name)
		if err != nil {
			return "", err
		}
		return sk.Body, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "deck:", err)
		os.Exit(1)
	}
	if err := run(os.Args[1:], cwd, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "deck:", err)
		os.Exit(1)
	}
}

// run despacha um subcomando. Recebe o diretório de partida e a saída em vez
// de usar os globais, para poder ser exercitado em teste.
func run(args []string, cwd string, out io.Writer) error {
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "":
		return runBoard(cwd)
	case "init":
		return runInit(cwd, out)
	case "ls":
		return runList(cwd, out)
	case "new":
		return runNew(args[1:], cwd, out)
	case "skills":
		return runSkills(cwd, out)
	case "prompt":
		return runPrompt(args[1:], cwd, out)
	case "help", "-h", "--help":
		fmt.Fprint(out, usage)
		return nil
	default:
		return fmt.Errorf("comando desconhecido %q (veja `deck help`)", cmd)
	}
}

func runInit(cwd string, out io.Writer) error {
	root, created, err := board.Init(cwd)
	if err != nil {
		return err
	}

	rel, _ := filepath.Rel(cwd, root)
	if len(created) == 0 {
		fmt.Fprintf(out, "%s já existe — nada a criar\n", rel)
		return nil
	}
	fmt.Fprintf(out, "board criado em %s\n", rel)
	for _, c := range created {
		fmt.Fprintf(out, "  %s\n", c)
	}
	fmt.Fprintln(out, "\nrode `deck` para abrir.")
	return nil
}

// load encontra e carrega o board a partir do diretório atual.
func load(cwd string) (*board.Board, error) {
	root, err := board.Find(cwd)
	if err != nil {
		return nil, err
	}
	return board.Load(root)
}

func runBoard(cwd string) error {
	b, err := load(cwd)
	if err != nil {
		return err
	}
	p := tea.NewProgram(ui.New(b), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// runList imprime o board em texto puro — útil para conferir de dentro de um
// script ou de um agente, sem subir o TUI.
func runList(cwd string, out io.Writer) error {
	b, err := load(cwd)
	if err != nil {
		return err
	}

	for _, col := range b.Columns {
		cards := b.CardsIn(col.Key)
		fmt.Fprintf(out, "\n%s (%d)\n", col.Title, len(cards))
		if len(cards) == 0 {
			fmt.Fprintln(out, "  —")
			continue
		}
		for _, c := range cards {
			fmt.Fprintf(out, "  %s  %s\n", c.ID, c.Title)
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
func runPrompt(args []string, cwd string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("uso: deck prompt <card-id>")
	}
	id := args[0]

	b, err := load(cwd)
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

	text, err := b.RenderPrompt(card, col, cwd)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, text)
	return nil
}

// runNew cria um card pela linha de comando, para que um script — ou um agente
// rodando num pane — possa alimentar o board sem abrir o TUI.
func runNew(args []string, cwd string, out io.Writer) error {
	var title, column string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--column", "-c":
			if i+1 >= len(args) {
				return fmt.Errorf("--column exige um valor")
			}
			column = args[i+1]
			i++
		default:
			if title == "" {
				title = args[i]
			}
		}
	}
	if title == "" {
		return fmt.Errorf(`uso: deck new "<título>" [--column <key>]`)
	}

	b, err := load(cwd)
	if err != nil {
		return err
	}

	if column == "" {
		// Sem coluna, entra na primeira do board — o começo da esteira.
		if len(b.Columns) == 0 {
			return fmt.Errorf("o board não tem colunas")
		}
		column = b.Columns[0].Key
	}
	if b.Column(column) == nil {
		return fmt.Errorf("coluna %q não existe (veja `deck ls`)", column)
	}

	card, err := b.NewCard(title, column)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s\n", card.ID)
	return nil
}

// runSkills lista as skills que uma coluna pode referenciar.
func runSkills(cwd string, out io.Writer) error {
	list := skill.List(cwd)
	if len(list) == 0 {
		fmt.Fprintln(out, "nenhuma skill encontrada em:")
		for _, r := range skill.Roots(cwd) {
			fmt.Fprintf(out, "  %s (%s)\n", r.Path, r.Source)
		}
		return nil
	}
	for _, s := range list {
		fmt.Fprintf(out, "%-34s %s\n", s.Name, s.Source)
		if s.Description != "" {
			fmt.Fprintf(out, "%-34s %s\n", "", truncate(s.Description, 100))
		}
	}
	fmt.Fprintf(out, "\nuse numa coluna com `skill: <nome>` no frontmatter\n")
	return nil
}

// truncate encurta para caber numa linha de terminal.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
