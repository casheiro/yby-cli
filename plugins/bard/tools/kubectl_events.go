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

	return string(output), nil
}
