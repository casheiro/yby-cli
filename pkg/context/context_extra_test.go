package context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManager_GetCurrent_CurrentInvalido cobre o branch onde manifest.Current
// não existe no mapa de environments (linha 91).
func TestManager_GetCurrent_CurrentInvalido(t *testing.T) {
	dir := t.TempDir()
	ybyDir := filepath.Join(dir, ".yby")
	require.NoError(t, os.MkdirAll(ybyDir, 0755))

	// current aponta para "nao-existe" que não está no mapa
	yaml := `current: nao-existe
environments:
  local:
    type: local
    description: Local
    values: config/values-local.yaml
`
	require.NoError(t, os.WriteFile(filepath.Join(ybyDir, "environments.yaml"), []byte(yaml), 0644))

	t.Setenv("YBY_ENV", "")

	m := NewManager(dir)
	_, _, err := m.GetCurrent()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inválido ou não encontrado")
}

// TestManager_SaveManifest_DiretorioInexistente cobre o branch de erro em SaveManifest
// quando o diretório .yby não existe.
func TestManager_SaveManifest_DiretorioInexistente(t *testing.T) {
	// Diretório que não existe para provocar erro no WriteFile
	m := NewManager("/caminho/inexistente/xyz")
	manifest := &EnvironmentsManifest{
		Current:      "dev",
		Environments: map[string]Environment{},
	}
	err := m.SaveManifest(manifest)
	assert.Error(t, err)
}

// TestManager_SetCurrent_LoadManifestFalha cobre a linha 96 onde LoadManifest
// retorna erro dentro de SetCurrent.
func TestManager_SetCurrent_LoadManifestFalha(t *testing.T) {
	m := NewManager("/caminho/inexistente/xyz")
	err := m.SetCurrent("prod")
	assert.Error(t, err)
}

// TestManager_AddEnvironment_LoadManifestFalha cobre o branch onde LoadManifest
// retorna erro dentro de AddEnvironment.
func TestManager_AddEnvironment_LoadManifestFalha(t *testing.T) {
	m := NewManager("/caminho/inexistente/xyz")
	env := Environment{Type: "remote", Description: "Staging"}
	err := m.AddEnvironment("staging", env, "")
	assert.Error(t, err)
}

// TestManager_ValidateIntegrity_Integro verifica que não há warnings quando todos
// os arquivos de values existem.
func TestManager_ValidateIntegrity_Integro(t *testing.T) {
	dir := t.TempDir()
	ybyDir := filepath.Join(dir, ".yby")
	require.NoError(t, os.MkdirAll(ybyDir, 0755))

	configDir := filepath.Join(dir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "values-local.yaml"), []byte("# local"), 0644))

	yaml := `current: local
environments:
  local:
    type: local
    description: Local
    values: config/values-local.yaml
`
	require.NoError(t, os.WriteFile(filepath.Join(ybyDir, "environments.yaml"), []byte(yaml), 0644))

	m := NewManager(dir)
	warnings, err := m.ValidateIntegrity()
	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

// TestManager_ValidateIntegrity_ArquivoAusente verifica que um warning é retornado
// quando o arquivo de values não existe.
func TestManager_ValidateIntegrity_ArquivoAusente(t *testing.T) {
	dir := t.TempDir()
	ybyDir := filepath.Join(dir, ".yby")
	require.NoError(t, os.MkdirAll(ybyDir, 0755))

	yaml := `current: local
environments:
  local:
    type: local
    description: Local
    values: config/values-local.yaml
`
	require.NoError(t, os.WriteFile(filepath.Join(ybyDir, "environments.yaml"), []byte(yaml), 0644))

	m := NewManager(dir)
	warnings, err := m.ValidateIntegrity()
	assert.NoError(t, err)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "ambiente 'local'")
	assert.Contains(t, warnings[0], "não encontrado")
}

