package plugin

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	projectContext "github.com/casheiro/yby-cli/v5/pkg/context"
	ybyerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/retry"
	"github.com/casheiro/yby-cli/v5/pkg/scaffold"
	"sigs.k8s.io/yaml"
)

// httpGet é uma variável para permitir mock em testes (substitui http.Get)
var httpGet = http.Get

// releaseBaseURL é a URL base para download de plugins nativos (mockável em testes)
var releaseBaseURL = "https://github.com/casheiro/yby-cli/releases/download"

// Manager orchestrates plugin discovery and execution.
type Manager struct {
	executor *Executor
	// manifestExecutor roda o hook manifest na descoberta, sem trust check: com a
	// verificação ligada, a descoberta só o chama para binários confiáveis.
	manifestExecutor *Executor
	plugins          []LoadedPlugin
	// ambiente é o ambiente pedido pelo usuário (DefinirAmbiente); vazio: nenhum.
	ambiente string
}

type LoadedPlugin struct {
	Manifest PluginManifest
	Path     string
}

// NewManager creates a new Plugin Manager.
func NewManager() *Manager {
	return &Manager{
		executor:         &Executor{Timeout: 30 * time.Second, SkipTrustCheck: true},
		manifestExecutor: &Executor{Timeout: 3 * time.Second, SkipTrustCheck: true},
		plugins:          make([]LoadedPlugin, 0),
	}
}

// EnableTrustCheck ativa a verificação de whitelist/checksum no executor principal e na
// descoberta. Deve ser chamado em ambientes de produção. A descoberta roda em todo comando
// do yby e um repositório clonado pode trazer um .yby/plugins: com a verificação ligada, o
// hook manifest só roda nos binários confiáveis, e os outros entram com um manifesto
// sintético (ver manifestoNaoConfiavel), sem rodar nada.
func (m *Manager) EnableTrustCheck() {
	m.executor.SkipTrustCheck = false
}

// Discover scans standard directories for plugins.
// Locations: ~/.yby/plugins, ./.yby/plugins
func (m *Manager) Discover() error {
	locations := []string{}

	// Home dir
	home, err := os.UserHomeDir()
	if err == nil {
		locations = append(locations, filepath.Join(home, ".yby", "plugins"))
	}

	// Local project dir (CWD)
	cwd, err := os.Getwd()
	if err == nil {
		locations = append(locations, filepath.Join(cwd, ".yby", "plugins"))
	}

	// Rodando na home, ./.yby/plugins é a própria ~/.yby/plugins: cada pasta é lida uma vez.
	var lidas []os.FileInfo
	for _, loc := range locations {
		info, err := os.Stat(loc)
		if err != nil || pastaJaLida(lidas, info) {
			continue
		}
		lidas = append(lidas, info)
		if err := m.scanDirectory(loc); err != nil {
			continue
		}
	}
	return nil
}

func pastaJaLida(lidas []os.FileInfo, info os.FileInfo) bool {
	for _, l := range lidas {
		if os.SameFile(l, info) {
			return true
		}
	}
	return false
}

func (m *Manager) scanDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Assume plugin is a directory containing a binary and manifest?
			// Or simple binaries?
			// Spec doesn't strictly define, but let's assume binaries named 'yby-plugin-*'
			// OR any executable that responds to 'manifest' hook?
			// To simplify: we assume binaries with executable permission.
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if !isPluginExecutable(entry.Name(), info.Mode()) {
			continue
		}

		// Simple filter: must start with yby-plugin-
		if !strings.HasPrefix(entry.Name(), "yby-plugin-") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		// Try to load manifest (optional, or implicit?)
		// Let's assume we invoke with Hook="manifest" to check?
		// Spec calls for Manifest struct but not how to get it initially.
		// For now, we trust it's a plugin if it matches name pattern.
		// Detailed implementation would verify.

		// Let's verify by calling "manifest" hook if possible, or just register.
		// Checking manifest is safer.
		manifest, err := m.manifestoDoCandidato(path)
		if err != nil {
			slog.Debug("Pulando candidato a plugin inválido", "name", entry.Name(), "error", err)
			continue
		}

		// Mesmo nome em duas pastas: fica o primeiro, que é o que o GetPlugin executaria.
		if anterior, ok := m.GetPlugin(manifest.Name); ok {
			slog.Debug("Plugin com nome repetido ignorado", "name", manifest.Name, "path", path, "carregado", anterior.Path)
			continue
		}

		m.plugins = append(m.plugins, LoadedPlugin{
			Manifest: *manifest,
			Path:     path,
		})
	}
	return nil
}

