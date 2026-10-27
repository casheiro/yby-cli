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
		},
		Execute: executeKubectlEvents,
	})
}

func executeKubectlEvents(ctx context.Context, params map[string]string) (string, error) {
	if err := conferirParams(params, "namespace"); err != nil {
		return "", err
	}
	args := []string{"get", "events", "--sort-by=.lastTimestamp"}

	if ns := namespaceDe(params); ns != "" {
		args = append(args, "-n", ns)
	}

	slog.Debug("executando kubectl", "args", strings.Join(args, " "))

	cmd := comandoKubectl(ctx, args)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("kubectl events falhou: %w\n%s", err, string(output))
	}

	return ultimosEventos(string(output)), nil
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
