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
		Intents:     []string{"investigate_pod", "diagnose_pod", "pod_health"},
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

	resp, err := invokePlugin(binaryPath, pedidoSentinelInvestigate(pod, params))
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

// pedidoSentinelInvestigate é o pedido ao Sentinel para investigar o pod, no cluster e no
// namespace da sessão quando a IA não diz o namespace.
func pedidoSentinelInvestigate(pod string, params map[string]string) plugin.PluginRequest {
	args := []string{"investigate", pod}
	if ns := namespaceDe(params); ns != "" {
		args = append(args, "-n", ns)
	}
	return pedidoAoPlugin(append(args, "-o", "json"))
}
