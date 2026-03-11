package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBootstrapClusterCmd_Structure valida a estrutura básica do comando bootstrap cluster.
func TestBootstrapClusterCmd_Structure(t *testing.T) {
	assert.Equal(t, "cluster", bootstrapClusterCmd.Use)
	assert.NotEmpty(t, bootstrapClusterCmd.Short)
	assert.NotEmpty(t, bootstrapClusterCmd.Long)
	assert.NotEmpty(t, bootstrapClusterCmd.Example)
}

// TestBootstrapClusterCmd_Help valida que --help executa sem erros.
func TestBootstrapClusterCmd_Help(t *testing.T) {
	resetCmdState(t)
	rootCmd.SetArgs([]string{"bootstrap", "cluster", "--help"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
}

// TestBootstrapClusterCmd_ESubcomandoDeBootstrap valida que cluster é subcomando de bootstrap.
func TestBootstrapClusterCmd_ESubcomandoDeBootstrap(t *testing.T) {
	encontrado := false
	for _, sub := range bootstrapCmd.Commands() {
		if sub.Use == "cluster" {
			encontrado = true
			break
		}
	}
	assert.True(t, encontrado, "comando 'cluster' deve ser subcomando de 'bootstrap'")
}

// TestBootstrapClusterCmd_TemRunE valida que o comando usa RunE (não Run).
func TestBootstrapClusterCmd_TemRunE(t *testing.T) {
	assert.NotNil(t, bootstrapClusterCmd.RunE, "comando deve usar RunE para tratamento adequado de erros")
}
