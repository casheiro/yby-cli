package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

func init() {
	Register(&Tool{
		Name:        "kubectl_logs",
		Description: "Exibe logs de um pod Kubernetes",
		Intents:     []string{"pod_logs", "view_logs", "check_logs"},
		Parameters: []ToolParam{
			{Name: "pod", Description: "Nome do pod", Required: true},
			{Name: "namespace", Description: "Namespace alvo", Required: false},
			{Name: "tail", Description: "Número de linhas do final (até 200). Padrão: 50", Required: false},
		},
		Execute: executeKubectlLogs,
	})
}

func executeKubectlLogs(ctx context.Context, params map[string]string) (string, error) {
	pod := params["pod"]
	if pod == "" {
		return "", fmt.Errorf("parâmetro 'pod' é obrigatório")
	}
	if err := conferirParams(params, "pod", "namespace", "tail"); err != nil {
		return "", err
	}

	args := argsKubectlLogs(pod, params)

	slog.Debug("executando kubectl", "args", strings.Join(args, " "))

	cmd := comandoKubectl(ctx, args)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("kubectl logs falhou: %w\n%s", err, string(output))
	}

	return string(output), nil
}

// maxLinhasLogs é o teto do --tail: a IA pedindo mais leva estas linhas, que já passam do
// que cabe no prompt (o Fluxo guarda o fim delas).
const maxLinhasLogs = 200

// argsKubectlLogs monta os args do kubectl logs, com o --tail pedido (50 sem pedido) até
// maxLinhasLogs. O tail já foi conferido como número.
func argsKubectlLogs(pod string, params map[string]string) []string {
	args := []string{"logs", pod}
	if ns := namespaceDe(params); ns != "" {
		args = append(args, "-n", ns)
	}
	tail := params["tail"]
	if tail == "" {
		tail = "50"
	}
	if n, err := strconv.Atoi(tail); err != nil || n > maxLinhasLogs {
		tail = strconv.Itoa(maxLinhasLogs)
	}
	return append(args, "--tail", tail)
}
