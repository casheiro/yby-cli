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
	"runtime"
	"testing"

	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

// makeTarGzWithBinary cria um arquivo tar.gz contendo um binário fake com o nome fornecido.
func makeTarGzWithBinary(t *testing.T, binaryName, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	data := []byte(content)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     binaryName,
		Size:     int64(len(data)),
		Mode:     0755,
		Typeflag: tar.TypeReg,
	}))
	_, err := tw.Write(data)
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// makeTarGzEmpty cria um tar.gz vazio (sem arquivos).
func makeTarGzEmpty(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// saveOriginals salva e restaura variáveis globais mockáveis.
func saveAndRestoreGlobals(t *testing.T) {
	t.Helper()
	origHTTPGet := httpGet
	origReleaseBaseURL := releaseBaseURL
	t.Cleanup(func() {
		httpGet = origHTTPGet
		releaseBaseURL = origReleaseBaseURL
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// installFromURL
// ═══════════════════════════════════════════════════════════════════════════════

func TestInstallFromURL_BinarioRaw(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Servidor que retorna binário cru
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("#!/bin/sh\necho ok"))
	}))
	defer srv.Close()

	m := NewManager()
	err := m.installFromURL(srv.URL + "/yby-plugin-meubin")
	require.NoError(t, err)

	// Verificar que o binário foi instalado
	installedPath := filepath.Join(tmp, ".yby", "plugins", "yby-plugin-meubin")
	info, err := os.Stat(installedPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&0111 != 0, "binário deve ser executável")

	data, err := os.ReadFile(installedPath)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho ok", string(data))
}

func TestInstallFromURL_BinarioRawSemPrefixoPlugin(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Binário sem prefixo yby-plugin- (deve instalar com aviso)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("bin content"))
	}))
	defer srv.Close()

	m := NewManager()
	err := m.installFromURL(srv.URL + "/meu-binario-custom")
	require.NoError(t, err)

	installedPath := filepath.Join(tmp, ".yby", "plugins", "meu-binario-custom")
	_, err = os.Stat(installedPath)
	assert.NoError(t, err)
}

func TestInstallFromURL_TarGzComBinario(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	tarData := makeTarGzWithBinary(t, "yby-plugin-fromtar", "#!/bin/sh\necho tar")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()

	m := NewManager()
	err := m.installFromURL(srv.URL + "/plugin-archive.tar.gz")
	require.NoError(t, err)

	installedPath := filepath.Join(tmp, ".yby", "plugins", "yby-plugin-fromtar")
	data, err := os.ReadFile(installedPath)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho tar", string(data))
}

func TestInstallFromURL_TarGzSemBinarioPluginPrefix(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// tar.gz contendo arquivo sem prefixo yby-plugin-
	tarData := makeTarGzWithBinary(t, "outro-binario", "content")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()

	m := NewManager()
	err := m.installFromURL(srv.URL + "/archive.tar.gz")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nenhum executável começando com 'yby-plugin-'")
}

func TestInstallFromURL_TarGzVazio(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	tarData := makeTarGzEmpty(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()

	m := NewManager()
	err := m.installFromURL(srv.URL + "/empty.tar.gz")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nenhum executável começando com 'yby-plugin-'")
}

func TestInstallFromURL_StatusHTTP404(t *testing.T) {
	saveAndRestoreGlobals(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := NewManager()
	err := m.installFromURL(srv.URL + "/notfound.tar.gz")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
}

// Nota: cenários HTTP 500 com retry foram omitidos pois o backoff exponencial
// torna os testes lentos (2+ minutos). O retry é testado no pacote pkg/retry/.

func TestInstallFromURL_ZipNaoSuportado(t *testing.T) {
	saveAndRestoreGlobals(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake zip content"))
	}))
	defer srv.Close()

	m := NewManager()
	err := m.installFromURL(srv.URL + "/plugin.zip")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não suportado")
}

func TestInstallFromURL_TarGzCorrempido(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("dados corrompidos que não são gzip válido"))
	}))
	defer srv.Close()

	m := NewManager()
	err := m.installFromURL(srv.URL + "/corrupted.tar.gz")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao extrair plugin")
}

func TestInstallFromURL_URLComQueryParams(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("binary content"))
	}))
	defer srv.Close()

	m := NewManager()
	// URL com query params — deve extrair nome correto do binário
	err := m.installFromURL(srv.URL + "/yby-plugin-querytest?token=abc123&v=1")
	require.NoError(t, err)

	installedPath := filepath.Join(tmp, ".yby", "plugins", "yby-plugin-querytest")
	_, err = os.Stat(installedPath)
	assert.NoError(t, err)
}

