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
		Description: "Coleta o estado, os eventos e o fim dos logs de um pod via plugin Sentinel",
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
	if err := conferirParams(params, "pod", "namespace"); err != nil {
		return "", err
	}

	binaryPath, err := discoverPluginBinary("sentinel")
	if err != nil {
		return "", fmt.Errorf("sentinel não disponível: %w", err)
	}

	resp, err := invokePlugin(ctx, binaryPath, pedidoSentinelInvestigate(pod, params))
	if err != nil {
		return "", err
	}

	return formatInvestigateResults(resp.Data), nil
}

// formatInvestigateResults converte os dados do investigate em texto para a IA: JSON
// compacto e, depois dele, os logs do pod em linhas. Dentro do JSON os logs seriam uma
// linha só, e o teto do Fluxo, que guarda o fim de uma saída longa, perderia o fim deles.
func formatInvestigateResults(data interface{}) string {
	if data == nil {
		return "O Sentinel não devolveu dados do pod (estado, eventos e logs)."
	}
	logs := ""
	if campos, ok := data.(map[string]interface{}); ok {
		if l, ok := campos["logs"].(string); ok {
			logs = l
			semLogs := make(map[string]interface{}, len(campos))
			for k, v := range campos {
				if k != "logs" {
					semLogs[k] = v
				}
			}
			data = semLogs
		}
	}
	formatted, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	if logs == "" {
		return string(formatted)
	}
	return string(formatted) + "\n\nLogs do pod (fim):\n" + logs
}

// pedidoSentinelInvestigate é o pedido ao Sentinel para investigar o pod, no cluster e no
// namespace da sessão quando a IA não diz o namespace.
func pedidoSentinelInvestigate(pod string, params map[string]string) plugin.PluginRequest {
	args := []string{"investigate", pod}
	if ns := namespaceDe(params); ns != "" {
		args = append(args, "-n", ns)
	}
	// --no-ai: o Sentinel só coleta e o Bard analisa com o provider da sessão (/modelo).
	return pedidoAoPlugin(append(args, "-o", "json", "--no-ai"))
}
