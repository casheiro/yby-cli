package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

func init() {
	Register(&Tool{
		Name:        "atlas_blueprint",
		Description: "Obtém o blueprint do projeto via plugin Atlas (componentes, relações, linguagens)",
		Intents:     []string{"infra_topology", "project_architecture", "blueprint"},
		Parameters:  []ToolParam{},
		Execute:     executeAtlasBlueprint,
	})
}

func executeAtlasBlueprint(ctx context.Context, params map[string]string) (string, error) {
	binaryPath, err := discoverPluginBinary("atlas")
	if err != nil {
		return "", fmt.Errorf("atlas não disponível: %w", err)
	}

	req := plugin.PluginRequest{
		Hook: "context",
	}

	resp, err := invokePlugin(binaryPath, req)
	if err != nil {
		return "", err
	}

	return formatBlueprintResults(resp.Data), nil
}

// formatBlueprintResults converte dados do blueprint em resumo legível.
func formatBlueprintResults(data interface{}) string {
	if data == nil {
		return "Nenhum blueprint disponível."
	}

	// Tentar extrair como mapa para resumo mais rico
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("%v", data)
	}

	var blueprint map[string]interface{}
	if err := json.Unmarshal(dataBytes, &blueprint); err != nil {
		return string(dataBytes)
	}

	var sb strings.Builder
	sb.WriteString("## Blueprint do Projeto\n\n")

	// Resumir componentes por tipo
	if components, ok := blueprint["components"]; ok {
		compBytes, _ := json.Marshal(components)
		var compList []map[string]interface{}
		if err := json.Unmarshal(compBytes, &compList); err == nil {
			typeCounts := make(map[string]int)
			for _, comp := range compList {
				if t, ok := comp["type"].(string); ok {
					typeCounts[t]++
				}
			}
			sb.WriteString(fmt.Sprintf("**Componentes**: %d total\n", len(compList)))
			for t, count := range typeCounts {
				sb.WriteString(fmt.Sprintf("  - %s: %d\n", t, count))
			}
			sb.WriteString("\n")
		}
	}

	// Resumir relações
	if relations, ok := blueprint["relations"]; ok {
		relBytes, _ := json.Marshal(relations)
		var relList []interface{}
		if err := json.Unmarshal(relBytes, &relList); err == nil {
			sb.WriteString(fmt.Sprintf("**Relações**: %d\n\n", len(relList)))
		}
	}

	// Linguagens
	if languages, ok := blueprint["languages"]; ok {
		langBytes, _ := json.Marshal(languages)
		sb.WriteString(fmt.Sprintf("**Linguagens**: %s\n", string(langBytes)))
	}

	return sb.String()
}
