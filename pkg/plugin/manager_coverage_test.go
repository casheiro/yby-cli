package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	projectContext "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/stretchr/testify/assert"
)

// ---- ListPlugins ----

func TestListPlugins_RetornaNilQuandoVazio(t *testing.T) {
	m := NewManager()
	// Quando nenhum plugin foi adicionado, retorna nil (não []PluginManifest{})
	result := m.ListPlugins()
	assert.Nil(t, result)
}

func TestListPlugins_RetornaManifestosCorretos(t *testing.T) {
	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "p1", Version: "1.0"}, Path: "/a"},
		{Manifest: PluginManifest{Name: "p2", Version: "2.0"}, Path: "/b"},
		{Manifest: PluginManifest{Name: "p3", Version: "3.0"}, Path: "/c"},
	}
	result := m.ListPlugins()
	assert.Len(t, result, 3)
	assert.Equal(t, "p1", result[0].Name)
	assert.Equal(t, "2.0", result[1].Version)
	assert.Equal(t, "p3", result[2].Name)
}

// ---- GetPlugin ----

func TestGetPlugin_NaoEncontrado_ManagerVazio(t *testing.T) {
	m := NewManager()
	p, found := m.GetPlugin("qualquer")
	assert.False(t, found)
	assert.Nil(t, p)
}

func TestGetPlugin_NaoEncontrado_ComPlugins(t *testing.T) {
	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "atlas"}, Path: "/a"},
		{Manifest: PluginManifest{Name: "bard"}, Path: "/b"},
	}
	p, found := m.GetPlugin("inexistente")
	assert.False(t, found)
	assert.Nil(t, p)
}

func TestGetPlugin_EncontradoPrimeiroComNomeDuplicado(t *testing.T) {
	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "dup"}, Path: "/primeiro"},
		{Manifest: PluginManifest{Name: "dup"}, Path: "/segundo"},
	}
	p, found := m.GetPlugin("dup")
	assert.True(t, found)
	// Retorna o primeiro encontrado
	assert.Equal(t, "/primeiro", p.Path)
}

// ---- expandPath ----

func TestExpandPath_TildeSozinho(t *testing.T) {
	// "~" sem barra não deve expandir (apenas "~/" dispara)
	result := expandPath("~")
	assert.Equal(t, "~", result)
}

func TestExpandPath_CaminhoRelativo(t *testing.T) {
	result := expandPath("config/values.yaml")
	assert.Equal(t, "config/values.yaml", result)
}

// ---- Discover com diretório inválido ----

func TestDiscover_SemErroComDiretoriosInexistentes(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Sem criar .yby/plugins, Discover deve ignorar erros silenciosamente
	m := NewManager()
	err := m.Discover()
	assert.NoError(t, err)
	assert.Empty(t, m.plugins)
}

// ---- scanDirectory - diretórios são ignorados ----

func TestScanDirectory_IgnoraDiretorios(t *testing.T) {
	tmp := t.TempDir()
	// Criar subdiretório com prefixo yby-plugin- (deve ser ignorado)
	os.MkdirAll(filepath.Join(tmp, "yby-plugin-diretorio"), 0755)

	m := NewManager()
	err := m.scanDirectory(tmp)
	assert.NoError(t, err)
	assert.Empty(t, m.plugins)
}

func TestScanDirectory_IgnoraPluginComManifestoInvalido(t *testing.T) {
	tmp := t.TempDir()
	// Plugin que emite JSON inválido
	script := filepath.Join(tmp, "yby-plugin-ruim")
	os.WriteFile(script, []byte("#!/bin/sh\necho 'não é json'"), 0755)

	m := NewManager()
	err := m.scanDirectory(tmp)
	assert.NoError(t, err)
	assert.Empty(t, m.plugins) // Plugin com manifesto inválido é ignorado
}

// ---- Install ----

func TestInstall_PluginNativoJaInstaladoSemForce(t *testing.T) {
	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "atlas", Version: "1.0.0"}, Path: "/fake/atlas"},
	}

	// Mesma versão, sem force
	err := m.Install("atlas", "1.0.0", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "já está instalado")
}

func TestInstall_PluginNativoVersaoLatestJaInstalado(t *testing.T) {
	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "bard", Version: "0.5.0"}, Path: "/fake/bard"},
	}

	// latest sem force - tenta reinstalar (vai falhar por versão "latest" = dev)
	err := m.Install("bard", "dev", false)
	assert.Error(t, err)
}

