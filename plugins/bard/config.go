package main

import (
	"os"

	"github.com/casheiro/yby-cli/pkg/ai"
	"gopkg.in/yaml.v3"
)

// BardConfig representa a configuração do Bard.
type BardConfig struct {
	TopK               int     `yaml:"top_k"`
	RelevanceThreshold float64 `yaml:"relevance_threshold"`
	SystemPromptExtra  string  `yaml:"system_prompt_extra"`
}

// loadBardConfig carrega a configuração do Bard a partir de .yby/bard.yaml.
// Se o arquivo não existir, retorna valores padrão. O corte de relevância padrão é o do
// pkg/ai, o mesmo da busca do Synapstor; o relevance_threshold do arquivo, mesmo 0, vale.
func loadBardConfig() BardConfig {
	cfg := BardConfig{
		TopK:               5,
		RelevanceThreshold: ai.RelevanciaMinima,
	}

	data, err := os.ReadFile(".yby/bard.yaml")
	if err != nil {
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)
	return cfg
}