// isPluginExecutable indica se o arquivo pode ser executado como plugin.
// No Windows o Go não expõe bit de execução (o modo é sempre 0666/0444),
// então vale a extensão .exe.
func isPluginExecutable(name string, mode os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Ext(name), ".exe")
	}
	return mode&0111 != 0
}

// manifestoDoCandidato devolve o manifesto de um candidato a plugin. Sem a verificação de
// confiança, roda o hook manifest como sempre. Com ela, só o binário confiável roda; o não
// confiável (ou cuja confiança não deu para conferir) entra com o manifesto sintético. A
// confiança sai do carimbo do registro quando ele bate (a descoberta roda em todo comando),
// sem ler o binário inteiro.
func (m *Manager) manifestoDoCandidato(path string) (*PluginManifest, error) {
	if m.executor.SkipTrustCheck {
		return m.loadManifest(path)
	}
	confiavel, err := confiavelNaDescoberta(path)
	if err != nil {
		slog.Debug("Falha ao conferir a confiança do plugin", "path", path, "error", err)
	}
	if err != nil || !confiavel {
		return manifestoNaoConfiavel(path)
	}
	return m.loadManifest(path)
}

// manifestoNaoConfiavel é o manifesto de um binário que a descoberta não executou: o nome
// sai do arquivo (o que o yby plugin trust aceita) e o único hook é o command, para que
// "yby <nome>" caia na verificação de confiança do executor e devolva o erro com a dica, e
// para que o plugin apareça no yby plugin list e no yby --help.
func manifestoNaoConfiavel(path string) (*PluginManifest, error) {
	nome := nomeDoArquivo(path)
	if nome == "" {
		return nil, fmt.Errorf("nome de plugin vazio em %s", filepath.Base(path))
	}
	return &PluginManifest{
		Name:         nome,
		Description:  "não confiável · rode yby plugin trust " + nome,
		Hooks:        []string{"command"},
		NaoConfiavel: true,
	}, nil
}

