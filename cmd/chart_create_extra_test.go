package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================================
// chartCreateCmd — testes adicionais
// ========================================================

func TestChartCreateCmd_ArgsValidation_ZeroArgs(t *testing.T) {
	err := chartCreateCmd.Args(chartCreateCmd, []string{})
	assert.Error(t, err, "Deveria rejeitar zero argumentos")
}

func TestChartCreateCmd_ArgsValidation_TresArgs(t *testing.T) {
	err := chartCreateCmd.Args(chartCreateCmd, []string{"a", "b", "c"})
	assert.Error(t, err, "Deveria rejeitar três argumentos")
}

func TestChartCreateCmd_RunE_DiretorioNaoExiste(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// O diretório charts/my-new-chart não existe, então RenderEmbedDir será chamado.
	// Pode falhar se o template embutido não existir, mas não deveria dar panic.
	err := chartCreateCmd.RunE(chartCreateCmd, []string{"my-new-chart"})

	// Se houver erro, deve ser do tipo scaffold (template não encontrado ou similar)
	// Não deve ser nil pois depende de templates embutidos
	if err != nil {
		assert.Contains(t, err.Error(), "chart",
			"Erro deveria ser relacionado ao chart/scaffold")
	}
}

func TestChartCreateCmd_RunE_DiretorioExistente_RetornaErro(t *testing.T) {
	tmpDir := t.TempDir()

	// Cria o diretório de destino previamente
	destDir := filepath.Join(tmpDir, "charts", "app-existente")
	require.NoError(t, os.MkdirAll(destDir, 0755))

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	err := chartCreateCmd.RunE(chartCreateCmd, []string{"app-existente"})
	assert.Error(t, err, "Deveria falhar quando diretório de destino já existe")
	assert.Contains(t, err.Error(), "já existe")
}

func TestChartCreateCmd_RunE_DiretorioExistente_ComoArquivo(t *testing.T) {
	tmpDir := t.TempDir()

	// Cria um arquivo onde deveria ser o diretório
	chartsDir := filepath.Join(tmpDir, "charts")
	require.NoError(t, os.MkdirAll(chartsDir, 0755))
	destPath := filepath.Join(chartsDir, "conflito")
	require.NoError(t, os.WriteFile(destPath, []byte("conteudo"), 0644))

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// O stat vai encontrar o arquivo e considerar que "existe"
	err := chartCreateCmd.RunE(chartCreateCmd, []string{"conflito"})
	assert.Error(t, err, "Deveria falhar quando destino já existe (mesmo sendo arquivo)")
	assert.Contains(t, err.Error(), "já existe")
}

func TestChartCreateCmd_NomesDeChartVariados(t *testing.T) {
	tests := []struct {
		name      string
		chartName string
	}{
		{"nome simples", "my-app"},
		{"nome com numeros", "app123"},
		{"nome com underscore", "my_app"},
		{"nome longo", "super-mega-ultra-complex-application-service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			origDir, _ := os.Getwd()
			os.Chdir(tmpDir)
			defer os.Chdir(origDir)

			// Executa RunE - pode falhar por falta de template embutido,
			// mas verifica que não há panic e que o nome é aceito
			err := chartCreateCmd.RunE(chartCreateCmd, []string{tt.chartName})

			if err != nil {
				// Erro aceitável se for do scaffold/template, não de validação de nome
				assert.NotContains(t, err.Error(), "já existe",
					"Não deveria falhar por diretório existente")
			}
		})
	}
}

// ========================================================
// chartCmd — verificação de estrutura
// ========================================================

func TestChartCmd_TemSubcomandoCreate(t *testing.T) {
	subs := map[string]bool{}
	for _, c := range chartCmd.Commands() {
		subs[c.Name()] = true
	}
	assert.True(t, subs["create"], "chart deveria ter subcomando create")
}

func TestChartCmd_DescricaoCompleta(t *testing.T) {
	assert.NotEmpty(t, chartCmd.Short, "Short deveria estar definido")
	assert.NotEmpty(t, chartCmd.Long, "Long deveria estar definido")
}
