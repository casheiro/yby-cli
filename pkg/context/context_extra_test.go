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

// TestManager_AddEnvironment_LoadManifestFalha cobre a linha 111 onde LoadManifest
// retorna erro dentro de AddEnvironment.
func TestManager_AddEnvironment_LoadManifestFalha(t *testing.T) {
	m := NewManager("/caminho/inexistente/xyz")
	err := m.AddEnvironment("staging", "remote", "Staging")
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
