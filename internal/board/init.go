package board

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultColumn é o template de uma coluna criada pelo init.
type defaultColumn struct {
	key        string
	title      string
	order      int
	agentKinds []string
	prompt     string
	postReview bool
}

// As colunas padrão. Colunas sem prompt (To Do, Done) são pontos de parada:
// entrar nelas não dispara agente nenhum.
var defaultColumns = []defaultColumn{
	{
		key: "todo", title: "To Do", order: 10,
	},
	{
		key: "refine", title: "Refine", order: 20, agentKinds: []string{"claude"},
		prompt: `Você vai refinar esta tarefa comigo antes de qualquer código ser escrito.

Leia o card em {{card_path}}.

Faça UMA pergunta por vez até ter clareza sobre:
- o problema real por trás do pedido
- critério de aceite verificável
- o que está fora de escopo
- riscos e dependências

Quando tiver o suficiente, reescreva o corpo do card no lugar, mantendo o
frontmatter intacto e preservando a seção ## Log.`,
	},
	{
		key: "in-progress", title: "In Progress", order: 30, agentKinds: []string{"claude"},
		prompt: `Implemente a tarefa descrita em {{card_path}}.

Antes de escrever código, leia o refinamento que já foi feito neste card —
ele está listado no rodapé deste prompt. Não refaça decisões já tomadas lá.

Trabalhe em {{cwd}}. Siga as convenções do código existente.

Grave um plano curto antes de executar: o que vai mudar, em quais arquivos, e
o que fica de fora. Ao terminar, registre o que de fato mudou.`,
	},
	{
		key: "code-review", title: "Code Review", order: 35, agentKinds: []string{"claude"},
		postReview: true,
		prompt: `Revise o código desta tarefa. O card está em {{card_path}}.

Leia o refinamento e o registro da implementação — estão no rodapé deste
prompt. Revise o diff contra o que a tarefa se propôs a fazer, não contra o
que você faria diferente.

Escreva o review em markdown, pronto para ser publicado no pull request:
- comece por um parágrafo curto dizendo se está bom para merge
- depois, os pontos, do mais grave ao menos grave
- cada ponto cita arquivo e linha, e diz o que fazer
- não aponte estilo que o linter já cobre

Se não houver nada a apontar, diga isso em uma linha. Review longo por
educação desperdiça o tempo de quem lê.`,
	},
	{
		key: "qa", title: "QA", order: 40, agentKinds: []string{"claude"},
		prompt: `Monte e execute o plano de testes da tarefa em {{card_path}}.

Leia o refinamento e o registro da implementação — estão listados no rodapé
deste prompt. O plano de testes sai de lá, não da sua imaginação.

Para cada critério de aceite do card, diga se passa ou falha, com evidência
concreta: arquivo, linha, saída de teste. Se houver PR associado
({{github_pr}}), considere os checks de CI como parte da evidência.

Reporte o que falha e o que ficou sem cobertura. Não conserte nada — quem
conserta é a coluna In Progress.`,
	},
	{
		key: "done", title: "Done", order: 50,
	},
}

// Init cria o diretório .deck com as colunas padrão. Não sobrescreve nada:
// rodar de novo num board existente só completa o que falta.
func Init(dir string) (string, []string, error) {
	root := filepath.Join(dir, DirName)
	var created []string

	for _, sub := range []string{"columns", "cards"} {
		path := filepath.Join(root, sub)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return "", nil, fmt.Errorf("criando %s: %w", path, err)
		}
	}

	// Arquivo central: o que o board tem ligado.
	cfgPath := filepath.Join(root, ConfigFileName)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		cfg := DefaultConfig()
		cfg.Path = cfgPath
		cfg.Body = configBody
		if err := cfg.Save(); err != nil {
			return "", nil, fmt.Errorf("criando %s: %w", ConfigFileName, err)
		}
		created = append(created, ConfigFileName)
	}

	for _, dc := range defaultColumns {
		path := filepath.Join(root, "columns", dc.key+".md")
		if _, err := os.Stat(path); err == nil {
			continue // já existe: preserva o que o usuário editou
		}
		col := &Column{
			Key:        dc.key,
			Path:       path,
			Title:      dc.title,
			Order:      dc.order,
			AgentKinds: dc.agentKinds,
			Prompt:     dc.prompt,
			PostReview: dc.postReview,
			doc:        &Doc{},
		}
		if err := col.Save(); err != nil {
			return "", nil, fmt.Errorf("criando coluna %s: %w", dc.key, err)
		}
		created = append(created, filepath.Join("columns", dc.key+".md"))
	}

	return root, created, nil
}

// configBody é a explicação que acompanha o config.md, para o arquivo se
// explicar sozinho quando você o abrir daqui a três meses.
const configBody = `# Configuração do board

Os interruptores acima aceitam ` + "`on`" + `, ` + "`off`" + ` ou ` + "`auto`" + `.
` + "`auto`" + ` liga a integração quando a ferramenta está disponível — é o padrão,
e faz o board funcionar sem configuração nenhuma.

- **github** — badges de PR nos cards e publicação de review.
  Precisa de ` + "`gh auth login`" + `.
- **herdr** — disparar e acompanhar agentes. Só funciona com o deck aberto
  dentro de um pane do herdr.
- **agent_kind** (na coluna) — aceita uma cadeia: ` + "`agent_kind: claude, codex`" + `.
  Se o primeiro provedor falhar — cota esgotada, por exemplo — o deck tenta o
  seguinte e registra no card qual foi usado.
- **skill** (na coluna) — usa uma skill do Claude Code como prompt em vez de
  escrever um do zero. Veja ` + "`deck skills`" + `.
- **worktree** — cada agente trabalha num checkout e branch próprios
  (` + "`deck/<card-id>`" + `), sem disputar a árvore com você. Em ` + "`auto`" + `, usa worktree
  quando o diretório é um repositório git e cai no modo simples quando não é.
- **github_auto_post** — publica o review no PR sem perguntar. Desligado por
  padrão: comentar num PR é público e não dá para desfazer direito, então cada
  publicação passa por uma confirmação até você dizer o contrário.

Este corpo é livre: anote aqui o que quiser sobre o board.
`
