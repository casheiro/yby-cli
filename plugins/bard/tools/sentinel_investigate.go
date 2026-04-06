package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

func init() {
	Register(&Tool{
		Name:        "sentinel_investigate",
		Description: "Investiga a segurança de um pod específico via plugin Sentinel",
		Parameters: []ToolParam{
			{Name: "pod", Description: "Nome do pod a investigar", Required: true},
			{Name: "namespace", Description: "Namespace do pod", Required: false},
		},
		Execute: executeSentinelInvestigate,
	})
}

func executeSentinelInvestigate(ctx context.Context, params map[string]string) (string, error) {
	pod := params["pod"]
	if pod == "" {
		return "", fmt.Errorf("parâmetro 'pod' é obrigatório")
	}

	binaryPath, err := discoverPluginBinary("sentinel")
	if err != nil {
		return "", fmt.Errorf("sentinel não disponível: %w", err)
	}

	args := []string{"investigate", pod}
	if ns := params["namespace"]; ns != "" {
		args = append(args, "-n", ns)
	}

	req := plugin.PluginRequest{
		Hook: "command",
		Args: args,
	}

	resp, err := invokePlugin(binaryPath, req)
	if err != nil {
		return "", err
	}

	if resp.Data == nil {
		return "Nenhuma informação de segurança encontrada para o pod.", nil
	}

	formatted, err := json.MarshalIndent(resp.Data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", resp.Data), nil
	}

	return string(formatted), nil
}
