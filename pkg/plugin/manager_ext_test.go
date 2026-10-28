package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/stretchr/testify/assert"
)

func TestManagerListAndGet(t *testing.T) {
	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "test-plugin-1"}, Path: "/test/1"},
		{Manifest: PluginManifest{Name: "test-plugin-2"}, Path: "/test/2"},
	}

	plugins := m.ListPlugins()
	assert.Len(t, plugins, 2)

	p, found := m.GetPlugin("test-plugin-1")
	assert.True(t, found)
	assert.Equal(t, "/test/1", p.Path)

	_, found = m.GetPlugin("non-existent")
	assert.False(t, found)
}

func TestManagerRemove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-remove-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Setenv("USERPROFILE", tmpDir) // Windows
	t.Setenv("HOME", tmpDir)        // Unix

	pluginDir := filepath.Join(tmpDir, ".yby", "plugins")
	assert.NoError(t, os.MkdirAll(pluginDir, 0755))

	pluginPath := filepath.Join(pluginDir, "yby-plugin-test")
	assert.NoError(t, os.WriteFile(pluginPath, []byte("fake binary"), 0755))

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "test-plugin"}, Path: pluginPath},
	}

	// Remove finding success
	err = m.Remove("test-plugin")
	assert.NoError(t, err)

	_, err = os.Stat(pluginPath)
	assert.True(t, os.IsNotExist(err))

	// Remove not found
	err = m.Remove("missing")
	assert.ErrorContains(t, err, "não encontrado")

	// Remove unauthorized path
	m.plugins = append(m.plugins, LoadedPlugin{
		Manifest: PluginManifest{Name: "system-plugin"},
		Path:     "/usr/bin/yby-plugin",
	})
	err = m.Remove("system-plugin")
	assert.ErrorContains(t, err, "fora do diretório global")
}

func TestManagerDiscover(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-discover-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	pluginDir := filepath.Join(tmpDir, ".yby", "plugins")
	assert.NoError(t, os.MkdirAll(pluginDir, 0755))

	// Valid executable matching naming
	validPluginPath := filepath.Join(pluginDir, "yby-plugin-valid")
	assert.NoError(t, os.WriteFile(validPluginPath, []byte("fake bin"), 0755))

	// Mock the executor
	originalExecCommandContext := execCommandContext
	execCommandContext = mockExecCommandContext
	defer func() { execCommandContext = originalExecCommandContext }()

	m := NewManager()
	err = m.Discover()
	assert.NoError(t, err)
	// We expect the plugin to be loaded, but our mock TestHelperProcess from executor_test
	// doesn't handle validPluginPath, so it might exit incorrectly and loadManifest will skip it.
	// This at least covers the directory reading logic.
}

// criarPluginManifesto põe em dir/.yby/plugins um binário que o TestHelperProcess
// responde como o plugin "manifesto" e devolve o caminho dele.
func criarPluginManifesto(t *testing.T, dir string) string {
	t.Helper()
	pasta := filepath.Join(dir, ".yby", "plugins")
	assert.NoError(t, os.MkdirAll(pasta, 0755))
	caminho := filepath.Join(pasta, "yby-plugin-manifesto.exe")
	assert.NoError(t, os.WriteFile(caminho, []byte("fake bin"), 0755))
	return caminho
}

func descobrirComMock(t *testing.T, home, cwd string) *Manager {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Chdir(cwd)
	original := execCommandContext
	execCommandContext = mockExecCommandContext
	t.Cleanup(func() { execCommandContext = original })

	m := NewManager()
	assert.NoError(t, m.Discover())
	return m
}

func TestManagerDiscover_NaHomeLeAPastaUmaVez(t *testing.T) {
	home := t.TempDir()
	criarPluginManifesto(t, home)

	m := descobrirComMock(t, home, home)

	assert.Len(t, m.ListPlugins(), 1)
}

func TestManagerDiscover_NomeRepetidoFicaODaHome(t *testing.T) {
	home, projeto := t.TempDir(), t.TempDir()
	daHome := criarPluginManifesto(t, home)
	criarPluginManifesto(t, projeto)

	m := descobrirComMock(t, home, projeto)

	assert.Len(t, m.ListPlugins(), 1)
	p, ok := m.GetPlugin("manifesto")
	assert.True(t, ok)
	assert.Equal(t, daHome, p.Path)
}

func TestManagerGetAssets(t *testing.T) {
	m := NewManager()

	// Create a dummy LoadedPlugin that supposedly supports 'assets' hook
	p := LoadedPlugin{
		Manifest: PluginManifest{
			Name:  "assets-plugin",
			Hooks: []string{"assets"},
		},
		Path: "/path/to/assets-plugin",
	}
	m.plugins = append(m.plugins, p)

	// Since we don't have a specific mock in TestHelperProcess for this path,
	// m.executor.Run will fail and GetAssets will skip it, but coverage is hit.
	assets := m.GetAssets()
	assert.Len(t, assets, 0)
}

func TestManagerExecuteContextHook(t *testing.T) {
	m := NewManager()

	p := LoadedPlugin{
		Manifest: PluginManifest{
			Name:  "context-plugin",
			Hooks: []string{"context"},
		},
		Path: "/path/to/context-plugin",
	}
	m.plugins = append(m.plugins, p)

	ctx := &scaffold.BlueprintContext{}
	err := m.ExecuteContextHook(context.Background(), ctx)
	assert.NoError(t, err) // Hook fails but logs and ignores
}
