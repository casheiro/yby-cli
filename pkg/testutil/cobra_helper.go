package testutil

import (
	"bytes"

	"github.com/spf13/cobra"
)

// ExecuteCommand executa um comando Cobra em memória e retorna o output capturado.
// Útil para testar comandos sem necessidade de execução real.
func ExecuteCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}
