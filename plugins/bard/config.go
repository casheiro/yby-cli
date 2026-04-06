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
	MaxTokens          int     `yaml:"max_tokens"`
}

// loadBardConfig carrega a configuração do Bard a partir de .yby/bard.yaml.
// Se o arquivo não existir, retorna valores padrão.
func loadBardConfig() BardConfig {
	cfg := BardConfig{
		TopK:               5,
		RelevanceThreshold: 0.6,
		MaxTokens:          32000,
	}

	data, err := os.ReadFile(".yby/bard.yaml")
	if err != nil {
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)
	return cfg
}

// filterByThreshold filtra documentos cujo score está abaixo do threshold.
func filterByThreshold(results []ai.UnknownDocument, threshold float64) []ai.UnknownDocument {
	var filtered []ai.UnknownDocument
	for _, res := range results {
		if float64(res.Score) >= threshold {
			filtered = append(filtered, res)
		}
	}
	return filtered
}
