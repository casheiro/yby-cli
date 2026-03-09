package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteCommandDocs_Simple(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "test.md")

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()

	cmd := &cobra.Command{
		Use:   "test-cmd",
		Short: "Comando de teste",
	}

	err = writeCommandDocs(f, cmd)
	assert.NoError(t, err)

	f.Close()
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-cmd")
}

func TestWriteCommandDocs_WithChildren(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "test.md")

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()

	parent := &cobra.Command{Use: "parent", Short: "Pai"}
	child := &cobra.Command{Use: "child", Short: "Filho", Run: func(cmd *cobra.Command, args []string) {}}
	parent.AddCommand(child)

	err = writeCommandDocs(f, parent)
	assert.NoError(t, err)

	f.Close()
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "parent")
	assert.Contains(t, string(data), "child")
}

func TestGenDocsCmd_Success(t *testing.T) {
	dir := t.TempDir()
	genDocsCmd.SetArgs([]string{dir})
	genDocsCmd.Run(genDocsCmd, []string{dir})

	// Verifica que o arquivo foi criado
	_, err := os.Stat(filepath.Join(dir, "CLI-Reference.md"))
	assert.NoError(t, err)
}