func (m *Manager) loadManifest(path string) (*PluginManifest, error) {
	// Usa manifestExecutor (sem trust check) pois estamos descobrindo o plugin
	req := PluginRequest{Hook: "manifest"}
	resp, err := m.manifestExecutor.Run(context.Background(), path, req)
	if err != nil {
		return nil, err
	}

	// Check if Data is map
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, err
	}

	var manifest PluginManifest
	if err := json.Unmarshal(dataBytes, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// GetAssets returns list of local paths from plugins that provide assets.
func (m *Manager) GetAssets() []string {
	paths := []string{}
	for _, p := range m.plugins {
		// Check if supports 'assets' hook
		supports := false
		for _, h := range p.Manifest.Hooks {
			if h == "assets" {
				supports = true
				break
			}
		}
		if !supports {
			continue
		}

		resp, err := m.executor.Run(context.Background(), p.Path, PluginRequest{Hook: "assets"})
		if err != nil {
			slog.Warn("Hook de assets do Plugin falhou", "plugin", p.Manifest.Name, "error", err)
			continue
		}

		// Parse
		dataBytes, _ := json.Marshal(resp.Data)
		var assets AssetsDefinition
		if err := json.Unmarshal(dataBytes, &assets); err == nil && assets.Path != "" {
			// Resolve relative path?
			if filepath.IsAbs(assets.Path) {
				paths = append(paths, assets.Path)
			} else {
				paths = append(paths, filepath.Join(filepath.Dir(p.Path), assets.Path))
			}
		}
	}
	return paths
}

// DefinirAmbiente diz ao Manager qual ambiente do .yby/environments.yaml o usuário pediu
// (-c/--context ou YBY_ENV) para o command dos plugins. Vazio: nenhum, e os plugins ficam
// no current-context do kubeconfig (o current do environments.yaml não conta). Quem
// decide é o core, que conhece as flags.
func (m *Manager) DefinirAmbiente(nome string) {
	m.ambiente = nome
}

// ExecuteContextHook roda o hook context de todos os plugins que o declaram e mescla as
// respostas em bp.Data. É o fornecimento de contexto: o yby init o usa, e o command só
// para quem declara consome_contexto.
func (m *Manager) ExecuteContextHook(ctx context.Context, bp *scaffold.BlueprintContext) error {
	return m.executarHooksDeContexto(ctx, bp, "")
}

// executarHooksDeContexto é o ExecuteContextHook sem o plugin exceto (o consumidor não
// fornece contexto a si mesmo).
func (m *Manager) executarHooksDeContexto(ctx context.Context, bp *scaffold.BlueprintContext, exceto string) error {
	bpBytes, _ := json.Marshal(bp)
	var bpMap map[string]interface{}
	if err := json.Unmarshal(bpBytes, &bpMap); err != nil {
		return err
	}

	for _, p := range m.plugins {
		if p.Manifest.Name == exceto || !temHook(p.Manifest, "context") {
			continue
		}

		req := PluginRequest{
			Hook:    "context",
			Context: bpMap,
		}

		resp, err := m.executor.Run(ctx, p.Path, req)
		if err != nil {
			slog.Warn("Hook de contexto do Plugin falhou", "plugin", p.Manifest.Name, "error", err)
			continue
		}

		// O patch vai para bp.Data; resposta que não é mapa fica de fora.
		patchMap, ok := resp.Data.(map[string]interface{})
		if ok {
			if bp.Data == nil {
				bp.Data = make(map[string]interface{})
			}
			for k, v := range patchMap {
				bp.Data[k] = v
			}
		}
	}
	return nil
}

func temHook(mf PluginManifest, hook string) bool {
	for _, h := range mf.Hooks {
		if h == hook {
			return true
		}
	}
	return false
}

// ExecuteCommandHook runs the 'command' hook on a specific plugin.
// This is used when the plugin is invoked directly as a CLI subcommand (e.g., "yby bard").
func (m *Manager) ExecuteCommandHook(pluginName string, args []string) error {
	var targetPlugin *LoadedPlugin
	for _, p := range m.plugins {
		if p.Manifest.Name == pluginName {
			targetPlugin = &p
			break
		}
	}

	if targetPlugin == nil {
		return ybyerrors.New(ybyerrors.ErrCodePluginNotFound, fmt.Sprintf("plugin %s não encontrado", pluginName))
	}

	// A ajuda abre na hora: sem contexto nenhum.
	var fullCtxMap map[string]interface{}
	if !pedeAjuda(args) {
		cwd, _ := os.Getwd()
		fullCtx, err := m.contextoDoComando(context.Background(), targetPlugin, cwd)
		if err != nil {
			return err
		}
		if fullCtx != nil {
			fullCtxBytes, _ := json.Marshal(fullCtx)
			_ = json.Unmarshal(fullCtxBytes, &fullCtxMap)
		}
	}

	req := PluginRequest{
		Hook:    "command",
		Args:    args,
		Context: fullCtxMap,
	}

	slog.Debug("Executing plugin", "plugin", pluginName)
	return m.executor.RunInteractive(context.Background(), targetPlugin.Path, req)
}

// pedeAjuda informa se os args pedem a ajuda do plugin (-h ou --help antes de um --).
func pedeAjuda(args []string) bool {
	for _, a := range args {
		switch a {
		case "--":
			return false
		case "-h", "--help":
			return true
		}
	}
	return false
}

// contextoDoComando monta o contexto do command de alvo. O barato (projeto, ambiente,
// infra e values) sai do .yby/environments.yaml sem rodar nada, e só quando um ambiente
// foi pedido; o caro (hooks context dos outros plugins, overview e backlog) vai só para
// quem declara consome_contexto. Sem nenhum dos dois, nil: o plugin abre direto.
func (m *Manager) contextoDoComando(ctx context.Context, alvo *LoadedPlugin, cwd string) (*PluginFullContext, error) {
	consome := alvo.Manifest.ConsomeContexto
	if m.ambiente == "" && !consome {
		return nil, nil
	}

	fullCtx := &PluginFullContext{
		ProjectName: filepath.Base(cwd),
		Environment: m.ambiente,
	}
	if m.ambiente != "" {
		infra, values, err := lerAmbiente(cwd, m.ambiente)
		if err != nil {
			return nil, err
		}
		fullCtx.Infra, fullCtx.Values = infra, values
	}
	if consome {
		fullCtx.Data = m.contextoCaro(ctx, cwd, fullCtx.Environment, alvo.Manifest.Name)
	}
	return fullCtx, nil
}

// contextoCaro junta o overview e o backlog do projeto com o que os outros plugins
// fornecem pelo hook context. ambiente é o pedido (vazio: nenhum), não o current.
func (m *Manager) contextoCaro(ctx context.Context, cwd, ambiente, consumidor string) map[string]interface{} {
	coreCtx, err := projectContext.GetCoreContext(cwd)
	if err != nil {
		slog.Warn("Falha ao carregar contexto core", "error", err)
		coreCtx = &projectContext.CoreContext{ProjectName: filepath.Base(cwd)}
	}
	coreCtx.Environment = ambiente

	dados := make(map[string]interface{})
	coreBytes, _ := json.Marshal(coreCtx)
	_ = json.Unmarshal(coreBytes, &dados)

	bp := &scaffold.BlueprintContext{
		ProjectName: coreCtx.ProjectName,
		Environment: coreCtx.Environment,
		Data:        dados,
	}
	if err := m.executarHooksDeContexto(ctx, bp, consumidor); err != nil {
		slog.Warn("Erro ao construir contexto do plugin", "error", err)
	}
	return bp.Data
}

// lerAmbiente lê o ambiente nome do .yby/environments.yaml de cwd: a infra, com o caminho
// do kube_config resolvido, e os values. Ambiente pedido que não existe é erro: o plugin
// rodaria no cluster errado.
func lerAmbiente(cwd, nome string) (PluginInfrastructure, map[string]interface{}, error) {
	manifest, err := projectContext.NewManager(cwd).LoadManifest()
	if err != nil {
		return PluginInfrastructure{}, nil, ybyerrors.Wrap(err, ybyerrors.ErrCodeConfig,
			fmt.Sprintf("o ambiente %s foi pedido, mas não deu para ler os ambientes do projeto", nome)).
			WithHint("rode na pasta do projeto ou tire o -c/--context (e o YBY_ENV)")
	}
	env, ok := manifest.Environments[nome]
	if !ok {
		return PluginInfrastructure{}, nil, ybyerrors.New(ybyerrors.ErrCodeConfig,
			fmt.Sprintf("o ambiente %s não existe em .yby/environments.yaml", nome)).
			WithHint("rode yby env list para ver os ambientes")
	}

	infra := PluginInfrastructure{
		KubeConfig:  caminhoDoProjeto(cwd, env.KubeConfig),
		KubeContext: env.KubeContext,
		Namespace:   env.Namespace,
	}
	var values map[string]interface{}
	if env.Values != "" {
		if v, err := loadValues(filepath.Join(cwd, env.Values)); err == nil {
			values = v
		} else {
			slog.Warn("Falha ao carregar values do ambiente", "file", env.Values, "error", err)
		}
	}
	return infra, values, nil
}

// caminhoDoProjeto resolve um caminho do environments.yaml: ~/ vai para a home e o
// relativo, para a pasta do projeto.
func caminhoDoProjeto(cwd, caminho string) string {
	caminho = expandPath(caminho)
	if caminho == "" || filepath.IsAbs(caminho) {
		return caminho
	}
	return filepath.Join(cwd, caminho)
}

func loadValues(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v map[string]interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func expandPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func (m *Manager) ListPlugins() []PluginManifest {
	var manifests []PluginManifest
	for _, p := range m.plugins {
		manifests = append(manifests, p.Manifest)
	}
	return manifests
}

// GetPlugin returns a loaded plugin by name.
func (m *Manager) GetPlugin(name string) (*LoadedPlugin, bool) {
	for _, p := range m.plugins {
		if p.Manifest.Name == name {
			return &p, true
		}
	}
	return nil, false
}

// Remove uninstalls a plugin by name.
func (m *Manager) Remove(name string) error {
	// Ensure we have the latest state
	if len(m.plugins) == 0 {
		if err := m.Discover(); err != nil {
			slog.Warn("falha ao descobrir plugins", "erro", err)
		}
	}

	p, found := m.GetPlugin(name)
	if !found {
		return ybyerrors.New(ybyerrors.ErrCodePluginNotFound, fmt.Sprintf("plugin '%s' não encontrado", name))
	}

	// Safety check: Only remove from user home dir to avoid deleting project files
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	startPath := filepath.Join(home, ".yby", "plugins")

	rel, err := filepath.Rel(startPath, p.Path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ybyerrors.New(ybyerrors.ErrCodePlugin,
			fmt.Sprintf("plugin '%s' está instalado fora do diretório global (%s). Remoção manual necessária", name, p.Path))
	}

	slog.Info("Removendo plugin", "nome", name, "caminho", p.Path)
	return os.Remove(p.Path)
}

// Update atualiza um plugin. Os nativos são baixados de novo do release da versão
// informada, que é a do yby em execução: os plugins saem no mesmo release do core.
func (m *Manager) Update(name, versao string) error {
	// Ensure we have the latest state
	if len(m.plugins) == 0 {
		if err := m.Discover(); err != nil {
			slog.Warn("falha ao descobrir plugins", "erro", err)
		}
	}

	p, found := m.GetPlugin(name)
	if !found {
		return ybyerrors.New(ybyerrors.ErrCodePluginNotFound, fmt.Sprintf("plugin '%s' não encontrado", name))
	}

	// Update logic depends on source.
	nativePlugins := map[string]bool{
		"atlas":     true,
		"bard":      true,
		"sentinel":  true,
		"synapstor": true,
		"viz":       true,
	}

	if nativePlugins[name] {
		// Native plugin: simple reinstall/upgrade
		slog.Info("Atualizando plugin nativo", "nome", name, "versão_atual", p.Manifest.Version)
		return m.Install(name, versao, true) // Force = true
	}

	// For generic plugins, we don't track the source URL currently.
	// Future improvement: save metadata file alongside binary.
	return ybyerrors.New(ybyerrors.ErrCodePlugin,
		fmt.Sprintf("update automático não suportado para plugins de terceiros '%s' (origem desconhecida)", name)).
		WithHint("reinstale com yby plugin install <origem> --force")
}

// Install downloads and installs a native plugin.
// Supports: "file:///path/to/binary" or native plugin names "atlas", "bard", "sentinel".
func (m *Manager) Install(pluginSource, version string, force bool) error {
	// Discover existing first to check for conflicts
	if len(m.plugins) == 0 {
		if err := m.Discover(); err != nil {
			slog.Warn("falha ao descobrir plugins", "erro", err)
		}
	}
	nativePlugins := map[string]bool{
		"atlas":     true,
		"bard":      true,
		"sentinel":  true,
		"synapstor": true,
		"viz":       true,
	}

	if nativePlugins[pluginSource] {
		// Check if already installed
		if !force {
			if existing, found := m.GetPlugin(pluginSource); found {
				if existing.Manifest.Version == version && version != "latest" {
					return ybyerrors.New(ybyerrors.ErrCodePlugin,
						fmt.Sprintf("plugin '%s' versão %s já está instalado", pluginSource, version)).
						WithHint("use --force para reinstalar")
				}
				if version == "latest" {
					slog.Warn("Plugin já existe, reinstalando latest", "nome", pluginSource, "versão_atual", existing.Manifest.Version)
				} else {
					slog.Warn("Plugin já existe, substituindo", "nome", pluginSource, "versão_atual", existing.Manifest.Version, "nova_versão", version)
				}
			}
		}
		return m.installNative(pluginSource, version)
	}

	slog.Info("Instalando plugin", "origem", pluginSource)

	// Determine source path
	var srcPath string
	if strings.HasPrefix(pluginSource, "file://") {
		srcPath = strings.TrimPrefix(pluginSource, "file://")
	} else if strings.HasPrefix(pluginSource, "http://") || strings.HasPrefix(pluginSource, "https://") {
		return m.installFromURL(pluginSource)
	} else {
		// Assume it might be a local file if exists
		if _, err := os.Stat(pluginSource); err == nil {
			srcPath = pluginSource
		} else {
			return ybyerrors.New(ybyerrors.ErrCodePlugin,
				fmt.Sprintf("origem do plugin não encontrada ou esquema não suportado ainda: %s", pluginSource))
		}
	}

	// Determine destination
	home, err := os.UserHomeDir()
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao obter diretório home do usuário")
	}
	pluginsDir := filepath.Join(home, ".yby", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao criar diretório de plugins")
	}

	pluginName := filepath.Base(srcPath)
	destPath := filepath.Join(pluginsDir, pluginName)

	// Copy
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(destPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Check if already installed (by name)
	if !force {
		if existing, found := m.GetPlugin(pluginName); found {
			return ybyerrors.New(ybyerrors.ErrCodePlugin,
				fmt.Sprintf("plugin '%s' já está instalado em %s", pluginName, existing.Path)).
				WithHint("use --force para sobrescrever")
		}
	}

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao copiar binário")
	}

	// Registrar checksum SHA256 no trust registry
	if err := TrustPlugin(destPath); err != nil {
		slog.Warn("Falha ao registrar checksum do plugin", "nome", pluginName, "erro", err)
	}

	slog.Info("Plugin instalado com sucesso", "nome", pluginName, "caminho", destPath)
	return nil
}

// pseudoVersao reconhece as pseudo-versões do Go (v5.8.4-0.20261028120000-abcdef123456),
// que apontam para um commit e não para um release.
var pseudoVersao = regexp.MustCompile(`\d{14}-[0-9a-f]{12}$`)

// versaoSemRelease diz se version não pode ser a tag de um release: "dev" (build sem os
// -ldflags do GoReleaser), "(devel)", pseudo-versões e versões com metadados de build
// ("+dirty") do go build dentro do repositório. "latest" também: o nome do pacote leva a
// versão, e não há tag "vlatest".
func versaoSemRelease(version string) bool {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	return v == "" || v == "dev" || v == "(devel)" || v == "latest" || strings.Contains(v, "+") || pseudoVersao.MatchString(v)
}

func (m *Manager) installNative(name, version string) error {
	if versaoSemRelease(version) {
		return ybyerrors.New(ybyerrors.ErrCodePlugin,
			fmt.Sprintf("a versão %s não é de um release, e o plugin %s só é baixado de um release", version, name)).
			WithHint(fmt.Sprintf("informe a tag de um release com --version (yby plugin install %s --version <tag>); as tags estão em github.com/casheiro/yby-cli/releases", name))
	}

	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Formato do GoReleaser: yby-plugin-<nome>_<versão>_<os>_<arch>.tar.gz (zip no
	// Windows). O {{ .Version }} do nome não tem o "v" da tag: yby-plugin-atlas_5.8.3_…
	// no release v5.8.3. A versão chega com "v" do go install (v5.8.4) e do --version, e
	// sem do GoReleaser (5.8.3).
	versaoArquivo := strings.TrimPrefix(version, "v")
	filename := fmt.Sprintf("yby-plugin-%s_%s_%s_%s.tar.gz", name, versaoArquivo, osName, arch)
	if osName == "windows" {
		filename = fmt.Sprintf("yby-plugin-%s_%s_%s_%s.zip", name, versaoArquivo, osName, arch)
	}

	// URL: https://github.com/casheiro/yby-cli/releases/download/<tag>/<filename>; a tag
	// tem o "v".
	tag := version
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	url := fmt.Sprintf("%s/%s/%s", releaseBaseURL, tag, filename)

	slog.Info("Baixando plugin", "nome", name, "url", url)

	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "yby-plugin-install-*")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao criar diretório temporário")
	}
	defer os.RemoveAll(tmpDir)

	// Download with retry
	var resp *http.Response
	err = retry.DoWithDefault(context.Background(), func() error {
		var errGet error
		resp, errGet = httpGet(url)
		if errGet != nil {
			return errGet
		}
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			return fmt.Errorf("server error: %d", resp.StatusCode)
		}
		return nil
	})
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeNetworkTimeout, "falha ao baixar plugin após tentativas")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ybyerrors.New(ybyerrors.ErrCodeNetworkTimeout, fmt.Sprintf("falha ao baixar plugin: status %d", resp.StatusCode))
	}

	// Extrair: o pacote é zip no Windows e tar.gz nos outros sistemas; o formato sai do
	// conteúdo, não só do nome
	if err := extrairPacote(resp.Body, filename, tmpDir); err != nil {
		return err
	}

	// Find the binary in extraction
	// Expected binary name: yby-plugin-<name>
	binaryName := fmt.Sprintf("yby-plugin-%s", name)
	if osName == "windows" {
		binaryName += ".exe"
	}

	// Search for binary in tmpDir (it might be in a subdir or root)
	var binaryPath string
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == binaryName {
			binaryPath = path
			return io.EOF // Stop search
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao encontrar binário no arquivo")
	}

	if binaryPath == "" {
		return ybyerrors.New(ybyerrors.ErrCodePlugin, fmt.Sprintf("binário %s não encontrado no arquivo baixado", binaryName))
	}

	// Install to final destination
	home, err := os.UserHomeDir()
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao obter diretório home do usuário")
	}
	pluginsDir := filepath.Join(home, ".yby", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao criar diretório de plugins")
	}

	finalPath := filepath.Join(pluginsDir, binaryName)

	// Move/Copy
	if err := copyFile(binaryPath, finalPath); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao instalar binário")
	}

	// Chmod +x
	if err := os.Chmod(finalPath, 0755); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao tornar plugin executável")
	}

	// Registrar checksum SHA256 no trust registry
	if err := TrustPlugin(finalPath); err != nil {
		slog.Warn("Falha ao registrar checksum do plugin", "nome", name, "erro", err)
	}

	slog.Info("Plugin instalado com sucesso", "nome", name, "caminho", finalPath)
	return nil
}

