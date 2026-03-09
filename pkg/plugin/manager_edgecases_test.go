package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Remove — edge cases de cobertura
// ═══════════════════════════════════════════════════════════════════════════════

func TestRemove_PluginNotFound_ComDiscoverAutomatico(t *testing.T) {
	// Quando m.plugins está vazio, Remove chama Discover() antes de verificar.
	// Mesmo após Discover (que não encontra nada), o plugin não existe.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".yby", "plugins"), 0755))

	m := NewManager()
	// Verificar que plugins está vazio para forçar o caminho do Discover
	assert.Empty(t, m.plugins)

	err := m.Remove("fantasma")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrado")
}

// ═══════════════════════════════════════════════════════════════════════════════
// Update — edge cases
// ═══════════════════════════════════════════════════════════════════════════════

func TestUpdate_PluginNativo_Sucesso_ComServerHTTP(t *testing.T) {
	// Atualizar um plugin nativo com servidor HTTP real servindo o tarball
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	binaryName := "yby-plugin-bard"
	tarData := makeTarGzWithBinary(t, binaryName, "#!/bin/sh\necho atualizado")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()
	releaseBaseURL = srv.URL

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "bard", Version: "0.1.0"}, Path: "/velho/bard"},
	}

	err := m.Update("bard")
	require.NoError(t, err)

	installedPath := filepath.Join(tmp, ".yby", "plugins", binaryName)
	data, err := os.ReadFile(installedPath)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho atualizado", string(data))
}

func TestUpdate_PluginNaoInstalado_ComDiscoverVazio(t *testing.T) {
	// m.plugins vazio → chama Discover → não encontra nada → erro
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".yby", "plugins"), 0755))

	m := NewManager()
	err := m.Update("inexistente")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrado")
}

func TestUpdate_PluginNativo_Sentinel(t *testing.T) {
	// Validar que sentinel é reconhecido como nativo
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	binaryName := "yby-plugin-sentinel"
	tarData := makeTarGzWithBinary(t, binaryName, "binary")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()
	releaseBaseURL = srv.URL

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "sentinel", Version: "0.1.0"}, Path: "/old"},
	}

	err := m.Update("sentinel")
	require.NoError(t, err)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Install — edge cases adicionais
// ═══════════════════════════════════════════════════════════════════════════════

func TestInstall_ForceTrue_PluginNativoExistente(t *testing.T) {
	// Com force=true, não deve verificar conflitos antes de instalar
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	binaryName := "yby-plugin-synapstor"
	tarData := makeTarGzWithBinary(t, binaryName, "novo binário")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()
	releaseBaseURL = srv.URL

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "synapstor", Version: "0.1.0"}, Path: "/old/synapstor"},
	}

	err := m.Install("synapstor", "2.0.0", true)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmp, ".yby", "plugins", binaryName))
	require.NoError(t, err)
	assert.Equal(t, "novo binário", string(data))
}

func TestInstall_PluginNativo_VersaoLatest_SemForce_JaExiste(t *testing.T) {
	// Plugin nativo com versão "latest" já instalado, sem force
	// Deve imprimir aviso e tentar reinstalar
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	binaryName := "yby-plugin-viz"
	tarData := makeTarGzWithBinary(t, binaryName, "binary")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()
	releaseBaseURL = srv.URL

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "viz", Version: "0.5.0"}, Path: "/old/viz"},
	}

	err := m.Install("viz", "latest", false)
	// Deve tentar instalar sem erro (aviso é impresso via fmt)
	require.NoError(t, err)
}

func TestInstall_PluginNativo_VersaoDiferente_SemForce_JaExiste(t *testing.T) {
	// Plugin nativo com versão diferente, sem force, já instalado
	// Deve imprimir aviso sobre substituição e tentar instalar
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	binaryName := "yby-plugin-atlas"
	tarData := makeTarGzWithBinary(t, binaryName, "binary v2")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()
	releaseBaseURL = srv.URL

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "atlas", Version: "1.0.0"}, Path: "/old/atlas"},
	}

	err := m.Install("atlas", "2.0.0", false)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmp, ".yby", "plugins", binaryName))
	require.NoError(t, err)
	assert.Equal(t, "binary v2", string(data))
}

// ═══════════════════════════════════════════════════════════════════════════════
// installNative — edge cases
// ═══════════════════════════════════════════════════════════════════════════════

