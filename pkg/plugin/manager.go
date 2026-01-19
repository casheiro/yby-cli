package plugin

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	projectContext "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/scaffold"
)

// Manager orchestrates plugin discovery and execution.
type Manager struct {
	executor *Executor
	plugins  []LoadedPlugin
}

type LoadedPlugin struct {
	Manifest PluginManifest
	Path     string
}

// NewManager creates a new Plugin Manager.
func NewManager() *Manager {
	return &Manager{
		executor: NewExecutor(),
		plugins:  make([]LoadedPlugin, 0),
	}
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

	for _, loc := range locations {
		if err := m.scanDirectory(loc); err != nil {
			// Log error but continue scanning other locations?
			// For now, silent ignore of non-existent dirs
			continue
		}
	}
	return nil
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

		// Check executable bit (approximate on Linux)
		if info.Mode()&0111 == 0 {
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
		manifest, err := m.loadManifest(path)
		if err != nil {
			fmt.Printf("⚠️  Pulando candidato a plugin inválido %s: %v\n", entry.Name(), err)
			continue
		}

		m.plugins = append(m.plugins, LoadedPlugin{
			Manifest: *manifest,
			Path:     path,
		})
	}
	return nil
}

func (m *Manager) loadManifest(path string) (*PluginManifest, error) {
	// Call plugin with hook="manifest"
	req := PluginRequest{Hook: "manifest"}
	resp, err := m.executor.Run(context.Background(), path, req)
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
			fmt.Printf("⚠️  Hook de assets do Plugin %s falhou: %v\n", p.Manifest.Name, err)
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

// ExecuteContextHook runs the 'context' hook on all applicable plugins and merges results.
func (m *Manager) ExecuteContextHook(ctx *scaffold.BlueprintContext) error {
	// Convert Context to map for sending
	// This is expensive but necessary as Context is struct.
	// We can use JSON marshalling.
	ctxBytes, _ := json.Marshal(ctx)
	var ctxMap map[string]interface{}
	if err := json.Unmarshal(ctxBytes, &ctxMap); err != nil {
		return err
	}

	for _, p := range m.plugins {
		supports := false
		for _, h := range p.Manifest.Hooks {
			if h == "context" {
				supports = true
				break
			}
		}
		if !supports {
			continue
		}

		req := PluginRequest{
			Hook:    "context",
			Context: ctxMap,
		}

		resp, err := m.executor.Run(context.Background(), p.Path, req)
		if err != nil {
			fmt.Printf("⚠️  Hook de contexto do Plugin %s falhou: %v\n", p.Manifest.Name, err)
			continue
		}

		// Merge patch
		patchMap, ok := resp.Data.(map[string]interface{})
		if ok {
			// Apply to Data field of BlueprintContext
			// We assume plugins return data to be put in specific fields or just Data bucket.
			// Spec: "Receber Patches. Atualizar BlueprintContext.Data."
			if ctx.Data == nil {
				ctx.Data = make(map[string]interface{})
			}
			for k, v := range patchMap {
				ctx.Data[k] = v
			}
		}
	}
	return nil
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
		return fmt.Errorf("plugin %s não encontrado", pluginName)
	}

	// Prepare Request
	// 1. Load Core Context (Synapstor / README / Identity)
	cwd, _ := os.Getwd()
	coreCtx, err := projectContext.GetCoreContext(cwd)
	if err != nil {
		fmt.Printf("⚠️  Aviso: Falha ao carregar contexto core: %v\n", err)
		// Fallback to empty core context
		coreCtx = &projectContext.CoreContext{
			ProjectName: "unknown",
			Environment: "unknown",
		}
	}

	// 2. Prepare BlueprintContext for plugin enrichment
	// We map CoreContext fields into the Data map so they are available to plugins (and consumers like Bard)
	initialData := make(map[string]interface{})

	// Convert CoreContext struct to map for Data bucket
	// We do this manually or via JSON roundtrip to be safe
	coreBytes, _ := json.Marshal(coreCtx)
	_ = json.Unmarshal(coreBytes, &initialData)

	blueprintCtx := &scaffold.BlueprintContext{
		ProjectName: coreCtx.ProjectName,
		Environment: coreCtx.Environment,
		Data:        initialData,
	}

	// 3. Run Context Hook (Collect data from Atlas, etc.)
	// This will populate blueprintCtx.Data with plugin contributions (e.g. "blueprint" from Atlas)
	if err := m.ExecuteContextHook(blueprintCtx); err != nil {
		fmt.Printf("⚠️  Erro ao coletar contexto dos plugins: %v\n", err)
	}

	// 4. Final Context for the command
	// We pass the aggregated Data map
	req := PluginRequest{
		Hook:    "command",
		Args:    args,
		Context: blueprintCtx.Data,
	}

	fmt.Printf("🚀 Executing plugin: %s\n", pluginName)
	return m.executor.RunInteractive(context.Background(), targetPlugin.Path, req)
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
		_ = m.Discover()
	}

	p, found := m.GetPlugin(name)
	if !found {
		return fmt.Errorf("plugin '%s' não encontrado", name)
	}

	// Safety check: Only remove from user home dir to avoid deleting project files
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	startPath := filepath.Join(home, ".yby", "plugins")

	rel, err := filepath.Rel(startPath, p.Path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("plugin '%s' está instalado fora do diretório global (%s). Remoção manual necessária", name, p.Path)
	}

	fmt.Printf("🗑️  Removendo plugin %s de %s...\n", name, p.Path)
	return os.Remove(p.Path)
}

