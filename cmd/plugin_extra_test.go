package cmd

import (
	"os"
	"testing"

	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/stretchr/testify/assert"
)

// ========================================================
// Helpers para testes de plugin
// ========================================================

// mockPluginManagerFactory substitui a factory de plugin manager para testes,
// usando um diretório temporário isolado para evitar descoberta de plugins reais.
// Retorna uma função de teardown para restaurar a factory original e HOME.
func mockPluginManagerFactory(tmpDir string) func() {
	orig := newPluginManager
	origHome := os.Getenv("HOME")

	// Define HOME para diretório temp para isolar de plugins reais
	os.Setenv("HOME", tmpDir)

	newPluginManager = func() *plugin.Manager {
		return plugin.NewManager()
	}
	return func() {
		newPluginManager = orig
		os.Setenv("HOME", origHome)
	}
}

// ========================================================
// Testes do plugin list
// ========================================================

func TestPluginListCmd_SemPlugins(t *testing.T) {
	teardown := mockPluginManagerFactory(t.TempDir())
	defer teardown()

	err := pluginListCmd.RunE(pluginListCmd, []string{})
	assert.NoError(t, err, "list sem plugins não deveria retornar erro")
}

func TestPluginListCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "list", pluginListCmd.Use)
	assert.NotEmpty(t, pluginListCmd.Short)
	assert.NotNil(t, pluginListCmd.RunE)
}

// ========================================================
// Testes do plugin install
// ========================================================

func TestPluginInstallCmd_PluginInexistente(t *testing.T) {
	teardown := mockPluginManagerFactory(t.TempDir())
	defer teardown()

	// Tenta instalar um arquivo que não existe
	err := pluginInstallCmd.RunE(pluginInstallCmd, []string{"/tmp/nao-existe-plugin"})
	assert.Error(t, err, "install de arquivo inexistente deveria retornar erro")
}

func TestPluginInstallCmd_NativoVersionDev(t *testing.T) {
	origVersion := Version
	defer func() { Version = origVersion }()
	Version = "dev"

	teardown := mockPluginManagerFactory(t.TempDir())
	defer teardown()

	// Tenta instalar plugin nativo com version=dev — deve falhar
	err := pluginInstallCmd.RunE(pluginInstallCmd, []string{"atlas"})
	assert.Error(t, err, "install de plugin nativo com version=dev deveria retornar erro")
}

func TestPluginInstallCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "install [path|name]", pluginInstallCmd.Use)
	assert.NotNil(t, pluginInstallCmd.RunE)

	// Verifica flags
	f := pluginInstallCmd.Flags().Lookup("version")
	assert.NotNil(t, f, "deveria ter flag --version")
	f = pluginInstallCmd.Flags().Lookup("force")
	assert.NotNil(t, f, "deveria ter flag --force")
}

// ========================================================
// Testes do plugin remove
// ========================================================

func TestPluginRemoveCmd_PluginNaoEncontrado(t *testing.T) {
	teardown := mockPluginManagerFactory(t.TempDir())
	defer teardown()

	err := pluginRemoveCmd.RunE(pluginRemoveCmd, []string{"plugin-inexistente"})
	assert.Error(t, err, "remove de plugin inexistente deveria retornar erro")
}

func TestPluginRemoveCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "remove [name]", pluginRemoveCmd.Use)
	assert.Contains(t, pluginRemoveCmd.Aliases, "rm")
	assert.Contains(t, pluginRemoveCmd.Aliases, "uninstall")
	assert.Contains(t, pluginRemoveCmd.Aliases, "delete")
	assert.NotNil(t, pluginRemoveCmd.RunE)
}

// ========================================================
// Testes do plugin update
// ========================================================

func TestPluginUpdateCmd_SemPlugins(t *testing.T) {
	teardown := mockPluginManagerFactory(t.TempDir())
	defer teardown()

	// Sem argumentos e sem plugins instalados — nenhum plugin para atualizar
	err := pluginUpdateCmd.RunE(pluginUpdateCmd, []string{})
	assert.NoError(t, err, "update sem plugins não deveria retornar erro")
}

func TestPluginUpdateCmd_PluginNaoEncontrado(t *testing.T) {
	teardown := mockPluginManagerFactory(t.TempDir())
	defer teardown()

	err := pluginUpdateCmd.RunE(pluginUpdateCmd, []string{"plugin-inexistente"})
	assert.Error(t, err, "update de plugin inexistente deveria retornar erro")
}

func TestPluginUpdateCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "update [name]", pluginUpdateCmd.Use)
	assert.NotNil(t, pluginUpdateCmd.RunE)
}

// ========================================================
// Teste do plugin cmd principal
// ========================================================

func TestPluginCmd_Help(t *testing.T) {
	err := pluginCmd.RunE(pluginCmd, []string{})
	assert.NoError(t, err, "plugin help não deveria retornar erro")
}

func TestPluginCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "plugin", pluginCmd.Use)
	assert.NotEmpty(t, pluginCmd.Short)
	assert.NotEmpty(t, pluginCmd.Long)
}

// ========================================================
// Teste da factory newPluginManager
// ========================================================

func TestNewPluginManagerFactory_Default(t *testing.T) {
	m := newPluginManager()
	assert.NotNil(t, m, "newPluginManager deveria retornar um manager não-nil")
}
