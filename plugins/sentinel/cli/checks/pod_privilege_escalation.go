//go:build k8s

package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PrivilegeEscalationCheck detecta containers com allowPrivilegeEscalation: true.
type PrivilegeEscalationCheck struct{}

func init() { Register(&PrivilegeEscalationCheck{}) }

func (c *PrivilegeEscalationCheck) ID() string         { return "POD_PRIVILEGE_ESCALATION" }
func (c *PrivilegeEscalationCheck) Name() string       { return "Escalação de Privilégios" }
func (c *PrivilegeEscalationCheck) Category() Category { return CategoryPodSecurity }
func (c *PrivilegeEscalationCheck) Severity() Severity { return SeverityMedium }
func (c *PrivilegeEscalationCheck) Description() string {
	return "Detecta containers com allowPrivilegeEscalation habilitado"
}

func (c *PrivilegeEscalationCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		allContainers := append(pod.Spec.Containers, pod.Spec.InitContainers...)
		for _, container := range allContainers {
			if container.SecurityContext == nil || container.SecurityContext.AllowPrivilegeEscalation == nil || *container.SecurityContext.AllowPrivilegeEscalation {
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityMedium,
					Category:       CategoryPodSecurity,
					Pod:            pod.Name,
					Container:      container.Name,
					Namespace:      namespace,
					Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
					Message:        fmt.Sprintf("Container '%s' no pod '%s' permite escalação de privilégios", container.Name, pod.Name),
					Recommendation: "Defina securityContext.allowPrivilegeEscalation: false",
					Type:           "warning",
					Description:    fmt.Sprintf("Container '%s' no pod '%s' permite escalação de privilégios", container.Name, pod.Name),
				})
			}
		}
	}
	return findings, nil
}