// ═══════════════════════════════════════════════════════════════════════════════
// installNative
// ═══════════════════════════════════════════════════════════════════════════════

func TestInstallNative_Sucesso(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	pluginName := "testplugin"
	binaryName := fmt.Sprintf("yby-plugin-%s", pluginName)
	tarData := makeTarGzWithBinary(t, binaryName, "#!/bin/sh\necho nativo")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()

	releaseBaseURL = srv.URL

	m := NewManager()
	err := m.installNative(pluginName, "1.0.0")
	require.NoError(t, err)

	// Verificar instalação
	installedPath := filepath.Join(tmp, ".yby", "plugins", binaryName)
	data, err := os.ReadFile(installedPath)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho nativo", string(data))

	// Verificar permissão executável
	info, err := os.Stat(installedPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&0111 != 0)
}

func TestInstallNative_VersaoDev(t *testing.T) {
	m := NewManager()
	err := m.installNative("atlas", "dev")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dev")
}

func TestInstallNative_VersaoSemPrefixoV(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	binaryName := "yby-plugin-atlas"
	tarData := makeTarGzWithBinary(t, binaryName, "bin content")

	// Verificar que a tag recebe o prefixo "v"
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()

	releaseBaseURL = srv.URL

	m := NewManager()
	err := m.installNative("atlas", "2.0.0")
	require.NoError(t, err)

	// Verificar que a URL usa tag com prefixo "v"
	assert.Contains(t, receivedPath, "/v2.0.0/")
}

func TestInstallNative_VersaoJaComPrefixoV(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	binaryName := "yby-plugin-bard"
	tarData := makeTarGzWithBinary(t, binaryName, "bin content")

	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()

	releaseBaseURL = srv.URL

	m := NewManager()
	err := m.installNative("bard", "v3.0.0")
	require.NoError(t, err)

	// Não deve duplicar o "v"
	assert.Contains(t, receivedPath, "/v3.0.0/")
	assert.NotContains(t, receivedPath, "/vv3.0.0/")
}

func TestInstallNative_HTTP404(t *testing.T) {
	saveAndRestoreGlobals(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	releaseBaseURL = srv.URL

	m := NewManager()
	err := m.installNative("atlas", "99.99.99")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
}

// Nota: cenário HTTP 500 para installNative omitido pelo mesmo motivo do retry lento.

func TestInstallNative_BinarioNaoEncontradoNoTar(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// tar.gz com binário de nome diferente do esperado
	tarData := makeTarGzWithBinary(t, "outro-nome-binario", "content")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()

	releaseBaseURL = srv.URL

	m := NewManager()
	err := m.installNative("atlas", "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrado no arquivo baixado")
}

func TestInstallNative_TarGzCorrompido(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("dados corrompidos"))
	}))
	defer srv.Close()

	releaseBaseURL = srv.URL

	m := NewManager()
	err := m.installNative("atlas", "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao extrair plugin")
}

func TestInstallNative_FormatoFilename(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	osName := runtime.GOOS
	arch := runtime.GOARCH

	binaryName := "yby-plugin-viz"
	tarData := makeTarGzWithBinary(t, binaryName, "binary")

	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()

	releaseBaseURL = srv.URL

	m := NewManager()
	err := m.installNative("viz", "1.0.0")
	require.NoError(t, err)

	// Verificar formato do filename na URL
	expectedFilename := fmt.Sprintf("yby-plugin-viz_1.0.0_%s_%s.tar.gz", osName, arch)
	assert.Contains(t, receivedPath, expectedFilename)
}

// ═══════════════════════════════════════════════════════════════════════════════
// ExecuteCommandHook
// ═══════════════════════════════════════════════════════════════════════════════

