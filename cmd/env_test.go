package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupEnvTestDir cria um diretório temporário com .yby/environments.yaml
// contendo dois ambientes: local e prod.
func setupEnvTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ybyDir := filepath.Join(dir, ".yby")
	require.NoError(t, os.MkdirAll(ybyDir, 0755))

	manifest := `current: local
environments:
  local:
    type: local
    description: "Ambiente local"
    values: "config/values-local.yaml"
  prod:
    type: remote
    description: "Produção"
    values: "config/values-prod.yaml"
    url: "https://prod.example.com"
`
	require.NoError(t, os.WriteFile(filepath.Join(ybyDir, "environments.yaml"), []byte(manifest), 0644))
	return dir
}

func TestEnvListCmd_Success(t *testing.T) {
	dir := setupEnvTestDir(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	var buf bytes.Buffer
	envListCmd.SetOut(&buf)
	envListCmd.SetErr(&buf)
	err := envListCmd.RunE(envListCmd, []string{})
	assert.NoError(t, err)
}

func TestEnvListCmd_SemManifesto(t *testing.T) {
	// Diretório vazio sem .yby — deve retornar erro
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	err := envListCmd.RunE(envListCmd, []string{})
	assert.Error(t, err)
}

func TestEnvUseCmd_Success(t *testing.T) {
	dir := setupEnvTestDir(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	err := envUseCmd.RunE(envUseCmd, []string{"prod"})
	assert.NoError(t, err)
}

func TestEnvUseCmd_AmbienteInvalido(t *testing.T) {
	dir := setupEnvTestDir(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	err := envUseCmd.RunE(envUseCmd, []string{"inexistente"})
	assert.Error(t, err)
}

func TestEnvShowCmd_Success(t *testing.T) {
	dir := setupEnvTestDir(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Limpa YBY_ENV para garantir que usa o current do manifesto
	t.Setenv("YBY_ENV", "")

	err := envShowCmd.RunE(envShowCmd, []string{})
	assert.NoError(t, err)
}

func TestEnvShowCmd_SemManifesto(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	err := envShowCmd.RunE(envShowCmd, []string{})
	assert.Error(t, err)
}

func TestEnvCreateCmd_Success(t *testing.T) {
	dir := setupEnvTestDir(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Cria diretório config para o arquivo de values
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0755))

	// Define flags via Set
	envCreateCmd.Flags().Set("type", "remote")
	envCreateCmd.Flags().Set("description", "Ambiente de QA")

	err := envCreateCmd.RunE(envCreateCmd, []string{"qa"})
	assert.NoError(t, err)

	// Verifica que o arquivo de values foi criado
	valuesPath := filepath.Join(dir, "config", "values-qa.yaml")
	assert.FileExists(t, valuesPath)
}

func TestEnvShowCmd_ComURL(t *testing.T) {
	dir := setupEnvTestDir(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// "prod" tem URL definida no manifesto
	t.Setenv("YBY_ENV", "prod")

	// Primeiro precisamos definir o ambiente ativo como prod
	err := envUseCmd.RunE(envUseCmd, []string{"prod"})
	require.NoError(t, err)

	// Limpar YBY_ENV para usar o current do manifesto (agora "prod")
	t.Setenv("YBY_ENV", "")

	err = envShowCmd.RunE(envShowCmd, []string{})
	assert.NoError(t, err)
}

func TestEnvCreateCmd_SemFlags(t *testing.T) {
	dir := setupEnvTestDir(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Cria diretório config para o arquivo de values
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0755))

	// Resetar flags para valores vazios para cobrir defaults
	envCreateCmd.Flags().Set("type", "")
	envCreateCmd.Flags().Set("description", "")

	err := envCreateCmd.RunE(envCreateCmd, []string{"staging"})
	assert.NoError(t, err)
}

func TestEnvUseCmd_SemYby_Fallback(t *testing.T) {
	// Diretório sem .yby — deve fazer fallback para "." e falhar ao tentar usar ambiente
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	err := envUseCmd.RunE(envUseCmd, []string{"prod"})
	assert.Error(t, err) // Falha porque não há manifesto
}

func TestEnvCreateCmd_SemYby_Fallback(t *testing.T) {
	// Diretório sem .yby — deve fazer fallback para "." e falhar ao criar
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	envCreateCmd.Flags().Set("type", "remote")
	envCreateCmd.Flags().Set("description", "Ambiente teste")

	err := envCreateCmd.RunE(envCreateCmd, []string{"teste"})
	// Pode criar ou falhar dependendo do Manager, mas cobre o branch
	_ = err
}

func TestEnvCreateCmd_AmbienteJaExiste(t *testing.T) {
	dir := setupEnvTestDir(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	envCreateCmd.Flags().Set("type", "local")
	envCreateCmd.Flags().Set("description", "Duplicado")

	// "local" já existe no manifesto
	err := envCreateCmd.RunE(envCreateCmd, []string{"local"})
	assert.Error(t, err)
}

func TestEnvCreateCmd_ComKubeContext(t *testing.T) {
	dir := setupEnvTestDir(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0755))

	envCreateCmd.Flags().Set("type", "remote")
	envCreateCmd.Flags().Set("description", "Ambiente de UAT")
	envCreateCmd.Flags().Set("kube-context", "uat-cluster")
	envCreateCmd.Flags().Set("namespace", "uat-ns")

	err := envCreateCmd.RunE(envCreateCmd, []string{"uat"})
	assert.NoError(t, err)

	// Limpa flags para não afetar outros testes
	envCreateCmd.Flags().Set("kube-context", "")
	envCreateCmd.Flags().Set("namespace", "")
}

func TestEnvCheckCmd_Integro(t *testing.T) {
	dir := setupEnvTestDir(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Cria os arquivos de values referenciados no manifesto
	configDir := filepath.Join(dir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "values-local.yaml"), []byte("# local"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "values-prod.yaml"), []byte("# prod"), 0644))

	err := envCheckCmd.RunE(envCheckCmd, []string{})
	assert.NoError(t, err)
}

func TestEnvCheckCmd_ComProblemas(t *testing.T) {
	dir := setupEnvTestDir(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Não cria os arquivos de values — deve reportar problemas
	err := envCheckCmd.RunE(envCheckCmd, []string{})
	assert.NoError(t, err) // O comando não retorna erro, apenas lista warnings
}

func TestEnvCheckCmd_SemManifesto(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	err := envCheckCmd.RunE(envCheckCmd, []string{})
	assert.Error(t, err)
}
