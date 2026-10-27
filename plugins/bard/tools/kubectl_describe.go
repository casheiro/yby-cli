package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

func init() {
	Register(&Tool{
		Name:        "kubectl_describe",
		Description: "Exibe detalhes de um recurso Kubernetes específico",
		Intents:     []string{"describe_resource", "resource_details"},
		Parameters: []ToolParam{
			{Name: "resource", Description: "Tipo de recurso (pod, service, deployment, etc.)", Required: true},
			{Name: "name", Description: "Nome do recurso", Required: true},
			{Name: "namespace", Description: "Namespace alvo", Required: false},
		},
		Execute: executeKubectlDescribe,
	})
}

func executeKubectlDescribe(ctx context.Context, params map[string]string) (string, error) {
	resource := params["resource"]
	if resource == "" {
		return "", fmt.Errorf("parâmetro 'resource' é obrigatório")
	}

	name := params["name"]
	if name == "" {
		return "", fmt.Errorf("parâmetro 'name' é obrigatório")
	}
	if err := conferirParams(params, "resource", "name", "namespace"); err != nil {
		return "", err
	}

	args := []string{"describe", resource, name}

	if ns := namespaceDe(params); ns != "" {
		args = append(args, "-n", ns)
	}

	slog.Debug("executando kubectl", "args", strings.Join(args, " "))

	cmd := comandoKubectl(ctx, args)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("kubectl describe falhou: %w\n%s", err, string(output))
	}

	return string(output), nil
}