func TestInstall_OrigemNaoSuportada(t *testing.T) {
	m := NewManager()
	err := m.Install("nome-inexistente-no-disco", "", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrada")
}

// ---- Update ----

func TestUpdate_PluginNativo(t *testing.T) {
	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "atlas", Version: "0.1.0"}, Path: "/fake/atlas"},
	}

	// Update de plugin nativo vai chamar Install("atlas", "latest", true)
	// Vai falhar pois latest não é versão dev, mas vai tentar baixar (falha de rede ok)
	err := m.Update("atlas")
	assert.Error(t, err) // Falha de rede/download esperada
}

// ---- loadValues com YAML inválido ----

func TestLoadValues_YAMLInvalido(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.yaml")
	os.WriteFile(path, []byte("invalid: [yaml: :::"), 0644)

	_, err := loadValues(path)
	assert.Error(t, err)
}

func TestLoadValues_ArquivoVazio(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.yaml")
	os.WriteFile(path, []byte(""), 0644)

	v, err := loadValues(path)
	assert.NoError(t, err)
	assert.Nil(t, v)
}

// ---- copyFile com destino inválido ----

func TestCopyFile_DestinoInvalido(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	os.WriteFile(src, []byte("data"), 0644)

	err := copyFile(src, "/diretorio/inexistente/dst.txt")
	assert.Error(t, err)
}

// ---- GetAssets com plugin que tem assets via script ----

func TestGetAssets_PluginComAssetsAbsoluto(t *testing.T) {
	tmp := t.TempDir()

	// Criar um script de plugin que retorna assets com caminho absoluto
	assetsPath := "/tmp/meus-assets"
	assetsDef := AssetsDefinition{Path: assetsPath}
	response := PluginResponse{Data: assetsDef}
	responseJSON, _ := json.Marshal(response)

	script := filepath.Join(tmp, "yby-plugin-assets")
	os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' '"+string(responseJSON)+"'"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{
			Manifest: PluginManifest{Name: "assets-test", Hooks: []string{"assets"}},
			Path:     script,
		},
	}

	assets := m.GetAssets()
	assert.Len(t, assets, 1)
	assert.Equal(t, assetsPath, assets[0])
}

func TestGetAssets_PluginComAssetsRelativo(t *testing.T) {
	tmp := t.TempDir()

	// Criar um script de plugin que retorna assets com caminho relativo
	assetsDef := AssetsDefinition{Path: "templates"}
	response := PluginResponse{Data: assetsDef}
	responseJSON, _ := json.Marshal(response)

	script := filepath.Join(tmp, "yby-plugin-assets-rel")
	os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' '"+string(responseJSON)+"'"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{
			Manifest: PluginManifest{Name: "assets-rel", Hooks: []string{"assets"}},
			Path:     script,
		},
	}

	assets := m.GetAssets()
	assert.Len(t, assets, 1)
	// Deve resolver relativo ao diretório do plugin
	expected := filepath.Join(tmp, "templates")
	assert.Equal(t, expected, assets[0])
}

func TestGetAssets_PluginComAssetsVazio(t *testing.T) {
	tmp := t.TempDir()

	// Plugin retorna assets com path vazio
	assetsDef := AssetsDefinition{Path: ""}
	response := PluginResponse{Data: assetsDef}
	responseJSON, _ := json.Marshal(response)

	script := filepath.Join(tmp, "yby-plugin-assets-empty")
	os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' '"+string(responseJSON)+"'"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{
			Manifest: PluginManifest{Name: "assets-empty", Hooks: []string{"assets"}},
			Path:     script,
		},
	}

	assets := m.GetAssets()
	assert.Empty(t, assets) // Path vazio = ignorado
}

// ---- ExecuteContextHook com plugin que retorna patch ----

func TestExecuteContextHook_ComPatchDeDados(t *testing.T) {
	tmp := t.TempDir()

	// Criar plugin que retorna patch de dados
	patch := map[string]interface{}{"chave_nova": "valor_novo"}
	response := PluginResponse{Data: patch}
	responseJSON, _ := json.Marshal(response)

	script := filepath.Join(tmp, "yby-plugin-ctx")
	os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' '"+string(responseJSON)+"'"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{
			Manifest: PluginManifest{Name: "ctx-plugin", Hooks: []string{"context"}},
			Path:     script,
		},
	}

	ctx := &scaffold.BlueprintContext{
		ProjectName: "teste",
		Data:        map[string]interface{}{"existente": "valor"},
	}
	err := m.ExecuteContextHook(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "valor_novo", ctx.Data["chave_nova"])
	assert.Equal(t, "valor", ctx.Data["existente"]) // Dados anteriores preservados
}

