package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

func init() {
	Register(&Tool{
		Name:        "sentinel_scan",
		Description: "Executa scan de segurança no cluster via plugin Sentinel",
		Intents:     []string{"security_scan", "scan_vulnerabilities", "check_security"},
		Parameters: []ToolParam{
			{Name: "namespace", Description: "Namespace a escanear. Se vazio, usa o namespace atual", Required: false},
		},
		Execute: executeSentinelScan,
	})
}

func executeSentinelScan(ctx context.Context, params map[string]string) (string, error) {
	binaryPath, err := discoverPluginBinary("sentinel")
	if err != nil {
		return "", fmt.Errorf("sentinel nao disponivel (paths tentados: %w)", err)
	}

	resp, err := invokePlugin(binaryPath, pedidoSentinelScan(params))
	if err != nil {
		return "", err
	}

	// Formatar resultado como texto legível
	return formatScanResults(resp.Data), nil
}

// pedidoSentinelScan é o pedido ao Sentinel para o scan, no cluster e no namespace da
// sessão quando a IA não diz o namespace. As recomendações da IA do Sentinel ficam de fora
// (--no-ai): quem analisa os achados é o próprio Bard.
func pedidoSentinelScan(params map[string]string) plugin.PluginRequest {
	args := []string{"scan"}
	if ns := namespaceDe(params); ns != "" {
		args = append(args, "-n", ns)
	}
	return pedidoAoPlugin(append(args, "-o", "json", "--no-ai"))
}

// formatScanResults converte os dados do scan em texto legível.
func formatScanResults(data interface{}) string {
	if data == nil {
		return "Nenhum finding reportado."
	}

	// Tentar formatar como JSON indentado
	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", data)
	}

	return string(formatted)
}