// Update attempts to update a plugin to the latest version.
func (m *Manager) Update(name string) error {
	// Ensure we have the latest state
	if len(m.plugins) == 0 {
		_ = m.Discover()
	}

	p, found := m.GetPlugin(name)
	if !found {
		return fmt.Errorf("plugin '%s' não encontrado", name)
	}

	// Update logic depends on source.
	// For native plugins, we can try to install "latest".
	nativePlugins := map[string]bool{
		"atlas":    true,
		"bard":     true,
		"sentinel": true,
		"forge":    true,
		"oracle":   true,
		"viz":      true,
	}

	if nativePlugins[name] {
		// Native plugin: simple reinstall/upgrade
		fmt.Printf("🔄 Atualizando plugin nativo '%s' (Atual: %s)...\n", name, p.Manifest.Version)
		return m.Install(name, "latest", true) // Force = true
	}

	// For generic plugins, we don't track the source URL currently.
	// Future improvement: save metadata file alongside binary.
	return fmt.Errorf("update automático não suportado para plugins de terceiros '%s' (origem desconhecida). Por favor, reinstale manualmente", name)
}

// Install downloads and installs a native plugin.
// Supports: "file:///path/to/binary" or native plugin names "atlas", "bard", "sentinel".
func (m *Manager) Install(pluginSource, version string, force bool) error {
	// Discover existing first to check for conflicts
	if len(m.plugins) == 0 {
		_ = m.Discover()
	}
	nativePlugins := map[string]bool{
		"atlas":    true,
		"bard":     true,
		"sentinel": true,
		"forge":    true,
		"oracle":   true,
		"viz":      true,
	}

	if nativePlugins[pluginSource] {
		// Check if already installed
		if !force {
			if existing, found := m.GetPlugin(pluginSource); found {
				if existing.Manifest.Version == version && version != "latest" {
					return fmt.Errorf("plugin '%s' versão %s já está instalado. Use --force para reinstalar", pluginSource, version)
				}
				if version == "latest" {
					fmt.Printf("⚠️  Plugin '%s' já existe (v%s). Reinstalando 'latest'...\n", pluginSource, existing.Manifest.Version)
				} else {
					fmt.Printf("⚠️  Plugin '%s' já existe (v%s). Substituindo por v%s...\n", pluginSource, existing.Manifest.Version, version)
				}
			}
		}
		return m.installNative(pluginSource, version)
	}

	fmt.Printf("📦 Instalando plugin de %s...\n", pluginSource)

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
			return fmt.Errorf("origem do plugin não encontrada ou esquema não suportado ainda: %s", pluginSource)
		}
	}

	// Determine destination
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("falha ao obter diretório home do usuário: %w", err)
	}
	pluginsDir := filepath.Join(home, ".yby", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("falha ao criar diretório de plugins: %w", err)
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
			return fmt.Errorf("plugin '%s' já está instalado em %s. Use --force para sobrescrever", pluginName, existing.Path)
		}
	}

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("falha ao copiar binário: %w", err)
	}

	fmt.Printf("✅ Plugin %s instalado com sucesso em %s\n", pluginName, destPath)
	return nil
}

