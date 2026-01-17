package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
			fmt.Printf("⚠️  Skipping invalid plugin candidate %s: %v\n", entry.Name(), err)
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
			fmt.Printf("⚠️  Plugin %s assets hook failed: %v\n", p.Manifest.Name, err)
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
			fmt.Printf("⚠️  Plugin %s context hook failed: %v\n", p.Manifest.Name, err)
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

func (m *Manager) ListPlugins() []PluginManifest {
	list := make([]PluginManifest, len(m.plugins))
	for i, p := range m.plugins {
		list[i] = p.Manifest
	}
	return list
}

// Install downloads and installs a native plugin.
// Supports: "file:///path/to/binary" or plain "binary_name" (needs repository URL logic, unimplemented).
// For now, we allow installing from local file path or "built-in" path relative to CWD for dev.
func (m *Manager) Install(pluginSource, version string) error {
	fmt.Printf("📦 Installing plugin from %s...\n", pluginSource)

	// Determine source path
	var srcPath string
	if strings.HasPrefix(pluginSource, "file://") {
		srcPath = strings.TrimPrefix(pluginSource, "file://")
	} else {
		// Assume it might be a local file if exists
		if _, err := os.Stat(pluginSource); err == nil {
			srcPath = pluginSource
		} else {
			return fmt.Errorf("plugin source not found or scheme not supported yet: %s", pluginSource)
		}
	}

	// Determine destination
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home: %w", err)
	}
	pluginsDir := filepath.Join(home, ".yby", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugins dir: %w", err)
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

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	fmt.Printf("✅ Plugin %s installed successfully to %s\n", pluginName, destPath)
	return nil
}
