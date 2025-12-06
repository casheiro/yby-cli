package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/casheiro/yby-cli/pkg/config"
	"github.com/joho/godotenv"
)

// ContextType defines the type of environment
type ContextType string

const (
	TypeLocal   ContextType = "local"
	TypeRemote  ContextType = "remote"
	TypeDefault ContextType = "default"
)

// Context represents an execution environment
type Context struct {
	Name    string
	Type    ContextType
	EnvFile string
}

// Manager handles context operations
type Manager struct {
	RootDir string
}

// NewManager creates a new context manager
func NewManager(rootDir string) *Manager {
	return &Manager{RootDir: rootDir}
}

// DetectContexts scans the directory for available contexts
func (m *Manager) DetectContexts() ([]Context, error) {
	var contexts []Context

	// 1. Check for Default (.env)
	if _, err := os.Stat(filepath.Join(m.RootDir, ".env")); err == nil {
		contexts = append(contexts, Context{
			Name:    "default",
			Type:    TypeDefault,
			EnvFile: ".env",
		})
	}

	// 2. Check for Local (local/k3d-config.yaml or local/.env)
	// We can assume strict "local" detection if the folder exists and likely has config
	if _, err := os.Stat(filepath.Join(m.RootDir, "local/k3d-config.yaml")); err == nil {
		contexts = append(contexts, Context{
			Name:    "local",
			Type:    TypeLocal,
			EnvFile: "local/.env", // Optional, might not exist
		})
	}

	// 3. Scan for .env.* (Remote contexts)
	entries, err := os.ReadDir(m.RootDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), ".env.") {
				name := strings.TrimPrefix(entry.Name(), ".env.")
				// avoid duplicates if someone names it .env.local (though typically .env.local is ignored by git, yby treats as context)
				if name == "local" {
					continue // Already handled or reserved
				}
				contexts = append(contexts, Context{
					Name:    name,
					Type:    TypeRemote,
					EnvFile: entry.Name(),
				})
			}
		}
	}

	return contexts, nil
}

// ResolveActive determines which context to use based on precedence:
// 1. Flag (passed as arg)
// 2. Config (.ybyrc)
// 3. Inference/Default
func (m *Manager) ResolveActive(flagContext string, cfg *config.Config) (string, error) {
	// 1. Flag
	if flagContext != "" {
		return flagContext, nil
	}

	// 2. Config
	if cfg != nil && cfg.CurrentContext != "" {
		return cfg.CurrentContext, nil
	}

	// 3. Default inference
	// If detected "default", return default.
	// We could check if "local" is available and default to it, but "default" (.env) is safer as standard behavior.
	return "default", nil
}

// LoadContext applies Strict Isolation rules to load variables
func (m *Manager) LoadContext(contextName string) error {
	// Strict Isolation: Do not load .env if loading a specific Named Context (unless it IS default)

	switch contextName {
	case "default":
		// Load .env
		file := filepath.Join(m.RootDir, ".env")
		if _, err := os.Stat(file); err == nil {
			return godotenv.Load(file)
		}
		// If default .env doesn't exist, it's fine, maybe just env vars
		return nil

	case "local":
		// Try local/.env
		file := filepath.Join(m.RootDir, "local", ".env")
		if _, err := os.Stat(file); err == nil {
			return godotenv.Load(file)
		}
		// logic for local might also imply no env file needed, just k3d calls
		return nil

	default:
		// Remote/Named context (e.g., staging)
		// MUST exist.
		filename := fmt.Sprintf(".env.%s", contextName)
		file := filepath.Join(m.RootDir, filename)

		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("context '%s' not found: file %s does not exist", contextName, filename)
		}

		// Load ONLY this file. explicitly.
		// godotenv.Overload might be meaningful if we want to overwrite existing OS vars,
		// but Load() is safer to not break CI vars.
		// However, we promise Isolation. existing OS vars (PROJECT_ID) might be set.
		// Yby philosophy: .env defines the state. Overload ensures the file wins?
		// Usually .env is for missing vars. Let's stick to Load, but ensure we don't load .env beforehand.
		return godotenv.Load(file)
	}
}
