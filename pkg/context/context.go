package context

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Environment definition in environments.yaml
type Environment struct {
	Type        string `yaml:"type"` // local, remote
	Description string `yaml:"description"`
	Values      string `yaml:"values"` // path to values file
	URL         string `yaml:"url,omitempty"`
}

// EnvironmentsManifest represents .yby/environments.yaml
type EnvironmentsManifest struct {
	Current      string                 `yaml:"current"`
	Environments map[string]Environment `yaml:"environments"`
}

// Manager handles environment context operations
type Manager struct {
	RootDir string
}

func NewManager(rootDir string) *Manager {
	return &Manager{RootDir: rootDir}
}

func (m *Manager) LoadManifest() (*EnvironmentsManifest, error) {
	path := filepath.Join(m.RootDir, ".yby", "environments.yaml")

	// Strict Check: No legacy .env fallback
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("arquivo de ambientes não encontrado (%s). Execute 'yby init' primeiro", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest EnvironmentsManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("erro lendo environments.yaml: %w", err)
	}

	return &manifest, nil
}

func (m *Manager) SaveManifest(manifest *EnvironmentsManifest) error {
	path := filepath.Join(m.RootDir, ".yby", "environments.yaml")

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (m *Manager) GetCurrent() (string, *Environment, error) {
	manifest, err := m.LoadManifest()
	if err != nil {
		return "", nil, err
	}

	// 1. Env Override (YBY_ENV)
	if env := os.Getenv("YBY_ENV"); env != "" {
		if val, ok := manifest.Environments[env]; ok {
			return env, &val, nil
		}
		return "", nil, fmt.Errorf("ambiente '%s' (YBY_ENV) não definido em environments.yaml", env)
	}

	// 2. Manifest Current
	currentName := manifest.Current
	if val, ok := manifest.Environments[currentName]; ok {
		return currentName, &val, nil
	}

	return "", nil, fmt.Errorf("ambiente atual '%s' inválido ou não encontrado", currentName)
}

func (m *Manager) SetCurrent(name string) error {
	manifest, err := m.LoadManifest()
	if err != nil {
		return err
	}

	if _, ok := manifest.Environments[name]; !ok {
		return fmt.Errorf("ambiente '%s' não existe", name)
	}

	manifest.Current = name
	return m.SaveManifest(manifest)
}

// AddEnvironment adds a new environment to the manifest
func (m *Manager) AddEnvironment(name, envType, description string) error {
	manifest, err := m.LoadManifest()
	if err != nil {
		return err
	}

	if _, exists := manifest.Environments[name]; exists {
		return fmt.Errorf("environment '%s' already exists", name)
	}

	// Create values file if not exists
	valuesFile := fmt.Sprintf("config/values-%s.yaml", name)
	if _, err := os.Stat(filepath.Join(m.RootDir, valuesFile)); os.IsNotExist(err) {
		// Create empty or copy from base? For now empty with comment
		content := fmt.Sprintf("# Values for %s environment", name)
		if err := os.WriteFile(filepath.Join(m.RootDir, valuesFile), []byte(content), 0644); err != nil {
			return fmt.Errorf("falha ao criar arquivo de values: %w", err)
		}
	}

	manifest.Environments[name] = Environment{
		Type:        envType,
		Description: description,
		Values:      valuesFile,
	}

	return m.SaveManifest(manifest)
}