func (m *Manager) installFromURL(url string) error {
	slog.Info("Baixando plugin genérico", "url", url)

	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "yby-plugin-generic-*")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao criar diretório temporário")
	}
	defer os.RemoveAll(tmpDir)

	// Download with retry
	var resp *http.Response
	err = retry.DoWithDefault(context.Background(), func() error {
		var errGet error
		resp, errGet = httpGet(url)
		if errGet != nil {
			return errGet
		}
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			return fmt.Errorf("server error: %d", resp.StatusCode)
		}
		return nil
	})
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeNetworkTimeout, "falha ao baixar plugin após tentativas")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ybyerrors.New(ybyerrors.ErrCodeNetworkTimeout, fmt.Sprintf("falha ao baixar plugin: status %d", resp.StatusCode))
	}

	// We need to guess the format from URL or Content-Type if possible,
	// but simplest is to assume tar.gz for now as per our convention, or check extension.
	filename := filepath.Base(url)
	// Remove query params if any
	if idx := strings.Index(filename, "?"); idx != -1 {
		filename = filename[:idx]
	}

	pluginName := "unknown"
	if strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".zip") {
		// Try to extract
		if strings.HasSuffix(filename, ".tar.gz") {
			if err := extractTarGz(resp.Body, tmpDir); err != nil {
				return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao extrair plugin")
			}
		} else {
			// Zip not implemented for untrusted URL yet in this snippet, sharing logic?
			// For minimal change, let's error if not tar.gz for Linux context
			return ybyerrors.New(ybyerrors.ErrCodePlugin,
				fmt.Sprintf("formato de arquivo de plugin genérico não suportado: %s (apenas .tar.gz suportado atualmente)", filename))
		}
	} else {
		// Maybe it's a raw binary?
		// Write directly to file
		// Check name convention yby-plugin-*
		if !strings.HasPrefix(filename, "yby-plugin-") {
			slog.Warn("Nome do binário do plugin não começa com 'yby-plugin-', pode não ser descoberto", "arquivo", filename)
		}
		pluginName = filename
		destFile := filepath.Join(tmpDir, pluginName)
		out, err := os.Create(destFile)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, resp.Body); err != nil {
			out.Close()
			return err
		}
		out.Close()
	}

	// If extracted, find binary
	binaryPath := ""
	// If it was an archive, we walk. If raw binary, it's at tmpDir/filename
	if strings.HasSuffix(filename, ".tar.gz") {
		err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Improve heuristic: check for executable bit or name prefix
			// Since we don't know the name, we look for 'yby-plugin-*'
			if !info.IsDir() && strings.HasPrefix(info.Name(), "yby-plugin-") {
				binaryPath = path
				pluginName = info.Name()
				return io.EOF
			}
			return nil
		})
		if err != nil && err != io.EOF {
			return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao percorrer arquivo fonte")
		}
	} else {
		binaryPath = filepath.Join(tmpDir, pluginName)
	}

	if binaryPath == "" {
		return ybyerrors.New(ybyerrors.ErrCodePlugin, "nenhum executável começando com 'yby-plugin-' encontrado no arquivo")
	}

	// Install
	home, err := os.UserHomeDir()
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao obter diretório home do usuário")
	}
	pluginsDir := filepath.Join(home, ".yby", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao criar diretório de plugins")
	}

	finalPath := filepath.Join(pluginsDir, pluginName)
	if err := copyFile(binaryPath, finalPath); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, fmt.Sprintf("falha ao instalar %s", pluginName))
	}
	if err := os.Chmod(finalPath, 0755); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao executar chmod")
	}

	// Registrar checksum SHA256 no trust registry
	if err := TrustPlugin(finalPath); err != nil {
		slog.Warn("Falha ao registrar checksum do plugin", "nome", pluginName, "erro", err)
	}

	slog.Info("Plugin genérico instalado", "caminho", finalPath)
	return nil
}

