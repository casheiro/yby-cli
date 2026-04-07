//go:build k8s

package checks

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ExposedSecretsCheck verifica se variáveis de ambiente contêm secrets com valores hardcoded.
type ExposedSecretsCheck struct{}

func init() { Register(&ExposedSecretsCheck{}) }

func (c *ExposedSecretsCheck) ID() string         { return "POD_EXPOSED_SECRETS" }
func (c *ExposedSecretsCheck) Name() string       { return "Secrets Expostos" }
func (c *ExposedSecretsCheck) Category() Category { return CategorySecrets }
func (c *ExposedSecretsCheck) Severity() Severity { return SeverityCritical }
func (c *ExposedSecretsCheck) Description() string {
	return "Detecta variáveis de ambiente sensíveis com valores hardcoded"
}

func (c *ExposedSecretsCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			for _, env := range container.Env {
				lowerName := strings.ToLower(env.Name)
				isSensitive := strings.Contains(lowerName, "password") ||
					strings.Contains(lowerName, "secret") ||
					strings.Contains(lowerName, "token") ||
					strings.Contains(lowerName, "key")
				if isSensitive && env.ValueFrom == nil {
					findings = append(findings, SecurityFinding{
						CheckID:        c.ID(),
						Severity:       SeverityCritical,
						Category:       CategorySecrets,
						Pod:            pod.Name,
						Container:      container.Name,
						Namespace:      namespace,
						Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
						Message:        fmt.Sprintf("Container '%s' no pod '%s' tem env '%s' com valor hardcoded (use secretKeyRef)", container.Name, pod.Name, env.Name),
						Recommendation: "Use secretKeyRef ou configMapKeyRef para referenciar valores sensíveis",
						Type:           "critical",
						Description:    fmt.Sprintf("Container '%s' no pod '%s' tem env '%s' com valor hardcoded (use secretKeyRef)", container.Name, pod.Name, env.Name),
					})
				}
			}
		}
	}
	return findings, nil
}
