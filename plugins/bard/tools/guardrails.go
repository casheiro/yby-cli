package tools

import (
	"fmt"
	"regexp"
	"strings"
)

// dangerousPattern representa um padrão perigoso com uma verificação customizada opcional.
type dangerousPattern struct {
	Pattern *regexp.Regexp
	Reason  string
	// ExceptIf, quando não nil, permite a operação se o texto combinar com este padrão.
	ExceptIf *regexp.Regexp
}

// dangerousPatterns contém padrões de operações perigosas que devem ser bloqueadas.
var dangerousPatterns = []dangerousPattern{
	{
		Pattern: regexp.MustCompile(`(?i)^delete\s+(namespace|ns)\b`),
		Reason:  "exclusão de namespace pode destruir todos os recursos contidos nele",
	},
	{
		Pattern: regexp.MustCompile(`(?i)^delete\s+clusterrole\b`),
		Reason:  "exclusão de ClusterRole pode comprometer o RBAC do cluster",
	},
	{
		Pattern: regexp.MustCompile(`(?i)^delete\s+clusterrolebinding\b`),
		Reason:  "exclusão de ClusterRoleBinding pode comprometer o RBAC do cluster",
	},
	{
		Pattern:  regexp.MustCompile(`(?i)\bdrain\s+node\b`),
		Reason:   "drain de node sem --ignore-daemonsets pode causar interrupção de serviço",
		ExceptIf: regexp.MustCompile(`(?i)--ignore-daemonsets`),
	},
	{
		Pattern: regexp.MustCompile(`(?i)\bcordon\b`),
		Reason:  "cordon de node impede scheduling e pode causar degradação do cluster",
	},
	{
		Pattern: regexp.MustCompile(`(?i)\bapply\b.*--force\b`),
		Reason:  "apply com --force pode recriar recursos destrutivamente",
	},
	{
		Pattern: regexp.MustCompile(`(?i)\bscale\b.*\breplicas[= ]+0\b`),
		Reason:  "escalar para 0 replicas pode causar downtime do serviço",
	},
}

// ValidateToolCall verifica se uma tool call contém operações perigosas.
// Retorna erro se a operação for considerada perigosa.
func ValidateToolCall(call ToolCall) error {
	// Verificar parâmetros que possam conter operações perigosas
	for _, param := range call.Params {
		for _, dp := range dangerousPatterns {
			if dp.Pattern.MatchString(param) {
				if dp.ExceptIf != nil && dp.ExceptIf.MatchString(param) {
					continue
				}
				return fmt.Errorf("operação bloqueada: %s", dp.Reason)
			}
		}
	}

	// Verificar combinação de tool name + params
	checkValue := buildCheckString(call)
	for _, dp := range dangerousPatterns {
		if dp.Pattern.MatchString(checkValue) {
			if dp.ExceptIf != nil && dp.ExceptIf.MatchString(checkValue) {
				continue
			}
			return fmt.Errorf("operação bloqueada: %s", dp.Reason)
		}
	}

	return nil
}

// buildCheckString constrói uma string representativa da operação para validação.
func buildCheckString(call ToolCall) string {
	var parts []string

	// Para kubectl tools, reconstruir o comando lógico
	switch call.Name {
	case "kubectl_get":
		parts = append(parts, "get")
	case "kubectl_describe":
		parts = append(parts, "describe")
	case "kubectl_logs":
		parts = append(parts, "logs")
	case "kubectl_events":
		parts = append(parts, "events")
	}

	for _, v := range call.Params {
		parts = append(parts, v)
	}

	return strings.Join(parts, " ")
}
