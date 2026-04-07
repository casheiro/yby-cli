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

	args := []string{"scan"}
	if ns := params["namespace"]; ns != "" {
		args = append(args, "-n", ns)
	}
	args = append(args, "-o", "json")

	req := plugin.PluginRequest{
		Hook: "command",
		Args: args,
	}

	resp, err := invokePlugin(binaryPath, req)
	if err != nil {
		return "", err
	}

	// Formatar resultado como texto legível
	return formatScanResults(resp.Data), nil
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
