//go:build k8s

package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ServiceAccountCheck detecta pods com automountServiceAccountToken habilitado.
type ServiceAccountCheck struct{}

func init() { Register(&ServiceAccountCheck{}) }

func (c *ServiceAccountCheck) ID() string         { return "POD_SERVICE_ACCOUNT_TOKEN" }
func (c *ServiceAccountCheck) Name() string       { return "Service Account Token Auto-Mount" }
func (c *ServiceAccountCheck) Category() Category { return CategoryPodSecurity }
func (c *ServiceAccountCheck) Severity() Severity { return SeverityMedium }
func (c *ServiceAccountCheck) Description() string {
	return "Detecta pods com automountServiceAccountToken não desabilitado explicitamente"
}

func (c *ServiceAccountCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
			findings = append(findings, SecurityFinding{
				CheckID:        c.ID(),
				Severity:       SeverityMedium,
				Category:       CategoryPodSecurity,
				Pod:            pod.Name,
				Namespace:      namespace,
				Resource:       pod.Name,
				Message:        fmt.Sprintf("Pod '%s' não desabilita automountServiceAccountToken", pod.Name),
				Recommendation: "Defina automountServiceAccountToken: false a menos que o pod precise acessar a API do Kubernetes",
				Type:           "warning",
				Description:    fmt.Sprintf("Pod '%s' não desabilita automountServiceAccountToken", pod.Name),
			})
		}
	}
	return findings, nil
}
