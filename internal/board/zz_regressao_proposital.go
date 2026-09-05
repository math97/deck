package board

import "os"

// regressaoProposital existe só para provar que o quality gate reprova um PR.
// Este arquivo é apagado logo depois.
func regressaoProposital() {
	os.WriteFile("/tmp/x", nil, 0o777)
}
