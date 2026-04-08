//go:build k8s

package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ResourceLimitsCheck verifica se containers possuem limites de CPU/memória definidos.
type ResourceLimitsCheck struct{}

func init() { Register(&ResourceLimitsCheck{}) }

func (c *ResourceLimitsCheck) ID() string         { return "POD_RESOURCE_LIMITS" }
func (c *ResourceLimitsCheck) Name() string       { return "Limites de Recursos" }
func (c *ResourceLimitsCheck) Category() Category { return CategoryPodSecurity }
func (c *ResourceLimitsCheck) Severity() Severity { return SeverityMedium }
func (c *ResourceLimitsCheck) Description() string {
	return "Detecta containers sem limites de CPU/memória"
}

func (c *ResourceLimitsCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			if container.Resources.Limits == nil || (container.Resources.Limits.Cpu().IsZero() && container.Resources.Limits.Memory().IsZero()) {
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityMedium,
					Category:       CategoryPodSecurity,
					Pod:            pod.Name,
					Container:      container.Name,
					Namespace:      namespace,
					Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
					Message:        fmt.Sprintf("Container '%s' no pod '%s' sem limites de CPU/memória definidos", container.Name, pod.Name),
					Recommendation: "Defina resources.limits.cpu e resources.limits.memory no container",
					Type:           "warning",
					Description:    fmt.Sprintf("Container '%s' no pod '%s' sem limites de CPU/memória definidos", container.Name, pod.Name),
				})
			}
		}
	}
	return findings, nil
}