// Nota: teste de httpGet com erro de conexão omitido pois o retry com backoff
// exponencial (cenkalti/backoff) torna o teste extremamente lento (3+ minutos).

// ═══════════════════════════════════════════════════════════════════════════════
// installFromURL — edge cases
// ═══════════════════════════════════════════════════════════════════════════════

// Nota: teste de installFromURL com erro de conexão omitido pelo mesmo motivo
// do retry lento com backoff exponencial.

// ═══════════════════════════════════════════════════════════════════════════════
// extractTarGz — edge cases
// ═══════════════════════════════════════════════════════════════════════════════

func TestExtractTarGz_ComSymlink_IgnoraSymlink(t *testing.T) {
	// Tar contendo symlink — o switch/case ignora (não cria nada)
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Symlink
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "link-malicioso",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
	}))

	// Arquivo real após o symlink
	content := "conteúdo seguro"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "arquivo-ok.txt",
		Size:     int64(len(content)),
		Mode:     0644,
		Typeflag: tar.TypeReg,
	}))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	dest := t.TempDir()
	err = extractTarGz(&buf, dest)
	assert.NoError(t, err)

	// Symlink não deve ter sido criado
	_, err = os.Lstat(filepath.Join(dest, "link-malicioso"))
	assert.True(t, os.IsNotExist(err), "symlinks devem ser ignorados")

	// Arquivo real deve existir
	data, err := os.ReadFile(filepath.Join(dest, "arquivo-ok.txt"))
	require.NoError(t, err)
	assert.Equal(t, "conteúdo seguro", string(data))
}

func TestExtractTarGz_ComHardLink_Ignorado(t *testing.T) {
	// Hard links também devem ser ignorados pelo switch/case default
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "hardlink",
		Typeflag: tar.TypeLink,
		Linkname: "target",
	}))

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	dest := t.TempDir()
	err := extractTarGz(&buf, dest)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(dest, "hardlink"))
	assert.True(t, os.IsNotExist(err), "hard links devem ser ignorados")
}

// ═══════════════════════════════════════════════════════════════════════════════
// Discover — cenários de scan com plugin válido
// ═══════════════════════════════════════════════════════════════════════════════

func TestDiscover_ComPluginValido(t *testing.T) {
	// Plugin com manifesto válido deve ser carregado
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	pluginDir := filepath.Join(tmp, ".yby", "plugins")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))

	// Script que retorna manifesto JSON válido
	manifest := PluginManifest{Name: "myplugin", Version: "1.0.0", Hooks: []string{"command"}}
	response := PluginResponse{Data: manifest}
	responseJSON, _ := json.Marshal(response)

	script := filepath.Join(pluginDir, "yby-plugin-myplugin")
	require.NoError(t, os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\nprintf '%%s' '%s'", string(responseJSON))), 0755))

	m := NewManager()
	err := m.Discover()
	assert.NoError(t, err)
	assert.Len(t, m.plugins, 1)
	assert.Equal(t, "myplugin", m.plugins[0].Manifest.Name)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Install com file:// — cenário de cópia com sucesso
// ═══════════════════════════════════════════════════════════════════════════════

func TestInstall_FileProtocol_CopiaSucesso(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	srcPath := filepath.Join(tmp, "yby-plugin-mybin")
	require.NoError(t, os.WriteFile(srcPath, []byte("#!/bin/sh\necho installed"), 0755))

	m := NewManager()
	err := m.Install("file://"+srcPath, "", true)
	require.NoError(t, err)

	installedPath := filepath.Join(tmp, ".yby", "plugins", "yby-plugin-mybin")
	data, err := os.ReadFile(installedPath)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho installed", string(data))
}

func TestInstall_LocalFile_CopiaSucesso(t *testing.T) {
	// Arquivo local sem file:// que existe no disco
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	srcPath := filepath.Join(tmp, "yby-plugin-localbin")
	require.NoError(t, os.WriteFile(srcPath, []byte("local binary"), 0755))

	m := NewManager()
	err := m.Install(srcPath, "", true)
	require.NoError(t, err)

	installedPath := filepath.Join(tmp, ".yby", "plugins", "yby-plugin-localbin")
	data, err := os.ReadFile(installedPath)
	require.NoError(t, err)
	assert.Equal(t, "local binary", string(data))
}
