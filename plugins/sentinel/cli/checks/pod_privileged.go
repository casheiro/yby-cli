//go:build k8s

package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PrivilegedCheck detecta containers com securityContext.privileged: true.
type PrivilegedCheck struct{}

func init() { Register(&PrivilegedCheck{}) }

func (c *PrivilegedCheck) ID() string         { return "POD_PRIVILEGED" }
func (c *PrivilegedCheck) Name() string       { return "Container Privilegiado" }
func (c *PrivilegedCheck) Category() Category { return CategoryPodSecurity }
func (c *PrivilegedCheck) Severity() Severity { return SeverityCritical }
func (c *PrivilegedCheck) Description() string {
	return "Detecta containers rodando em modo privilegiado"
}

func (c *PrivilegedCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		allContainers := append(pod.Spec.Containers, pod.Spec.InitContainers...)
		for _, container := range allContainers {
			if container.SecurityContext != nil && container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityCritical,
					Category:       CategoryPodSecurity,
					Pod:            pod.Name,
					Container:      container.Name,
					Namespace:      namespace,
					Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
					Message:        fmt.Sprintf("Container '%s' no pod '%s' roda em modo privilegiado", container.Name, pod.Name),
					Recommendation: "Remova securityContext.privileged: true — containers privilegiados têm acesso irrestrito ao host",
					Type:           "critical",
					Description:    fmt.Sprintf("Container '%s' no pod '%s' roda em modo privilegiado", container.Name, pod.Name),
				})
			}
		}
	}
	return findings, nil
}
