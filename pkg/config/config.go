package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// StateDir is the directory for local state
const StateDir = ".yby"

// StateFile is the file for local state
const StateFile = "state.yaml"

// Config represents the persisted configuration in .yby/state.yaml
type Config struct {
	CurrentContext string `yaml:"current_context"`
}

// Load reads the configuration
func Load() (*Config, error) {
	// Try current directory .yby/state.yaml
	path := filepath.Join(StateDir, StateFile)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Fallback detection (if needed in future) or return empty
		return &Config{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save writes the configuration
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	// Ensure directory exists
	if _, err := os.Stat(StateDir); os.IsNotExist(err) {
		if err := os.Mkdir(StateDir, 0755); err != nil {
			return err
		}
	}

	return os.WriteFile(filepath.Join(StateDir, StateFile), data, 0644)
}
