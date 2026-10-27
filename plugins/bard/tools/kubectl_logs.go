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
			{Name: "container", Description: "Contêiner do pod. Obrigatório em pod com vários contêineres", Required: false},
			{Name: "previous", Description: "\"true\" para os logs da instância anterior do contêiner (pod em CrashLoop)", Required: false},
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
	if err := conferirParams(params, "pod", "namespace", "container", "previous", "tail"); err != nil {
		return "", err
	}

	args := argsKubectlLogs(pod, params)

	slog.Debug("executando kubectl", "args", strings.Join(args, " "))

	cmd := comandoKubectl(ctx, args)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("kubectl logs falhou: %w\n%s%s", err, string(output), conteineresDoPod(ctx, pod, params, string(output)))
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
	if c := params["container"]; c != "" {
		args = append(args, "-c", c)
	}
	if previousPedido(params["previous"]) {
		args = append(args, "--previous")
	}
	return append(args, "--tail", tail)
}

// previousPedido diz se a IA pediu os logs da instância anterior.
func previousPedido(valor string) bool {
	switch strings.ToLower(strings.TrimSpace(valor)) {
	case "true", "1", "sim", "yes":
		return true
	}
	return false
}

// conteineresDoPod devolve a linha "Contêineres do pod: a, b" para o erro do kubectl logs
// quando a falha é de contêiner (o kubectl já lista os contêineres na mensagem de "escolha
// um"; o nome inválido ele não lista). Entram os contêineres, os de init e os efêmeros,
// que o kubectl logs -c também aceita. Vazio nos outros erros ou se a consulta falha.
func conteineresDoPod(ctx context.Context, pod string, params map[string]string, saida string) string {
	if !strings.Contains(saida, "container") || strings.Contains(saida, "choose one of") {
		return ""
	}
	args := []string{"get", "pod", pod, "-o", "jsonpath={.spec.containers[*].name} {.spec.initContainers[*].name} {.spec.ephemeralContainers[*].name}"}
	if ns := namespaceDe(params); ns != "" {
		args = append(args, "-n", ns)
	}
	out, err := comandoKubectl(ctx, args).Output()
	nomes := strings.Fields(string(out))
	if err != nil || len(nomes) == 0 {
		return ""
	}
	return "\nContêineres do pod: " + strings.Join(nomes, ", ") + "\n"
}
