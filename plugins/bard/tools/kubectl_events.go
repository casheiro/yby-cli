package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

func init() {
	Register(&Tool{
		Name:        "kubectl_events",
		Description: "Lista eventos Kubernetes ordenados por timestamp",
		Intents:     []string{"pod_events", "cluster_events", "check_events"},
		Parameters: []ToolParam{
			{Name: "namespace", Description: "Namespace alvo. Se vazio, usa o namespace atual", Required: false},
			{Name: "pod", Description: "Nome do pod: só os eventos dele. Se vazio, os do namespace inteiro", Required: false},
		},
		Execute: executeKubectlEvents,
	})
}

func executeKubectlEvents(ctx context.Context, params map[string]string) (string, error) {
	if err := conferirParams(params, "namespace", "pod"); err != nil {
		return "", err
	}
	args := argsKubectlEvents(params)

	slog.Debug("executando kubectl", "args", strings.Join(args, " "))

	cmd := comandoKubectl(ctx, args)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("kubectl events falhou: %w\n%s", err, string(output))
	}

	return ultimosEventos(string(output)), nil
}

// argsKubectlEvents monta os args do kubectl get events: com pod, só os eventos desse objeto.
func argsKubectlEvents(params map[string]string) []string {
	args := []string{"get", "events", "--sort-by=.lastTimestamp"}
	if ns := namespaceDe(params); ns != "" {
		args = append(args, "-n", ns)
	}
	if pod := params["pod"]; pod != "" {
		args = append(args, "--field-selector=involvedObject.name="+pod)
	}
	return args
}

// maxEventos é quantos eventos o kubectl_events leva à IA: os mais recentes.
const maxEventos = 50

// ultimosEventos deixa o cabeçalho e os últimos maxEventos da saída, que o
// --sort-by=.lastTimestamp põe do mais antigo para o mais novo, e conta os que saíram.
func ultimosEventos(saida string) string {
	linhas := strings.Split(strings.TrimRight(saida, "\n"), "\n")
	if len(linhas) <= maxEventos+1 {
		return saida
	}
	omitidos := len(linhas) - 1 - maxEventos
	partes := append([]string{linhas[0], fmt.Sprintf("… %d eventos mais antigos omitidos", omitidos)}, linhas[len(linhas)-maxEventos:]...)
	return strings.Join(partes, "\n") + "\n"
}