func (m *Manager) installNative(name, version string) error {
	if version == "dev" {
		fmt.Println("⚠️  Rodando em modo dev. Assumindo release 'latest' para plugins.")
		// In a real scenario, we might want to fail or look for local builds.
		// For now, let's warn and fail because we don't know the URL for sure without a tag.
		return fmt.Errorf("não é possível instalar plugins nativos em modo dev (version=dev). Construa localmente ou especifique uma versão")
	}

	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Format: yby-plugin-<name>_<version>_<os>_<arch>.tar.gz
	// Example: yby-plugin-atlas_v0.1.0_linux_amd64.tar.gz
	// Note: GoReleaser usually removes "v" from version in template {{ .Version }}
	// BUT checking .goreleaser.yaml: name_template: "yby-plugin-atlas_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
	// And ldflags: cmd.Version={{.Version}}.
	// Usually tags have 'v', but .Version might strip it or not depending on goreleaser config.
	// Let's assume the version string passed here matches what's in the filename.

	filename := fmt.Sprintf("yby-plugin-%s_%s_%s_%s.tar.gz", name, version, osName, arch)
	if osName == "windows" {
		filename = fmt.Sprintf("yby-plugin-%s_%s_%s_%s.zip", name, version, osName, arch)
	}

	// URL: https://github.com/casheiro/yby-cli/releases/download/<tag>/<filename>
	// Note: Release tags usually start with 'v', but artifact filenames (from goreleaser) do not always.
	// We need to ensure the tag component has 'v'.
	tag := version
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	url := fmt.Sprintf("https://github.com/casheiro/yby-cli/releases/download/%s/%s", tag, filename)

	fmt.Printf("⬇️  Baixando plugin %s de %s...\n", name, url)

	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "yby-plugin-install-*")
	if err != nil {
		return fmt.Errorf("falha ao criar diretório temporário: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("falha ao baixar plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("falha ao baixar plugin: status %d", resp.StatusCode)
	}

	// Extract
	// Handling tar.gz only for now (as per Linux user environment)
	// TODO: Handle Zip for windows if needed in future
	if strings.HasSuffix(filename, ".tar.gz") {
		if err := extractTarGz(resp.Body, tmpDir); err != nil {
			return fmt.Errorf("falha ao extrair plugin: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported archive format: %s", filename)
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
		return fmt.Errorf("falha ao encontrar binário no arquivo: %w", err)
	}

	if binaryPath == "" {
		return fmt.Errorf("binário %s não encontrado no arquivo baixado", binaryName)
	}

	// Install to final destination
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("falha ao obter diretório home do usuário: %w", err)
	}
	pluginsDir := filepath.Join(home, ".yby", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("falha ao criar diretório de plugins: %w", err)
	}

	finalPath := filepath.Join(pluginsDir, binaryName)

	// Move/Copy
	if err := copyFile(binaryPath, finalPath); err != nil {
		return fmt.Errorf("falha ao instalar binário: %w", err)
	}

	// Chmod +x
	if err := os.Chmod(finalPath, 0755); err != nil {
		return fmt.Errorf("falha ao tornar plugin executável: %w", err)
	}

	fmt.Printf("✅ Plugin %s instalado com sucesso em %s\n", name, finalPath)
	return nil
}

func (m *Manager) installFromURL(url string) error {
	fmt.Printf("⬇️  Baixando plugin genérico de %s...\n", url)

	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "yby-plugin-generic-*")
	if err != nil {
		return fmt.Errorf("falha ao criar diretório temporário: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("falha ao baixar plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("falha ao baixar plugin: status %d", resp.StatusCode)
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
				return fmt.Errorf("falha ao extrair plugin: %w", err)
			}
		} else {
			// Zip not implemented for untrusted URL yet in this snippet, sharing logic?
			// For minimal change, let's error if not tar.gz for Linux context
			return fmt.Errorf("formato de arquivo de plugin genérico não suportado: %s (apenas .tar.gz suportado atualmente)", filename)
		}
	} else {
		// Maybe it's a raw binary?
		// Write directly to file
		// Check name convention yby-plugin-*
		if !strings.HasPrefix(filename, "yby-plugin-") {
			fmt.Println("⚠️  Aviso: Nome do binário do plugin não começa com 'yby-plugin-'. Pode não ser descoberto...")
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
			return fmt.Errorf("falha ao percorrer arquivo fonte: %w", err)
		}
	} else {
		binaryPath = filepath.Join(tmpDir, pluginName)
	}

	if binaryPath == "" {
		return fmt.Errorf("nenhum executável começando com 'yby-plugin-' encontrado no arquivo")
	}

	// Install
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("falha ao obter diretório home do usuário: %w", err)
	}
	pluginsDir := filepath.Join(home, ".yby", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("falha ao criar diretório de plugins: %w", err)
	}

	finalPath := filepath.Join(pluginsDir, pluginName)
	if err := copyFile(binaryPath, finalPath); err != nil {
		return fmt.Errorf("falha ao instalar %s: %w", pluginName, err)
	}
	if err := os.Chmod(finalPath, 0755); err != nil {
		return fmt.Errorf("falha ao executar chmod: %w", err)
	}

	fmt.Printf("✅ Plugin genérico instalado: %s\n", finalPath)
	return nil
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

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
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
