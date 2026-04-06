package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

func init() {
	Register(&Tool{
		Name:        "kubectl_logs",
		Description: "Exibe logs de um pod Kubernetes",
		Parameters: []ToolParam{
			{Name: "pod", Description: "Nome do pod", Required: true},
			{Name: "namespace", Description: "Namespace alvo", Required: false},
			{Name: "tail", Description: "Número de linhas do final. Padrão: 50", Required: false},
		},
		Execute: executeKubectlLogs,
	})
}

func executeKubectlLogs(ctx context.Context, params map[string]string) (string, error) {
	pod := params["pod"]
	if pod == "" {
		return "", fmt.Errorf("parâmetro 'pod' é obrigatório")
	}

	args := []string{"logs", pod}

	if ns := params["namespace"]; ns != "" {
		args = append(args, "-n", ns)
	}

	tail := params["tail"]
	if tail == "" {
		tail = "50"
	}
	args = append(args, "--tail", tail)

	slog.Debug("executando kubectl", "args", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("kubectl logs falhou: %w\n%s", err, string(output))
	}

	return string(output), nil
}
