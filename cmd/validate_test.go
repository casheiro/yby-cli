package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCmd_WithMock(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	// Criar diretório temporário com estrutura mínima
	dir := t.TempDir()
	ybyDir := filepath.Join(dir, ".yby")
	os.MkdirAll(ybyDir, 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "system"), 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "bootstrap"), 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "cluster-config"), 0755)
	os.MkdirAll(filepath.Join(dir, "config"), 0755)
	os.WriteFile(filepath.Join(dir, "config", "cluster-values.yaml"), []byte("{}"), 0644)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	err := validateCmd.RunE(validateCmd, []string{})
	// Com mock, comandos helm devem executar sem erro
	assert.NoError(t, err)
}
