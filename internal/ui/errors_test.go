package ui

import "errors"

// Erros de mentira usados nos testes.
var (
	errWorktreeDirty = errors.New("worktree tem trabalho não commitado")
	errImportFailed  = errors.New("gh: not found")
)
