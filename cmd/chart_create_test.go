package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChartCreateCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "create [NAME]", chartCreateCmd.Use, "Use deveria ser 'create [NAME]'")
	assert.NotEmpty(t, chartCreateCmd.Short, "Short não deveria ser vazio")
	assert.NotNil(t, chartCreateCmd.RunE, "RunE não deveria ser nil")
}

func TestChartCreateCmd_ExigeExatamenteUmArg(t *testing.T) {
	// Cobra.ExactArgs(1) valida que exatamente 1 argumento é passado
	err := chartCreateCmd.Args(chartCreateCmd, []string{})
	assert.Error(t, err, "Deveria exigir pelo menos 1 argumento")

	err = chartCreateCmd.Args(chartCreateCmd, []string{"meu-chart"})
	assert.NoError(t, err, "Deveria aceitar exatamente 1 argumento")

	err = chartCreateCmd.Args(chartCreateCmd, []string{"a", "b"})
	assert.Error(t, err, "Deveria rejeitar mais de 1 argumento")
}

func TestChartCreateCmd_EhSubcomandoDeChart(t *testing.T) {
	found := false
	for _, sub := range chartCmd.Commands() {
		if sub.Name() == "create" {
			found = true
			break
		}
	}
	assert.True(t, found, "create deveria ser subcomando de chart")
}

func TestChartCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "chart", chartCmd.Use, "chartCmd.Use deveria ser 'chart'")
	assert.NotEmpty(t, chartCmd.Short, "chartCmd.Short não deveria ser vazio")
	assert.NotEmpty(t, chartCmd.Long, "chartCmd.Long não deveria ser vazio")
}

func TestChartCreateCmd_FalhaQuandoDiretorioExiste(t *testing.T) {
	tmpDir := t.TempDir()

	// Criar diretório de destino pré-existente
	destDir := filepath.Join(tmpDir, "charts", "existing-app")
	err := os.MkdirAll(destDir, 0755)
	assert.NoError(t, err)

	// Salvar e restaurar diretório de trabalho
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// RunE deve falhar porque o diretório de destino já existe
	err = chartCreateCmd.RunE(chartCreateCmd, []string{"existing-app"})
	assert.Error(t, err, "Deveria falhar quando diretório de destino já existe")
	assert.Contains(t, err.Error(), "já existe", "Mensagem deveria indicar que diretório já existe")
}