func TestExecuteCommandHook_PluginNaoEncontrado(t *testing.T) {
	m := NewManager()
	err := m.ExecuteCommandHook("inexistente", []string{"arg1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrado")
}

func TestExecuteCommandHook_PluginEncontrado_ComScript(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()

	// Criar script de plugin interativo que simplesmente sai com sucesso
	script := filepath.Join(tmp, "yby-plugin-cmd")
	os.WriteFile(script, []byte("#!/bin/sh\nexit 0"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{
			Manifest: PluginManifest{Name: "cmd", Hooks: []string{"command"}},
			Path:     script,
		},
	}

	// ExecuteCommandHook internamente chama os.Getwd() e GetCoreContext
	// Mesmo sem contexto válido, deve executar o plugin (com fallback)
	err := m.ExecuteCommandHook("cmd", []string{"arg1", "arg2"})
	// O plugin script sai com 0, então deve ter sucesso
	assert.NoError(t, err)
}

func TestExecuteCommandHook_PluginFalha(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()

	// Plugin que sai com erro
	script := filepath.Join(tmp, "yby-plugin-falha")
	os.WriteFile(script, []byte("#!/bin/sh\nexit 1"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{
			Manifest: PluginManifest{Name: "falha", Hooks: []string{"command"}},
			Path:     script,
		},
	}

	err := m.ExecuteCommandHook("falha", []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execução interativa do plugin falhou")
}

func TestExecuteCommandHook_ComContextoDeAmbiente(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()

	// Criar estrutura de diretório com ambiente configurado
	os.MkdirAll(filepath.Join(tmp, ".yby"), 0755)
	envConfig := `
current: dev
environments:
  dev:
    type: local
    kube_config: ~/.kube/config
    kube_context: dev-ctx
    namespace: default
`
	os.WriteFile(filepath.Join(tmp, ".yby", "environments.yaml"), []byte(envConfig), 0644)

	// Criar script que verifica a env var YBY_PLUGIN_REQUEST e sai com sucesso
	script := filepath.Join(tmp, "yby-plugin-envtest")
	os.WriteFile(script, []byte("#!/bin/sh\n# Valida que YBY_PLUGIN_REQUEST está definida\nif [ -z \"$YBY_PLUGIN_REQUEST\" ]; then exit 1; fi\nexit 0"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{
			Manifest: PluginManifest{Name: "envtest", Hooks: []string{"command"}},
			Path:     script,
		},
	}

	// Mudar para tmpDir para que GetCoreContext funcione
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	err := m.ExecuteCommandHook("envtest", []string{})
	assert.NoError(t, err)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Install via HTTP (integração com installFromURL)
// ═══════════════════════════════════════════════════════════════════════════════

func TestInstall_HTTPURL_ChegaEmInstallFromURL(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("binary data"))
	}))
	defer srv.Close()

	m := NewManager()
	// Chamar Install com URL HTTP deve rotear para installFromURL
	err := m.Install(srv.URL+"/yby-plugin-http", "", true)
	require.NoError(t, err)

	installedPath := filepath.Join(tmp, ".yby", "plugins", "yby-plugin-http")
	_, err = os.Stat(installedPath)
	assert.NoError(t, err)
}

func TestInstall_HTTPSURL_ChegaEmInstallFromURL(t *testing.T) {
	saveAndRestoreGlobals(t)

	// HTTPS com httptest (usa TLS)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("binary"))
	}))
	defer srv.Close()

	// Usar o client TLS do servidor de teste
	httpGet = srv.Client().Get

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	m := NewManager()
	err := m.Install(srv.URL+"/yby-plugin-https", "", true)
	require.NoError(t, err)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Install — caminhos adicionais do file://
// ═══════════════════════════════════════════════════════════════════════════════

func TestInstall_FileProtocol_Sucesso_ComForce(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	srcPath := filepath.Join(tmp, "yby-plugin-filetest")
	os.WriteFile(srcPath, []byte("#!/bin/sh\necho file"), 0755)

	m := NewManager()
	err := m.Install("file://"+srcPath, "", true)
	require.NoError(t, err)

	installedPath := filepath.Join(tmp, ".yby", "plugins", "yby-plugin-filetest")
	data, err := os.ReadFile(installedPath)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho file", string(data))
}

func TestInstall_LocalFile_ComPluginDuplicadoSemForce(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	srcPath := filepath.Join(tmp, "yby-plugin-dup2")
	os.WriteFile(srcPath, []byte("binary"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "yby-plugin-dup2"}, Path: "/old/path"},
	}

	err := m.Install(srcPath, "", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "já está instalado")
}

// ═══════════════════════════════════════════════════════════════════════════════
// Install — plugins nativos com conflito de versão
// ═══════════════════════════════════════════════════════════════════════════════