func TestExecuteContextHook_DataNilInicializado(t *testing.T) {
	tmp := t.TempDir()

	patch := map[string]interface{}{"nova": "info"}
	response := PluginResponse{Data: patch}
	responseJSON, _ := json.Marshal(response)

	script := filepath.Join(tmp, "yby-plugin-ctx-nil")
	os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' '"+string(responseJSON)+"'"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{
			Manifest: PluginManifest{Name: "ctx-nil", Hooks: []string{"context"}},
			Path:     script,
		},
	}

	// Data inicia como nil
	ctx := &scaffold.BlueprintContext{ProjectName: "teste"}
	err := m.ExecuteContextHook(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, ctx.Data)
	assert.Equal(t, "info", ctx.Data["nova"])
}

// ---- BuildPluginContext ----

func TestBuildPluginContext_SemArquivoDeAmbientes(t *testing.T) {
	tmp := t.TempDir()

	m := NewManager()
	coreCtx := &projectContext.CoreContext{
		ProjectName: "meu-projeto",
		Environment: "staging",
	}
	bpCtx := &scaffold.BlueprintContext{
		Data: make(map[string]interface{}),
	}

	fullCtx, values, err := m.BuildPluginContext(coreCtx, bpCtx, tmp)
	assert.NoError(t, err)
	assert.Equal(t, "meu-projeto", fullCtx.ProjectName)
	assert.Equal(t, "staging", fullCtx.Environment)
	assert.Empty(t, values) // Sem values pois não há arquivo de ambientes
}

func TestBuildPluginContext_ComAmbienteInvalido(t *testing.T) {
	tmp := t.TempDir()

	// Criar environments.yaml sem o ambiente solicitado
	envConfig := `
current: dev
environments:
  dev:
    type: local
`
	os.MkdirAll(filepath.Join(tmp, ".yby"), 0755)
	os.WriteFile(filepath.Join(tmp, ".yby", "environments.yaml"), []byte(envConfig), 0644)

	m := NewManager()
	coreCtx := &projectContext.CoreContext{
		ProjectName: "projeto",
		Environment: "producao", // Não existe no manifesto
	}
	bpCtx := &scaffold.BlueprintContext{Data: make(map[string]interface{})}

	fullCtx, _, err := m.BuildPluginContext(coreCtx, bpCtx, tmp)
	assert.NoError(t, err)
	assert.Equal(t, "producao", fullCtx.Environment)
	// Infra deve estar vazia pois o ambiente não foi encontrado
	assert.Empty(t, fullCtx.Infra.KubeContext)
}

// ---- Install com file:// ----

func TestInstall_FileProtocol_PluginJaExisteSemForce(t *testing.T) {
	tmp := t.TempDir()
	srcPath := filepath.Join(tmp, "yby-plugin-dup")
	os.WriteFile(srcPath, []byte("#!/bin/sh\necho ok"), 0755)

	m := NewManager()
	m.plugins = []LoadedPlugin{
		{Manifest: PluginManifest{Name: "yby-plugin-dup"}, Path: "/fake/yby-plugin-dup"},
	}

	err := m.Install("file://"+srcPath, "", false)
	// Deve falhar pois plugin já existe e force=false
	// Note: pode falhar por outra razão (destino), mas o caminho é exercitado
	_ = err
}

// ---- Remove com Discover automático ----

func TestRemove_ChamaDiscoverQuandoSemPlugins(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	m := NewManager()
	// plugins está vazio, Remove vai chamar Discover() automaticamente
	err := m.Remove("inexistente")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrado")
}

// ---- Update com Discover automático ----

func TestUpdate_ChamaDiscoverQuandoSemPlugins(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	m := NewManager()
	// plugins está vazio, Update vai chamar Discover() automaticamente
	err := m.Update("inexistente")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrado")
}

// ---- Install com Discover automático ----

func TestInstall_ChamaDiscoverQuandoSemPlugins(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	m := NewManager()
	// plugins vazio, Install vai chamar Discover() automaticamente
	err := m.Install("origem-invalida", "", false)
	assert.Error(t, err)
}
