package tools

import (
	"context"
	"fmt"
	"log/slog"
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
			{Name: "output_format", Description: "Formato de saída (wide ou name). Padrão: wide", Required: false},
		},
		Execute: executeKubectlGet,
	})
}

// formatosGet são os formatos de saída que o kubectl_get aceita. Os outros (yaml, json,
// jsonpath, go-template, custom-columns) mostram o conteúdo dos recursos, como o data dos
// Secrets, que não vai à IA; a lista em wide mostra só nome, tipo e contagem.
var formatosGet = map[string]bool{"wide": true, "name": true}

func executeKubectlGet(ctx context.Context, params map[string]string) (string, error) {
	if err := conferirParams(params, "resource", "namespace"); err != nil {
		return "", err
	}
	resource := params["resource"]
	if resource == "" {
		resource = "pods" // default quando não especificado
	}

	outputFormat := strings.ToLower(strings.TrimSpace(params["output_format"]))
	if outputFormat == "" {
		outputFormat = "wide"
	}
	if !formatosGet[outputFormat] {
		return "", fmt.Errorf("formato de saída %q não permitido: o Bard lista só em wide ou name, e o conteúdo dos recursos (como o data dos Secrets) não vai à IA", params["output_format"])
	}

	args := []string{"get", resource}

	if ns := namespaceDe(params); ns != "" {
		args = append(args, "-n", ns)
	}

	args = append(args, "-o", outputFormat)

	slog.Debug("executando kubectl", "args", strings.Join(args, " "))

	cmd := comandoKubectl(ctx, args)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("kubectl get falhou: %w\n%s", err, string(output))
	}

	return string(output), nil
}
