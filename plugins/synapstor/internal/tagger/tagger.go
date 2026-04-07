// Package tagger implementa auto-tagging de documentos UKI via IA.
package tagger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
)

// TagResult representa o resultado do tagging de um UKI.
type TagResult struct {
	Path string   `json:"path"`
	Tags []string `json:"tags"`
}

// TagUKI gera tags para um conteúdo de UKI usando IA.
func TagUKI(ctx context.Context, provider ai.Provider, content string) ([]string, error) {
	if provider == nil {
		return nil, fmt.Errorf("provedor de IA não configurado")
	}

	resp, err := provider.Completion(ctx, prompts.Get("synapstor.tagger"), content)
	if err != nil {
		return nil, fmt.Errorf("erro na IA: %w", err)
	}

	// Limpar resposta
	clean := strings.TrimSpace(resp)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var tags []string
	if err := json.Unmarshal([]byte(clean), &tags); err != nil {
		return nil, fmt.Errorf("erro ao parsear tags da IA: %w (resposta: %s)", err, resp)
	}

	// Validar limites
	if len(tags) < 1 {
		return nil, fmt.Errorf("IA retornou 0 tags")
	}
	if len(tags) > 7 {
		tags = tags[:7]
	}

	return tags, nil
}

// TagAll processa todos os UKIs em um diretório e gera tags.
func TagAll(ctx context.Context, provider ai.Provider, ukiDir string) ([]TagResult, error) {
	entries, err := os.ReadDir(ukiDir)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler diretório de UKIs: %w", err)
	}

	var results []TagResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(ukiDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		tags, err := TagUKI(ctx, provider, string(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "aviso: falha ao tagear %s: %v\n", entry.Name(), err)
			continue
		}

		results = append(results, TagResult{
			Path: filePath,
			Tags: tags,
		})
	}

	return results, nil
}