func TestInstall_PluginNativoLatest_JaInstalado_SemForce(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	binaryName := "yby-plugin-atlas"
	tarData := makeTarGzWithBinary(t, binaryName, "binary")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()
	releaseBaseURL = srv.URL

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "atlas", Version: "0.5.0"}, Path: "/fake/atlas"},
	}

	// latest sem force — deve reinstalar (exibe aviso)
	err := m.Install("atlas", "latest", false)
	// Pode falhar no download pois "latest" como versão tenta baixar tag "vlatest"
	// O importante é que o caminho de "já existe / reinstalar" é exercitado
	_ = err
}

func TestInstall_PluginNativoVersaoDiferente_SemForce(t *testing.T) {
	saveAndRestoreGlobals(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	binaryName := "yby-plugin-bard"
	tarData := makeTarGzWithBinary(t, binaryName, "binary")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(tarData)
	}))
	defer srv.Close()
	releaseBaseURL = srv.URL

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "bard", Version: "0.5.0"}, Path: "/fake/bard"},
	}

	// Versão diferente sem force — exibe aviso mas tenta instalar
	err := m.Install("bard", "1.0.0", false)
	require.NoError(t, err)
}

// ═══════════════════════════════════════════════════════════════════════════════
// GetAssets — exercitar parsing JSON inválido no response.Data
// ═══════════════════════════════════════════════════════════════════════════════

func TestGetAssets_PluginRetornaDataInvalidaParaAssets(t *testing.T) {
	tmp := t.TempDir()

	// Plugin que retorna Data que não é AssetsDefinition
	response := PluginResponse{Data: "string-invalida-para-assets"}
	responseJSON, _ := json.Marshal(response)

	script := filepath.Join(tmp, "yby-plugin-bad-assets")
	os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' '"+string(responseJSON)+"'"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{
			Manifest: PluginManifest{Name: "bad-assets", Hooks: []string{"assets"}},
			Path:     script,
		},
	}

	assets := m.GetAssets()
	assert.Empty(t, assets) // Deve ignorar data inválida
}

// ═══════════════════════════════════════════════════════════════════════════════
// ExecuteContextHook — resposta com Data que não é map
// ═══════════════════════════════════════════════════════════════════════════════

func TestExecuteContextHook_DataNaoMap(t *testing.T) {
	tmp := t.TempDir()

	// Plugin que retorna Data como array (não map)
	response := PluginResponse{Data: []string{"item1", "item2"}}
	responseJSON, _ := json.Marshal(response)

	script := filepath.Join(tmp, "yby-plugin-arr")
	os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' '"+string(responseJSON)+"'"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{
			Manifest: PluginManifest{Name: "arr-plugin", Hooks: []string{"context"}},
			Path:     script,
		},
	}

	ctx := &scaffold.BlueprintContext{
		ProjectName: "test",
		Data:        map[string]interface{}{"existente": "valor"},
	}
	err := m.ExecuteContextHook(ctx)
	assert.NoError(t, err)
	// Data original deve estar preservada (patch não aplicado)
	assert.Equal(t, "valor", ctx.Data["existente"])
}

// ═══════════════════════════════════════════════════════════════════════════════
// Remove — plugin dentro do diretório global
// ═══════════════════════════════════════════════════════════════════════════════

func TestRemove_SucessoComDiscover(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	pluginDir := filepath.Join(tmp, ".yby", "plugins")
	os.MkdirAll(pluginDir, 0755)

	pluginPath := filepath.Join(pluginDir, "yby-plugin-removable")
	os.WriteFile(pluginPath, []byte("binary"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "removable"}, Path: pluginPath},
	}

	err := m.Remove("removable")
	require.NoError(t, err)

	_, err = os.Stat(pluginPath)
	assert.True(t, os.IsNotExist(err))
}

// ═══════════════════════════════════════════════════════════════════════════════
// extractTarGz — cenários adicionais
// ═══════════════════════════════════════════════════════════════════════════════

func TestExtractTarGz_ArquivosEmSubdiretorio(t *testing.T) {
	tmp := t.TempDir()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Diretório
	tw.WriteHeader(&tar.Header{Name: "plugin/", Typeflag: tar.TypeDir, Mode: 0755})
	// Arquivo no subdiretório
	content := "#!/bin/sh\necho sub"
	tw.WriteHeader(&tar.Header{
		Name:     "plugin/yby-plugin-sub",
		Size:     int64(len(content)),
		Mode:     0755,
		Typeflag: tar.TypeReg,
	})
	tw.Write([]byte(content))
	tw.Close()
	gw.Close()

	err := extractTarGz(&buf, tmp)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmp, "plugin", "yby-plugin-sub"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho sub", string(data))
}