// extrairPacote extrai o pacote baixado em dest, pelo formato do conteúdo (zip ou gzip);
// sem assinatura conhecida, vale a extensão de filename.
func extrairPacote(r io.Reader, filename, dest string) error {
	dados, err := io.ReadAll(r)
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeNetworkTimeout, "falha ao baixar plugin")
	}
	ehZip := bytes.HasPrefix(dados, []byte("PK\x03\x04"))
	ehGzip := bytes.HasPrefix(dados, []byte{0x1f, 0x8b})
	if !ehZip && !ehGzip {
		ehZip = strings.HasSuffix(filename, ".zip")
		ehGzip = strings.HasSuffix(filename, ".tar.gz")
	}
	switch {
	case ehZip:
		err = extractZip(dados, dest)
	case ehGzip:
		err = extractTarGz(bytes.NewReader(dados), dest)
	default:
		return ybyerrors.New(ybyerrors.ErrCodePlugin, fmt.Sprintf("formato de pacote não suportado: %s", filename))
	}
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao extrair plugin")
	}
	return nil
}

// extractZip extrai os arquivos de um zip em dest, com a mesma proteção contra caminhos
// fora de dest (Zip Slip) e a mesma máscara de permissões do extractTarGz.
func extractZip(dados []byte, dest string) error {
	leitor, err := zip.NewReader(bytes.NewReader(dados), int64(len(dados)))
	if err != nil {
		return err
	}
	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)
	for _, f := range leitor.File {
		target := filepath.Join(dest, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) {
			return ybyerrors.New(ybyerrors.ErrCodeValidation, fmt.Sprintf("entrada inválida no zip: %s", f.Name))
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if !f.Mode().IsRegular() {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := extrairArquivoDoZip(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extrairArquivoDoZip(f *zip.File, target string) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, f.Mode()&0755|0600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func extractTarGz(r io.Reader, dest string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		// Prevenir Zip Slip: garantir que o destino está dentro de dest
		cleanDest := filepath.Clean(dest) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) {
			return ybyerrors.New(ybyerrors.ErrCodeValidation, fmt.Sprintf("entrada inválida no tarball: %s", header.Name))
		}

		// Sanitizar permissões com máscara segura
		safeMode := os.FileMode(header.Mode) & 0755

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, safeMode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
