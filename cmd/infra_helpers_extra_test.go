package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFindInfraRoot_DirectYby valida que .yby no diretório atual é encontrado corretamente.
func TestFindInfraRoot_DirectYby(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".yby"), 0755))

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	root, err := FindInfraRoot()
	require.NoError(t, err)
	assert.Equal(t, dir, root)
}

// TestFindInfraRoot_MonorepoWithInfra valida detecção de monorepo com infra/.yby.
func TestFindInfraRoot_MonorepoWithInfra(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "infra", ".yby"), 0755))

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	root, err := FindInfraRoot()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "infra"), root)
}

// TestFindInfraRoot_ChildDir valida busca ascendente a partir de subdiretório profundo.
func TestFindInfraRoot_ChildDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".yby"), 0755))
	childDir := filepath.Join(dir, "subdir", "deep")
	require.NoError(t, os.MkdirAll(childDir, 0755))

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(childDir)

	root, err := FindInfraRoot()
	require.NoError(t, err)
	assert.Equal(t, dir, root)
}

// TestFindInfraRoot_NotFound_MensagemErro valida que a mensagem de erro contém informação diagnóstica.
func TestFindInfraRoot_NotFound_MensagemErro(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	_, err := FindInfraRoot()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "raiz da infra não encontrada")
}

// TestFindInfraRoot_PriorizaDiretorioAtual verifica que .yby no diretório atual tem
// prioridade sobre infra/.yby do mesmo diretório.
func TestFindInfraRoot_PriorizaDiretorioAtual(t *testing.T) {
	dir := t.TempDir()
	// Cria tanto .yby quanto infra/.yby
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".yby"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "infra", ".yby"), 0755))

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	root, err := FindInfraRoot()
	require.NoError(t, err)
	// .yby no diretório atual deve ter prioridade sobre infra/.yby
	assert.Equal(t, dir, root)
}

// TestFindInfraRoot_SubdirProfundo3Niveis valida busca ascendente com 3+ níveis de profundidade.
func TestFindInfraRoot_SubdirProfundo3Niveis(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".yby"), 0755))
	deepDir := filepath.Join(dir, "a", "b", "c", "d")
	require.NoError(t, os.MkdirAll(deepDir, 0755))

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(deepDir)

	root, err := FindInfraRoot()
	require.NoError(t, err)
	assert.Equal(t, dir, root)
}

// TestJoinInfra_CaminhoComplexo valida join com caminhos compostos.
func TestJoinInfra_CaminhoComplexo(t *testing.T) {
	result := JoinInfra("/root", "charts/system")
	assert.Equal(t, filepath.Join("/root", "charts/system"), result)
}

// TestJoinInfra_RaizVazia valida join quando root é vazio.
func TestJoinInfra_RaizVazia(t *testing.T) {
	result := JoinInfra("", "arquivo.yaml")
	assert.Equal(t, "arquivo.yaml", result)
}

// TestJoinInfra_PathVazio valida join quando path é vazio.
func TestJoinInfra_PathVazio(t *testing.T) {
	result := JoinInfra("/root", "")
	assert.Equal(t, "/root", result)
}

// TestJoinInfra_AmbosVazios valida join quando ambos são vazios.
func TestJoinInfra_AmbosVazios(t *testing.T) {
	result := JoinInfra("", "")
	assert.Equal(t, "", result)
}