// TestManager_ValidateIntegrity_LoadManifestFalha cobre o branch de erro
// quando LoadManifest falha em ValidateIntegrity.
func TestManager_ValidateIntegrity_LoadManifestFalha(t *testing.T) {
	m := NewManager("/caminho/inexistente/xyz")
	_, err := m.ValidateIntegrity()
	assert.Error(t, err)
}

// TestManager_LoadManifest_ReadFileFalha cobre o branch de ReadFile retornando
// erro que não é "not exist" (linha 48).
func TestManager_LoadManifest_ReadFileFalha(t *testing.T) {
	dir := t.TempDir()
	ybyDir := filepath.Join(dir, ".yby")
	require.NoError(t, os.MkdirAll(ybyDir, 0755))

	// Cria environments.yaml como diretório para provocar erro de leitura
	require.NoError(t, os.MkdirAll(filepath.Join(ybyDir, "environments.yaml"), 0755))

	m := NewManager(dir)
	_, err := m.LoadManifest()
	assert.Error(t, err)
}

// TestEnvironment_SemCampoCloud verifica que um environments.yaml sem o campo
// "cloud" carrega sem erro e com Cloud == nil (retro-compatibilidade).
func TestEnvironment_SemCampoCloud(t *testing.T) {
	dir := t.TempDir()
	ybyDir := filepath.Join(dir, ".yby")
	require.NoError(t, os.MkdirAll(ybyDir, 0755))

	yamlLegado := `current: local
environments:
  local:
    type: local
    description: Local dev
    values: config/values-local.yaml
    kube_context: kind-local
    namespace: default
`
	require.NoError(t, os.WriteFile(filepath.Join(ybyDir, "environments.yaml"), []byte(yamlLegado), 0644))

	m := NewManager(dir)
	manifest, err := m.LoadManifest()
	if err != nil {
		t.Fatalf("environments.yaml sem campo cloud não deveria falhar: %v", err)
	}
	env, ok := manifest.Environments["local"]
	if !ok {
		t.Fatal("ambiente 'local' deveria existir")
	}
	if env.Cloud != nil {
		t.Errorf("Cloud deveria ser nil quando campo ausente, got %+v", env.Cloud)
	}
}

// TestEnvironment_ComCampoCloud verifica que os campos de CloudConfig são
// carregados corretamente quando presentes no environments.yaml.
func TestEnvironment_ComCampoCloud(t *testing.T) {
	dir := t.TempDir()
	ybyDir := filepath.Join(dir, ".yby")
	require.NoError(t, os.MkdirAll(ybyDir, 0755))

	yamlCloud := `current: prod
environments:
  prod:
    type: eks
    description: Producao AWS
    values: config/values-prod.yaml
    cloud:
      provider: aws
      region: us-east-1
      cluster: prod-cluster
      profile: default
      role_arn: arn:aws:iam::123456789012:role/eks-role
`
	require.NoError(t, os.WriteFile(filepath.Join(ybyDir, "environments.yaml"), []byte(yamlCloud), 0644))

	m := NewManager(dir)
	manifest, err := m.LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest falhou: %v", err)
	}
	env, ok := manifest.Environments["prod"]
	if !ok {
		t.Fatal("ambiente 'prod' deveria existir")
	}
	if env.Cloud == nil {
		t.Fatal("Cloud deveria estar preenchido")
	}
	if env.Cloud.Provider != "aws" {
		t.Errorf("Cloud.Provider esperado 'aws', got '%s'", env.Cloud.Provider)
	}
	if env.Cloud.Region != "us-east-1" {
		t.Errorf("Cloud.Region esperado 'us-east-1', got '%s'", env.Cloud.Region)
	}
	if env.Cloud.Cluster != "prod-cluster" {
		t.Errorf("Cloud.Cluster esperado 'prod-cluster', got '%s'", env.Cloud.Cluster)
	}
	if env.Cloud.RoleARN != "arn:aws:iam::123456789012:role/eks-role" {
		t.Errorf("Cloud.RoleARN inesperado: %s", env.Cloud.RoleARN)
	}
}
