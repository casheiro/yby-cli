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
		Name:        "kubectl_get",
		Description: "Executa kubectl get para listar recursos Kubernetes",
		Intents:     []string{"list_resources", "get_resources", "list_pods", "list_services"},
		Parameters: []ToolParam{
			{Name: "resource", Description: "Tipo de recurso (pods, services, deployments, etc.)", Required: true},
			{Name: "namespace", Description: "Namespace alvo", Required: false},
			{Name: "output_format", Description: "Formato de saída (wide, json, yaml). Padrão: wide", Required: false},
		},
		Execute: executeKubectlGet,
	})
}

func executeKubectlGet(ctx context.Context, params map[string]string) (string, error) {
	resource := params["resource"]
	if resource == "" {
		resource = "pods" // default quando não especificado
	}

	args := []string{"get", resource}

	if ns := params["namespace"]; ns != "" {
		args = append(args, "-n", ns)
	}

	outputFormat := params["output_format"]
	if outputFormat == "" {
		outputFormat = "wide"
	}
	args = append(args, "-o", outputFormat)

	slog.Debug("executando kubectl", "args", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("kubectl get falhou: %w\n%s", err, string(output))
	}

	return string(output), nil
}
