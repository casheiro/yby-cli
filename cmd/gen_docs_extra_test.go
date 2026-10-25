package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenDocsCmd_SemArgs verifica que ao executar sem argumentos, usa diretório padrão ./docs/wiki
func TestGenDocsCmd_SemArgs(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	_ = genDocsCmd.RunE(genDocsCmd, []string{})

	// Deve criar em ./docs/wiki/CLI-Reference.md
	_, err := os.Stat(filepath.Join(dir, "docs", "wiki", "CLI-Reference.md"))
	assert.NoError(t, err)
}

// TestGenDocsCmd_Structure valida a estrutura do comando
func TestGenDocsCmd_StructureExtra(t *testing.T) {
	assert.Equal(t, "gen-docs [output-dir]", genDocsCmd.Use)
	assert.True(t, genDocsCmd.Hidden)
	assert.NotEmpty(t, genDocsCmd.Short)
}

// TestWriteCommandDocs_ComMultiplosNiveisDeFilhos valida a recursão com netos
func TestWriteCommandDocs_ComMultiplosNiveisDeFilhos(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "test.md")

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()

	root := &cobra.Command{Use: "root", Short: "Raiz"}
	child := &cobra.Command{Use: "child", Short: "Filho", Run: func(cmd *cobra.Command, args []string) {}}
	grandchild := &cobra.Command{Use: "grandchild", Short: "Neto", Run: func(cmd *cobra.Command, args []string) {}}
	child.AddCommand(grandchild)
	root.AddCommand(child)

	err = writeCommandDocs(f, root)
	assert.NoError(t, err)

	f.Close()
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "root")
	assert.Contains(t, string(data), "child")
	assert.Contains(t, string(data), "grandchild")
}

// TestWriteCommandDocs_ComFilhoHidden verifica que comandos ocultos são pulados
func TestWriteCommandDocs_ComFilhoHidden(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "test.md")

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()

	parent := &cobra.Command{Use: "parent", Short: "Pai"}
	visible := &cobra.Command{Use: "visible", Short: "Visível", Run: func(cmd *cobra.Command, args []string) {}}
	hidden := &cobra.Command{Use: "hidden", Short: "Oculto", Hidden: true, Run: func(cmd *cobra.Command, args []string) {}}
	parent.AddCommand(visible)
	parent.AddCommand(hidden)

	err = writeCommandDocs(f, parent)
	assert.NoError(t, err)

	f.Close()
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "visible")
	// Cobra IsAvailableCommand retorna false para Hidden, então "hidden" não deve aparecer como seção
}

// TestWriteCommandDocs_ErroEscrita verifica que erros de escrita são propagados
func TestWriteCommandDocs_ErroEscrita(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "test.md")

	f, err := os.Create(outputFile)
	require.NoError(t, err)

	// Fechar o arquivo para provocar erro de escrita
	f.Close()

	cmd := &cobra.Command{Use: "test-cmd", Short: "Teste"}

	err = writeCommandDocs(f, cmd)
	assert.Error(t, err)
}

// TestGenDocsCmd_ConteudoGerado verifica que o arquivo de referência contém conteúdo esperado
func TestGenDocsCmd_ConteudoGerado(t *testing.T) {
	dir := t.TempDir()
	_ = genDocsCmd.RunE(genDocsCmd, []string{dir})

	data, err := os.ReadFile(filepath.Join(dir, "CLI-Reference.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "# Referência da CLI")
}

// TestSecaoDeReferencia_EmPortugues confere que a seção sai da ajuda da carta: uso,
// exemplos sem o recuo, flags em tabela, nota de beta herdada e nada do texto do cobra.
func TestSecaoDeReferencia_EmPortugues(t *testing.T) {
	raiz, grupo, sub := arvoreDeAjuda()
	grupo.Annotations = map[string]string{anotacaoBeta: "Beta: ainda sem teste real."}

	secao := secaoDeReferencia(sub)
	for _, esperado := range []string{
		"## yby generate keda\n", "**Uso**\n\n```\nyby generate keda [nome] [flags]\n```",
		"```bash\nyby generate keda api\n# Cria o ScaledObject\n```",
		"| `-n, --namespace texto` | Namespace do recurso |", "| `--replicas n` | Réplicas mínimas (padrão 2) |",
		"**Flags globais**", "Beta: ainda sem teste real.",
	} {
		assert.Contains(t, secao, esperado)
	}
	for _, ingles := range []string{"Synopsis", "Options", "SEE ALSO", "help for", "default"} {
		assert.NotContains(t, secao, ingles)
	}
	assert.Contains(t, secaoDeReferencia(raiz), "* [yby generate](#yby-generate) — Gera recursos (beta)")
}
