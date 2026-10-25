package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// BardConfig representa a configuração do Bard.
type BardConfig struct {
	TopK               int     `yaml:"top_k"`
	RelevanceThreshold float64 `yaml:"relevance_threshold"`
	SystemPromptExtra  string  `yaml:"system_prompt_extra"`
}

// loadBardConfig carrega a configuração do Bard a partir de .yby/bard.yaml.
// Se o arquivo não existir, retorna valores padrão.
func loadBardConfig() BardConfig {
	cfg := BardConfig{
		TopK:               5,
		RelevanceThreshold: 0.6,
	}

	data, err := os.ReadFile(".yby/bard.yaml")
	if err != nil {
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)
	return cfg
}
