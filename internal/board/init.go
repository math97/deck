package board

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultColumn é o template de uma coluna criada pelo init.
type defaultColumn struct {
	key       string
	title     string
	order     int
	agentKind string
	prompt    string
}

// As colunas padrão. Colunas sem prompt (To Do, Done) são pontos de parada:
// entrar nelas não dispara agente nenhum.
var defaultColumns = []defaultColumn{
	{
		key: "todo", title: "To Do", order: 10,
	},
	{
		key: "refine", title: "Refine", order: 20, agentKind: "claude",
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
		key: "in-progress", title: "In Progress", order: 30, agentKind: "claude",
		prompt: `Implemente a tarefa descrita em {{card_path}}.

Trabalhe em {{cwd}}. Siga as convenções do código existente.
Não altere o frontmatter do card nem a seção ## Log.

Ao terminar, resuma em poucas linhas o que mudou e o que ficou de fora.`,
	},
	{
		key: "qa", title: "QA", order: 40, agentKind: "claude",
		prompt: `Valide a tarefa descrita em {{card_path}} contra o diff atual em {{cwd}}.

Percorra cada critério de aceite do card e diga, para cada um, se passa ou
falha — com a evidência concreta (arquivo, linha, saída de teste).

Reporte apenas o que falha ou o que ficou sem cobertura. Não conserte nada.`,
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

	for _, dc := range defaultColumns {
		path := filepath.Join(root, "columns", dc.key+".md")
		if _, err := os.Stat(path); err == nil {
			continue // já existe: preserva o que o usuário editou
		}
		col := &Column{
			Key:       dc.key,
			Path:      path,
			Title:     dc.title,
			Order:     dc.order,
			AgentKind: dc.agentKind,
			Prompt:    dc.prompt,
			doc:       &Doc{},
		}
		if err := col.Save(); err != nil {
			return "", nil, fmt.Errorf("criando coluna %s: %w", dc.key, err)
		}
		created = append(created, filepath.Join("columns", dc.key+".md"))
	}

	return root, created, nil
}
