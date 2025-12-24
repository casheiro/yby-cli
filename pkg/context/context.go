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
// 1. Env Var YBY_ENV
// 2. Flag (passed as arg)
// 3. Config (.ybyrc)
// 4. Default inference (default)
func (m *Manager) ResolveActive(flagContext string, cfg *config.Config) (string, error) {
	// 1. Env Var (Highest Priority for Automation)
	if env := os.Getenv("YBY_ENV"); env != "" {
		return env, nil
	}

	// 2. Flag
	if flagContext != "" {
		return flagContext, nil
	}

	// 3. Config
	if cfg != nil && cfg.CurrentContext != "" {
		return cfg.CurrentContext, nil
	}

	// 4. Default
	return "default", nil
}

// LoadContext loads the appropriate .env file and sets up the environment environment.
// It effectively sets the "Mode" of operation (Local Mirror vs Remote GitOps).
func (m *Manager) LoadContext(contextName string) error {
	var envFile string

	switch contextName {
	case "default":
		// Try standard .env
		envFile = filepath.Join(m.RootDir, ".env")
	case "local":
		// Try .env.local (Standard for local overrides)
		envFile = filepath.Join(m.RootDir, ".env.local")
		// If .env.local missing, try .env as fallback
		if _, err := os.Stat(envFile); os.IsNotExist(err) {
			envFile = filepath.Join(m.RootDir, ".env")
		}
	default:
		// Named contexts (staging, prod) -> .env.<name>
		envFile = filepath.Join(m.RootDir, fmt.Sprintf(".env.%s", contextName))
	}

	// Load valid file
	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Overload(envFile); err != nil {
			return fmt.Errorf("error loading %s: %w", envFile, err)
		}
	} else if contextName != "default" && contextName != "local" {
		// Strictness: If user explicitly asked for "prod" (via YBY_ENV=prod) and .env.prod is missing, we should probably warn or error?
		// For now, allow it (maybe configured via variables only)
	}

	return nil
}
